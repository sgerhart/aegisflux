package rollout

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"log/slog"
)

// TelemetryMonitor monitors telemetry data from agents
type TelemetryMonitor struct {
	logger        *slog.Logger
	nc            *nats.Conn
	subscriptions map[string]*nats.Subscription
	telemetryData map[string]*TelemetryData
	dataMutex     sync.RWMutex
	callbacks     []TelemetryCallback
	callbackMutex sync.RWMutex
}

// TelemetryCallback is a function called when telemetry data is received
type TelemetryCallback func(targetID string, telemetry *TelemetryData)

// TelemetryFilter defines filtering criteria for telemetry data
type TelemetryFilter struct {
	TargetIDs    []string          `json:"target_ids,omitempty"`
	MinViolations int              `json:"min_violations,omitempty"`
	MaxLatency   float64           `json:"max_latency,omitempty"`
	MinPacketCount int64           `json:"min_packet_count,omitempty"`
	AgentVersion string            `json:"agent_version,omitempty"`
	CustomFilters map[string]interface{} `json:"custom_filters,omitempty"`
}

// TelemetryAggregation contains aggregated telemetry data
type TelemetryAggregation struct {
	TargetID       string    `json:"target_id"`
	TotalViolations int      `json:"total_violations"`
	TotalErrors    int       `json:"total_errors"`
	TotalPackets   int64     `json:"total_packets"`
	TotalBlocks    int64     `json:"total_blocks"`
	AverageLatency float64   `json:"average_latency"`
	MaxLatency     float64   `json:"max_latency"`
	MinLatency     float64   `json:"min_latency"`
	LastUpdate     time.Time `json:"last_update"`
	DataPoints     int       `json:"data_points"`
}

// NewTelemetryMonitor creates a new telemetry monitor
func NewTelemetryMonitor(logger *slog.Logger, nc *nats.Conn) *TelemetryMonitor {
	return &TelemetryMonitor{
		logger:        logger,
		nc:            nc,
		subscriptions: make(map[string]*nats.Subscription),
		telemetryData: make(map[string]*TelemetryData),
		callbacks:     make([]TelemetryCallback, 0),
	}
}

// StartMonitoring starts monitoring telemetry from agents
func (tm *TelemetryMonitor) StartMonitoring(ctx context.Context) error {
	tm.logger.Info("Starting telemetry monitoring")

	// Subscribe to general telemetry
	subject := "agent.telemetry"
	sub, err := tm.nc.Subscribe(subject, func(msg *nats.Msg) {
		tm.handleTelemetryMessage(msg)
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to telemetry: %w", err)
	}

	tm.subscriptions[subject] = sub
	tm.logger.Info("Subscribed to telemetry", "subject", subject)

	// Subscribe to specific target telemetry patterns
	specificSubject := "agent.telemetry.*"
	subSpecific, err := tm.nc.Subscribe(specificSubject, func(msg *nats.Msg) {
		tm.handleTelemetryMessage(msg)
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to specific telemetry: %w", err)
	}

	tm.subscriptions[specificSubject] = subSpecific
	tm.logger.Info("Subscribed to specific telemetry", "subject", specificSubject)

	return nil
}

// StopMonitoring stops all telemetry monitoring
func (tm *TelemetryMonitor) StopMonitoring() {
	tm.logger.Info("Stopping telemetry monitoring")

	tm.dataMutex.Lock()
	defer tm.dataMutex.Unlock()

	for subject, sub := range tm.subscriptions {
		if err := sub.Unsubscribe(); err != nil {
			tm.logger.Error("Failed to unsubscribe", "subject", subject, "error", err)
		}
	}

	tm.subscriptions = make(map[string]*nats.Subscription)
}

// handleTelemetryMessage processes incoming telemetry messages
func (tm *TelemetryMonitor) handleTelemetryMessage(msg *nats.Msg) {
	var telemetry TelemetryData
	if err := json.Unmarshal(msg.Data, &telemetry); err != nil {
		tm.logger.Error("Failed to unmarshal telemetry", "error", err)
		return
	}

	// Extract target ID from subject if available
	targetID := tm.extractTargetID(msg.Subject)
	if targetID == "" {
		targetID = "unknown"
	}

	// Update telemetry data
	tm.dataMutex.Lock()
	tm.telemetryData[targetID] = &telemetry
	tm.dataMutex.Unlock()

	// Notify callbacks
	tm.callbackMutex.RLock()
	for _, callback := range tm.callbacks {
		go callback(targetID, &telemetry)
	}
	tm.callbackMutex.RUnlock()

	tm.logger.Debug("Received telemetry",
		"target_id", targetID,
		"violations", telemetry.Violations,
		"errors", telemetry.Errors,
		"latency", telemetry.Latency)
}

// extractTargetID extracts target ID from NATS subject
func (tm *TelemetryMonitor) extractTargetID(subject string) string {
	// Handle patterns like "agent.telemetry.host-123" or "agent.telemetry"
	parts := strings.Split(subject, ".")
	if len(parts) >= 3 && parts[2] != "" {
		return parts[2]
	}
	return ""
}

// AddCallback adds a callback function for telemetry events
func (tm *TelemetryMonitor) AddCallback(callback TelemetryCallback) {
	tm.callbackMutex.Lock()
	defer tm.callbackMutex.Unlock()
	tm.callbacks = append(tm.callbacks, callback)
}

// GetTelemetryData retrieves telemetry data for a target
func (tm *TelemetryMonitor) GetTelemetryData(targetID string) (*TelemetryData, error) {
	tm.dataMutex.RLock()
	defer tm.dataMutex.RUnlock()

	data, exists := tm.telemetryData[targetID]
	if !exists {
		return nil, fmt.Errorf("no telemetry data for target: %s", targetID)
	}

	return data, nil
}

// GetAllTelemetryData retrieves all telemetry data
func (tm *TelemetryMonitor) GetAllTelemetryData() map[string]*TelemetryData {
	tm.dataMutex.RLock()
	defer tm.dataMutex.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]*TelemetryData)
	for k, v := range tm.telemetryData {
		result[k] = v
	}

	return result
}

