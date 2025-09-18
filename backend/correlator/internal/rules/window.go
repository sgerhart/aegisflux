package rules

import (
	"sync"
	"time"

	"github.com/sgerhart/aegisflux/backend/correlator/internal/model"
)

// WindowBuffer maintains a per-host deque of recent events with garbage collection
type WindowBuffer struct {
	mu       sync.RWMutex
	hosts    map[string]*HostEventBuffer
	maxAge   time.Duration
	gcTicker *time.Ticker
	stopGC   chan struct{}
}

// HostEventBuffer maintains events for a specific host
type HostEventBuffer struct {
	mu     sync.RWMutex
	events []EventWithTimestamp
}

// EventWithTimestamp wraps an event with its timestamp for efficient time-based operations
type EventWithTimestamp struct {
	Event     *model.Event
	Timestamp time.Time
}

// NewWindowBuffer creates a new window buffer with specified maximum age
func NewWindowBuffer(maxAge time.Duration) *WindowBuffer {
	return &WindowBuffer{
		hosts:  make(map[string]*HostEventBuffer),
		maxAge: maxAge,
	}
}

// StartGC starts the garbage collection routine
func (wb *WindowBuffer) StartGC(gcInterval time.Duration) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	
	if wb.gcTicker != nil {
		return // Already started
	}
	
	wb.gcTicker = time.NewTicker(gcInterval)
	wb.stopGC = make(chan struct{})
	
	go wb.gcRoutine(wb.gcTicker, wb.stopGC)
}

// StopGC stops the garbage collection routine
func (wb *WindowBuffer) StopGC() {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	
	if wb.gcTicker != nil {
		wb.gcTicker.Stop()
		wb.gcTicker = nil
	}
	
	if wb.stopGC != nil {
		close(wb.stopGC)
		wb.stopGC = nil
	}
}

// Add adds an event to the appropriate host buffer
func (wb *WindowBuffer) Add(ev *model.Event) {
	if ev == nil {
		return
	}
	
	wb.mu.Lock()
	defer wb.mu.Unlock()
	
	hostID := ev.HostID
	if hostID == "" {
		return
	}
	
	// Get or create host buffer
	hostBuffer, exists := wb.hosts[hostID]
	if !exists {
		hostBuffer = &HostEventBuffer{
			events: make([]EventWithTimestamp, 0),
		}
		wb.hosts[hostID] = hostBuffer
	}
	
	// Add event with current timestamp
	eventWithTS := EventWithTimestamp{
		Event:     ev,
		Timestamp: time.Now(),
	}
	
	hostBuffer.mu.Lock()
	hostBuffer.events = append(hostBuffer.events, eventWithTS)
	hostBuffer.mu.Unlock()
}

// RecentByType returns recent events of a specific type for a host within the time window
func (wb *WindowBuffer) RecentByType(hostID string, eventType string, within time.Duration) []*model.Event {
	if hostID == "" || eventType == "" {
		return nil
	}
	
	wb.mu.RLock()
	hostBuffer, exists := wb.hosts[hostID]
	wb.mu.RUnlock()
	
	if !exists {
		return nil
	}
	
	hostBuffer.mu.RLock()
	defer hostBuffer.mu.RUnlock()
	
	now := time.Now()
	cutoff := now.Add(-within)
	
	var result []*model.Event
	
	// Iterate through events in reverse order (most recent first)
	for i := len(hostBuffer.events) - 1; i >= 0; i-- {
		eventWithTS := hostBuffer.events[i]
		
		// Skip events outside the time window
		if eventWithTS.Timestamp.Before(cutoff) {
			continue
		}
		
		// Check if event type matches
		if eventWithTS.Event.EventType == eventType {
			result = append(result, eventWithTS.Event)
		}
	}
	
	return result
}

