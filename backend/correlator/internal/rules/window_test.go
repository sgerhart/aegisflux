package rules

import (
	"fmt"
	"testing"
	"time"

	"github.com/sgerhart/aegisflux/backend/correlator/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestWindowBuffer_Add(t *testing.T) {
	wb := NewWindowBuffer(1 * time.Hour)
	
	event1 := &model.Event{
		HostID:    "host-001",
		EventType: "exec",
		BinaryPath: "/bin/bash",
	}
	
	event2 := &model.Event{
		HostID:    "host-002", 
		EventType: "connect",
		BinaryPath: "/usr/bin/curl",
	}
	
	// Add events
	wb.Add(event1)
	wb.Add(event2)
	
	// Verify stats
	stats := wb.GetStats()
	assert.Equal(t, 2, stats["host_count"])
	assert.Equal(t, 2, stats["total_events"])
}

func TestWindowBuffer_RecentByType(t *testing.T) {
	wb := NewWindowBuffer(1 * time.Hour)
	
	// Create events with specific timestamps
	event1 := &model.Event{
		HostID:    "host-001",
		EventType: "exec",
		BinaryPath: "/bin/bash",
	}
	
	event2 := &model.Event{
		HostID:    "host-001",
		EventType: "connect", 
		BinaryPath: "/usr/bin/curl",
	}
	
	event3 := &model.Event{
		HostID:    "host-001",
		EventType: "exec",
		BinaryPath: "/bin/sh",
	}
	
	// Add events
	wb.Add(event1)
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	wb.Add(event2)
	time.Sleep(10 * time.Millisecond)
	wb.Add(event3)
	
	// Test RecentByType for exec events
	execEvents := wb.RecentByType("host-001", "exec", 1*time.Minute)
	assert.Len(t, execEvents, 2)
	assert.Equal(t, "exec", execEvents[0].EventType)
	assert.Equal(t, "exec", execEvents[1].EventType)
	
	// Test RecentByType for connect events
	connectEvents := wb.RecentByType("host-001", "connect", 1*time.Minute)
	assert.Len(t, connectEvents, 1)
	assert.Equal(t, "connect", connectEvents[0].EventType)
	
	// Test RecentByType for non-existent type
	fileEvents := wb.RecentByType("host-001", "file", 1*time.Minute)
	assert.Len(t, fileEvents, 0)
	
	// Test RecentByType for non-existent host
	unknownHostEvents := wb.RecentByType("host-999", "exec", 1*time.Minute)
	assert.Len(t, unknownHostEvents, 0)
}

func TestWindowBuffer_RecentEvents(t *testing.T) {
	wb := NewWindowBuffer(1 * time.Hour)
	
	event1 := &model.Event{
		HostID:    "host-001",
		EventType: "exec",
		BinaryPath: "/bin/bash",
	}
	
	event2 := &model.Event{
		HostID:    "host-001",
		EventType: "connect",
		BinaryPath: "/usr/bin/curl",
	}
	
	event3 := &model.Event{
		HostID:    "host-002",
		EventType: "exec",
		BinaryPath: "/bin/sh",
	}
	
	// Add events
	wb.Add(event1)
	wb.Add(event2)
	wb.Add(event3)
	
	// Test RecentEvents for host-001
	host1Events := wb.RecentEvents("host-001", 1*time.Minute)
	assert.Len(t, host1Events, 2)
	
	// Test RecentEvents for host-002
	host2Events := wb.RecentEvents("host-002", 1*time.Minute)
	assert.Len(t, host2Events, 1)
	
	// Test RecentEvents for non-existent host
	unknownHostEvents := wb.RecentEvents("host-999", 1*time.Minute)
	assert.Len(t, unknownHostEvents, 0)
}

func TestWindowBuffer_TimeWindowFilter(t *testing.T) {
	wb := NewWindowBuffer(1 * time.Hour)
	
	// Create events
	event1 := &model.Event{
		HostID:    "host-001",
		EventType: "exec",
		BinaryPath: "/bin/bash",
	}
	
	event2 := &model.Event{
		HostID:    "host-001", 
		EventType: "exec",
		BinaryPath: "/bin/sh",
	}
	
	// Add first event
	wb.Add(event1)
	
	// Wait for some time
	time.Sleep(100 * time.Millisecond)
	
	// Add second event
	wb.Add(event2)
	
	// Test with short time window (should only get recent event)
	recentEvents := wb.RecentByType("host-001", "exec", 50*time.Millisecond)
	assert.Len(t, recentEvents, 1)
	assert.Equal(t, "/bin/sh", recentEvents[0].BinaryPath)
	
	// Test with longer time window (should get both events)
	allEvents := wb.RecentByType("host-001", "exec", 1*time.Minute)
	assert.Len(t, allEvents, 2)
}