// GetTelemetryAggregation calculates aggregated telemetry data
func (tm *TelemetryMonitor) GetTelemetryAggregation(targetID string, duration time.Duration) (*TelemetryAggregation, error) {
	tm.dataMutex.RLock()
	defer tm.dataMutex.RUnlock()

	data, exists := tm.telemetryData[targetID]
	if !exists {
		return nil, fmt.Errorf("no telemetry data for target: %s", targetID)
	}

	// In a real implementation, you'd aggregate data over time
	// For now, we'll return the current data point
	aggregation := &TelemetryAggregation{
		TargetID:        targetID,
		TotalViolations: data.Violations,
		TotalErrors:     data.Errors,
		TotalPackets:    data.PacketCount,
		TotalBlocks:     data.BlockCount,
		AverageLatency:  data.Latency,
		MaxLatency:      data.Latency,
		MinLatency:      data.Latency,
		LastUpdate:      data.LastUpdate,
		DataPoints:      1,
	}

	return aggregation, nil
}

// FilterTelemetryData filters telemetry data based on criteria
func (tm *TelemetryMonitor) FilterTelemetryData(filter TelemetryFilter) map[string]*TelemetryData {
	tm.dataMutex.RLock()
	defer tm.dataMutex.RUnlock()

	filtered := make(map[string]*TelemetryData)

	for targetID, data := range tm.telemetryData {
		if tm.matchesFilter(targetID, data, filter) {
			filtered[targetID] = data
		}
	}

	return filtered
}

// matchesFilter checks if telemetry data matches the filter criteria
func (tm *TelemetryMonitor) matchesFilter(targetID string, data *TelemetryData, filter TelemetryFilter) bool {
	// Check target ID filter
	if len(filter.TargetIDs) > 0 {
		found := false
		for _, id := range filter.TargetIDs {
			if id == targetID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check violations filter
	if filter.MinViolations > 0 && data.Violations < filter.MinViolations {
		return false
	}

	// Check latency filter
	if filter.MaxLatency > 0 && data.Latency > filter.MaxLatency {
		return false
	}

	// Check packet count filter
	if filter.MinPacketCount > 0 && data.PacketCount < filter.MinPacketCount {
		return false
	}

	// Check agent version filter
	if filter.AgentVersion != "" && data.AgentVersion != filter.AgentVersion {
		return false
	}

	return true
}

// PublishTelemetryRequest publishes a request for telemetry data
func (tm *TelemetryMonitor) PublishTelemetryRequest(targets []string, requestType string) error {
	request := map[string]interface{}{
		"type":       requestType,
		"targets":    targets,
		"timestamp":  time.Now(),
		"request_id": fmt.Sprintf("req-%d", time.Now().UnixNano()),
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry request: %w", err)
	}

	subject := "agent.telemetry.request"
	if err := tm.nc.Publish(subject, requestJSON); err != nil {
		return fmt.Errorf("failed to publish telemetry request: %w", err)
	}

	tm.logger.Info("Published telemetry request",
		"subject", subject,
		"targets", targets,
		"type", requestType)

	return nil
}

// WatchForViolations watches for telemetry violations and calls callback
func (tm *TelemetryMonitor) WatchForViolations(threshold int, callback func(targetID string, violations int)) {
	tm.AddCallback(func(targetID string, telemetry *TelemetryData) {
		if telemetry.Violations >= threshold {
			callback(targetID, telemetry.Violations)
		}
	})
}

// WatchForErrors watches for telemetry errors and calls callback
func (tm *TelemetryMonitor) WatchForErrors(threshold int, callback func(targetID string, errors int)) {
	tm.AddCallback(func(targetID string, telemetry *TelemetryData) {
		if telemetry.Errors >= threshold {
			callback(targetID, telemetry.Errors)
		}
	})
}

// WatchForLatency watches for high latency and calls callback
func (tm *TelemetryMonitor) WatchForLatency(threshold float64, callback func(targetID string, latency float64)) {
	tm.AddCallback(func(targetID string, telemetry *TelemetryData) {
		if telemetry.Latency >= threshold {
			callback(targetID, telemetry.Latency)
		}
	})
}