// RecentEvents returns all recent events for a host within the time window
func (wb *WindowBuffer) RecentEvents(hostID string, within time.Duration) []*model.Event {
	if hostID == "" {
		return nil
	}
	
	wb.mu.RLock()
	hostBuffer, exists := wb.hosts[hostID]
	wb.mu.RUnlock()
	
	if !exists {
		return nil
	}
	
	hostBuffer.mu.RLock()
	defer hostBuffer.mu.RUnlock()
	
	now := time.Now()
	cutoff := now.Add(-within)
	
	var result []*model.Event
	
	// Iterate through events in reverse order (most recent first)
	for i := len(hostBuffer.events) - 1; i >= 0; i-- {
		eventWithTS := hostBuffer.events[i]
		
		// Skip events outside the time window
		if eventWithTS.Timestamp.Before(cutoff) {
			continue
		}
		
		result = append(result, eventWithTS.Event)
	}
	
	return result
}

// GC performs garbage collection to remove old events
func (wb *WindowBuffer) GC(now time.Time) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	
	cutoff := now.Add(-wb.maxAge)
	
	for hostID, hostBuffer := range wb.hosts {
		hostBuffer.mu.Lock()
		
		// Remove old events
		var keptEvents []EventWithTimestamp
		for _, eventWithTS := range hostBuffer.events {
			if eventWithTS.Timestamp.After(cutoff) {
				keptEvents = append(keptEvents, eventWithTS)
			}
		}
		
		hostBuffer.events = keptEvents
		hostBuffer.mu.Unlock()
		
		// Remove empty host buffers
		if len(keptEvents) == 0 {
			delete(wb.hosts, hostID)
		}
	}
}

// gcRoutine runs the garbage collection routine
func (wb *WindowBuffer) gcRoutine(ticker *time.Ticker, stopChan chan struct{}) {
	for {
		select {
		case <-ticker.C:
			wb.GC(time.Now())
		case <-stopChan:
			return
		}
	}
}

// GetStats returns statistics about the window buffer
func (wb *WindowBuffer) GetStats() map[string]interface{} {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	
	totalEvents := 0
	hostCount := len(wb.hosts)
	
	for _, hostBuffer := range wb.hosts {
		hostBuffer.mu.RLock()
		totalEvents += len(hostBuffer.events)
		hostBuffer.mu.RUnlock()
	}
	
	return map[string]interface{}{
		"host_count":   hostCount,
		"total_events": totalEvents,
		"max_age":      wb.maxAge.String(),
	}
}

// GetHostStats returns statistics for a specific host
func (wb *WindowBuffer) GetHostStats(hostID string) map[string]interface{} {
	if hostID == "" {
		return nil
	}
	
	wb.mu.RLock()
	hostBuffer, exists := wb.hosts[hostID]
	wb.mu.RUnlock()
	
	if !exists {
		return map[string]interface{}{
			"host_id":      hostID,
			"event_count":  0,
			"oldest_event": nil,
			"newest_event": nil,
		}
	}
	
	hostBuffer.mu.RLock()
	defer hostBuffer.mu.RUnlock()
	
	eventCount := len(hostBuffer.events)
	var oldestEvent, newestEvent *time.Time
	
	if eventCount > 0 {
		oldest := hostBuffer.events[0].Timestamp
		newest := hostBuffer.events[eventCount-1].Timestamp
		oldestEvent = &oldest
		newestEvent = &newest
	}
	
	return map[string]interface{}{
		"host_id":      hostID,
		"event_count":  eventCount,
		"oldest_event": oldestEvent,
		"newest_event": newestEvent,
	}
}

// Clear removes all events from the buffer
func (wb *WindowBuffer) Clear() {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	
	for hostID := range wb.hosts {
		delete(wb.hosts, hostID)
	}
}

// ClearHost removes all events for a specific host
func (wb *WindowBuffer) ClearHost(hostID string) {
	if hostID == "" {
		return
	}
	
	wb.mu.Lock()
	defer wb.mu.Unlock()
	
	delete(wb.hosts, hostID)
}