func TestWindowBuffer_GC(t *testing.T) {
	wb := NewWindowBuffer(100 * time.Millisecond) // Very short max age
	
	event1 := &model.Event{
		HostID:    "host-001",
		EventType: "exec",
		BinaryPath: "/bin/bash",
	}
	
	event2 := &model.Event{
		HostID:    "host-001",
		EventType: "connect",
		BinaryPath: "/usr/bin/curl",
	}
	
	// Add events
	wb.Add(event1)
	time.Sleep(50 * time.Millisecond)
	wb.Add(event2)
	
	// Verify both events are present
	stats := wb.GetStats()
	assert.Equal(t, 2, stats["total_events"])
	
	// Wait for first event to expire
	time.Sleep(60 * time.Millisecond)
	
	// Run GC
	wb.GC(time.Now())
	
	// Verify only recent event remains
	stats = wb.GetStats()
	assert.Equal(t, 1, stats["total_events"])
	
	// Verify only the recent event is accessible
	execEvents := wb.RecentByType("host-001", "connect", 1*time.Minute)
	assert.Len(t, execEvents, 1)
	assert.Equal(t, "connect", execEvents[0].EventType)
	
	// Verify old event is not accessible
	oldExecEvents := wb.RecentByType("host-001", "exec", 1*time.Minute)
	assert.Len(t, oldExecEvents, 0)
}

func TestWindowBuffer_GCRoutine(t *testing.T) {
	wb := NewWindowBuffer(50 * time.Millisecond) // Very short max age
	
	event := &model.Event{
		HostID:    "host-001",
		EventType: "exec",
		BinaryPath: "/bin/bash",
	}
	
	// Add event
	wb.Add(event)
	
	// Verify event is present
	stats := wb.GetStats()
	assert.Equal(t, 1, stats["total_events"])
	
	// Start GC routine
	wb.StartGC(10 * time.Millisecond)
	
	// Wait for GC to run and clean up expired events
	time.Sleep(100 * time.Millisecond)
	
	// Stop GC
	wb.StopGC()
	
	// Verify event was cleaned up
	stats = wb.GetStats()
	assert.Equal(t, 0, stats["total_events"])
	assert.Equal(t, 0, stats["host_count"])
}

func TestWindowBuffer_GetStats(t *testing.T) {
	wb := NewWindowBuffer(1 * time.Hour)
	
	// Initial stats
	stats := wb.GetStats()
	assert.Equal(t, 0, stats["host_count"])
	assert.Equal(t, 0, stats["total_events"])
	assert.Equal(t, "1h0m0s", stats["max_age"])
	
	// Add events
	event1 := &model.Event{
		HostID:    "host-001",
		EventType: "exec",
		BinaryPath: "/bin/bash",
	}
	
	event2 := &model.Event{
		HostID:    "host-002",
		EventType: "connect",
		BinaryPath: "/usr/bin/curl",
	}
	
	wb.Add(event1)
	wb.Add(event2)
	
	// Updated stats
	stats = wb.GetStats()
	assert.Equal(t, 2, stats["host_count"])
	assert.Equal(t, 2, stats["total_events"])
}

func TestWindowBuffer_GetHostStats(t *testing.T) {
	wb := NewWindowBuffer(1 * time.Hour)
	
	// Test stats for non-existent host
	stats := wb.GetHostStats("host-999")
	assert.Equal(t, "host-999", stats["host_id"])
	assert.Equal(t, 0, stats["event_count"])
	assert.Nil(t, stats["oldest_event"])
	assert.Nil(t, stats["newest_event"])
	
	// Add events
	event1 := &model.Event{
		HostID:    "host-001",
		EventType: "exec",
		BinaryPath: "/bin/bash",
	}
	
	event2 := &model.Event{
		HostID:    "host-001",
		EventType: "connect",
		BinaryPath: "/usr/bin/curl",
	}
	
	wb.Add(event1)
	time.Sleep(10 * time.Millisecond)
	wb.Add(event2)
	
	// Test stats for host with events
	stats = wb.GetHostStats("host-001")
	assert.Equal(t, "host-001", stats["host_id"])
	assert.Equal(t, 2, stats["event_count"])
	assert.NotNil(t, stats["oldest_event"])
	assert.NotNil(t, stats["newest_event"])
	
	// Verify timestamp ordering
	oldestEvent := stats["oldest_event"].(*time.Time)
	newestEvent := stats["newest_event"].(*time.Time)
	assert.True(t, oldestEvent.Before(*newestEvent) || oldestEvent.Equal(*newestEvent))
}

func TestWindowBuffer_Clear(t *testing.T) {
	wb := NewWindowBuffer(1 * time.Hour)
	
	// Add events
	event1 := &model.Event{
		HostID:    "host-001",
		EventType: "exec",
		BinaryPath: "/bin/bash",
	}
	
	event2 := &model.Event{
		HostID:    "host-002",
		EventType: "connect",
		BinaryPath: "/usr/bin/curl",
	}
	
	wb.Add(event1)
	wb.Add(event2)
	
	// Verify events are present
	stats := wb.GetStats()
	assert.Equal(t, 2, stats["host_count"])
	assert.Equal(t, 2, stats["total_events"])
	
	// Clear all
	wb.Clear()
	
	// Verify all events are gone
	stats = wb.GetStats()
	assert.Equal(t, 0, stats["host_count"])
	assert.Equal(t, 0, stats["total_events"])
}

func TestWindowBuffer_ClearHost(t *testing.T) {
	wb := NewWindowBuffer(1 * time.Hour)
	
	// Add events for multiple hosts
	event1 := &model.Event{
		HostID:    "host-001",
		EventType: "exec",
		BinaryPath: "/bin/bash",
	}
	
	event2 := &model.Event{
		HostID:    "host-002",
		EventType: "connect",
		BinaryPath: "/usr/bin/curl",
	}
	
	wb.Add(event1)
	wb.Add(event2)
	
	// Verify both hosts have events
	stats := wb.GetStats()
	assert.Equal(t, 2, stats["host_count"])
	assert.Equal(t, 2, stats["total_events"])
	
	// Clear host-001
	wb.ClearHost("host-001")
	
	// Verify only host-002 remains
	stats = wb.GetStats()
	assert.Equal(t, 1, stats["host_count"])
	assert.Equal(t, 1, stats["total_events"])
	
	// Verify host-001 events are gone
	host1Events := wb.RecentEvents("host-001", 1*time.Minute)
	assert.Len(t, host1Events, 0)
	
	// Verify host-002 events remain
	host2Events := wb.RecentEvents("host-002", 1*time.Minute)
	assert.Len(t, host2Events, 1)
}

func TestWindowBuffer_ConcurrentAccess(t *testing.T) {
	wb := NewWindowBuffer(1 * time.Hour)
	
	// Test concurrent access
	done := make(chan bool, 10)
	
	// Start multiple goroutines adding events
	for i := 0; i < 5; i++ {
		go func(hostID string) {
			for j := 0; j < 10; j++ {
				event := &model.Event{
					HostID:    hostID,
					EventType: "exec",
					BinaryPath: "/bin/bash",
				}
				wb.Add(event)
			}
			done <- true
		}(fmt.Sprintf("host-%03d", i))
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		<-done
	}
	
	// Verify all events were added
	stats := wb.GetStats()
	assert.Equal(t, 5, stats["host_count"])
	assert.Equal(t, 50, stats["total_events"])
	
	// Verify events can be retrieved
	for i := 0; i < 5; i++ {
		hostID := fmt.Sprintf("host-%03d", i)
		events := wb.RecentEvents(hostID, 1*time.Minute)
		assert.Len(t, events, 10)
	}
}

func TestWindowBuffer_NilEvent(t *testing.T) {
	wb := NewWindowBuffer(1 * time.Hour)
	
	// Add nil event (should be ignored)
	wb.Add(nil)
	
	// Verify no events were added
	stats := wb.GetStats()
	assert.Equal(t, 0, stats["total_events"])
}

func TestWindowBuffer_EmptyHostID(t *testing.T) {
	wb := NewWindowBuffer(1 * time.Hour)
	
	// Add event with empty host ID (should be ignored)
	event := &model.Event{
		HostID:    "",
		EventType: "exec",
		BinaryPath: "/bin/bash",
	}
	wb.Add(event)
	
	// Verify no events were added
	stats := wb.GetStats()
	assert.Equal(t, 0, stats["total_events"])
}
