package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"aegisflux/agents/local-agent/internal/rollback"
	"aegisflux/agents/local-agent/internal/types"
)

// TelemetryMonitor monitors telemetry data and triggers rollbacks
type TelemetryMonitor struct {
	logger           *slog.Logger
	nc               *nats.Conn
	hostID           string
	rollbackManager  *rollback.RollbackManager
	thresholds       TelemetryThresholds
	monitoringData   map[string]*types.ProgramTelemetry
	dataMutex        sync.RWMutex
	stopChan         chan struct{}
	callbacks        []TelemetryCallback
	callbackMutex    sync.RWMutex
}

// TelemetryThresholds defines thresholds for monitoring
type TelemetryThresholds struct {
	MaxErrors          int     `json:"max_errors"`
	MaxViolations      int     `json:"max_violations"`
	MaxCPUPercent      float64 `json:"max_cpu_percent"`
	MaxLatencyMs       float64 `json:"max_latency_ms"`
	MaxMemoryKB        int64   `json:"max_memory_kb"`
	VerifierFailure    bool    `json:"verifier_failure"`
	CheckIntervalSec   int     `json:"check_interval_sec"`
	RollbackDelaySec   int     `json:"rollback_delay_sec"`
}

// TelemetryCallback is a function called when telemetry thresholds are exceeded
type TelemetryCallback func(artifactID string, telemetry *types.ProgramTelemetry, threshold string, value interface{})

// NewTelemetryMonitor creates a new telemetry monitor
func NewTelemetryMonitor(logger *slog.Logger, nc *nats.Conn, hostID string, rollbackManager *rollback.RollbackManager, thresholds TelemetryThresholds) *TelemetryMonitor {
	return &TelemetryMonitor{
		logger:          logger,
		nc:              nc,
		hostID:          hostID,
		rollbackManager: rollbackManager,
		thresholds:      thresholds,
		monitoringData:  make(map[string]*types.ProgramTelemetry),
		stopChan:        make(chan struct{}),
		callbacks:       make([]TelemetryCallback, 0),
	}
}

// Start starts the telemetry monitor
func (tm *TelemetryMonitor) Start(ctx context.Context) error {
	tm.logger.Info("Starting telemetry monitor",
		"thresholds", tm.thresholds)

	// Subscribe to telemetry events
	if err := tm.subscribeToTelemetryEvents(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to telemetry events: %w", err)
	}

	// Start monitoring loop
	go tm.monitoringLoop(ctx)

	return nil
}

// Stop stops the telemetry monitor
func (tm *TelemetryMonitor) Stop() {
	tm.logger.Info("Stopping telemetry monitor")
	close(tm.stopChan)
}

// UpdateTelemetry updates telemetry data for a program
func (tm *TelemetryMonitor) UpdateTelemetry(artifactID string, telemetry *types.ProgramTelemetry) {
	tm.dataMutex.Lock()
	tm.monitoringData[artifactID] = telemetry
	tm.dataMutex.Unlock()

	// Check thresholds immediately
	if tm.checkThresholds(telemetry) {
		threshold, value := tm.getExceededThreshold(telemetry)
		tm.triggerRollback(artifactID, telemetry, threshold, value)
	}

	// Update rollback manager
	tm.rollbackManager.UpdateTelemetry(artifactID, *telemetry)
}

// checkThresholds checks if any thresholds are exceeded
func (tm *TelemetryMonitor) checkThresholds(telemetry *types.ProgramTelemetry) bool {
	if telemetry.Errors > int64(tm.thresholds.MaxErrors) {
		return true
	}
	
	if telemetry.Violations > int64(tm.thresholds.MaxViolations) {
		return true
	}
	
	if telemetry.CPUPercent > tm.thresholds.MaxCPUPercent {
		return true
	}
	
	if telemetry.LatencyMs > tm.thresholds.MaxLatencyMs {
		return true
	}
	
	if telemetry.MemoryKB > tm.thresholds.MaxMemoryKB {
		return true
	}
	
	if tm.thresholds.VerifierFailure && telemetry.VerifierMsg != nil && *telemetry.VerifierMsg != "" {
		return true
	}
	
	return false
}

// getExceededThreshold returns the first exceeded threshold and its value
func (tm *TelemetryMonitor) getExceededThreshold(telemetry *types.ProgramTelemetry) (string, interface{}) {
	if telemetry.Errors > int64(tm.thresholds.MaxErrors) {
		return "max_errors", telemetry.Errors
	}
	
	if telemetry.Violations > int64(tm.thresholds.MaxViolations) {
		return "max_violations", telemetry.Violations
	}
	
	if telemetry.CPUPercent > tm.thresholds.MaxCPUPercent {
		return "max_cpu_percent", telemetry.CPUPercent
	}
	
	if telemetry.LatencyMs > tm.thresholds.MaxLatencyMs {
		return "max_latency_ms", telemetry.LatencyMs
	}
	
	if telemetry.MemoryKB > tm.thresholds.MaxMemoryKB {
		return "max_memory_kb", telemetry.MemoryKB
	}
	
	if tm.thresholds.VerifierFailure && telemetry.VerifierMsg != nil && *telemetry.VerifierMsg != "" {
		return "verifier_failure", *telemetry.VerifierMsg
	}
	
	return "unknown", nil
}

// triggerRollback triggers a rollback for a program
func (tm *TelemetryMonitor) triggerRollback(artifactID string, telemetry *types.ProgramTelemetry, threshold string, value interface{}) {
	tm.logger.Warn("Telemetry threshold exceeded, triggering rollback",
		"artifact_id", artifactID,
		"threshold", threshold,
		"value", value,
		"errors", telemetry.Errors,
		"violations", telemetry.Violations,
		"cpu_percent", telemetry.CPUPercent,
		"latency_ms", telemetry.LatencyMs,
		"memory_kb", telemetry.MemoryKB)

	// Notify callbacks
	tm.notifyCallbacks(artifactID, telemetry, threshold, value)

	// Trigger rollback through rollback manager
	reason := rollback.RollbackReasonTelemetryThreshold
	if threshold == "max_cpu_percent" {
		reason = rollback.RollbackReasonHighCPU
	} else if threshold == "verifier_failure" {
		reason = rollback.RollbackReasonVerifierFailure
	}

	tm.rollbackManager.RequestRollback(artifactID, reason, map[string]interface{}{
		"threshold": threshold,
		"value":     value,
		"telemetry": telemetry,
	})
}

// monitoringLoop continuously monitors telemetry data
func (tm *TelemetryMonitor) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(tm.thresholds.CheckIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			tm.logger.Info("Telemetry monitoring context cancelled")
			return

		case <-tm.stopChan:
			tm.logger.Info("Telemetry monitoring stopped")
			return

		case <-ticker.C:
			tm.checkAllPrograms()
		}
	}
}

// checkAllPrograms checks all monitored programs for threshold violations
func (tm *TelemetryMonitor) checkAllPrograms() {
	tm.dataMutex.RLock()
	programs := make([]*types.ProgramTelemetry, 0, len(tm.monitoringData))
	artifactIDs := make([]string, 0, len(tm.monitoringData))
	
	for artifactID, telemetry := range tm.monitoringData {
		programs = append(programs, telemetry)
		artifactIDs = append(artifactIDs, artifactID)
	}
	tm.dataMutex.RUnlock()

	for i, telemetry := range programs {
		artifactID := artifactIDs[i]
		
		if tm.checkThresholds(telemetry) {
			threshold, value := tm.getExceededThreshold(telemetry)
			tm.triggerRollback(artifactID, telemetry, threshold, value)
		}
	}
}

// subscribeToTelemetryEvents subscribes to telemetry events from programs
func (tm *TelemetryMonitor) subscribeToTelemetryEvents(ctx context.Context) error {
	// Subscribe to program telemetry events
	subject := fmt.Sprintf("agent.telemetry.%s", tm.hostID)
	
	subscription, err := tm.nc.Subscribe(subject, func(msg *nats.Msg) {
		tm.handleTelemetryEvent(msg)
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to telemetry events: %w", err)
	}

	tm.logger.Info("Subscribed to telemetry events",
		"subject", subject)

	// Cleanup subscription on context cancellation
	go func() {
		<-ctx.Done()
		subscription.Unsubscribe()
	}()

	return nil
}

// handleTelemetryEvent handles telemetry events from programs
func (tm *TelemetryMonitor) handleTelemetryEvent(msg *nats.Msg) {
	var event types.TelemetryEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		tm.logger.Error("Failed to unmarshal telemetry event", "error", err)
		return
	}

	if event.Type == "program_telemetry" {
		tm.UpdateTelemetry(event.Data.ArtifactID, &event.Data)
	}
}

// AddCallback adds a telemetry callback
func (tm *TelemetryMonitor) AddCallback(callback TelemetryCallback) {
	tm.callbackMutex.Lock()
	defer tm.callbackMutex.Unlock()
	tm.callbacks = append(tm.callbacks, callback)
}

// notifyCallbacks notifies all registered callbacks
func (tm *TelemetryMonitor) notifyCallbacks(artifactID string, telemetry *types.ProgramTelemetry, threshold string, value interface{}) {
	tm.callbackMutex.RLock()
	defer tm.callbackMutex.RUnlock()

	for _, callback := range tm.callbacks {
		go callback(artifactID, telemetry, threshold, value)
	}
}

// GetMonitoringData returns current monitoring data
func (tm *TelemetryMonitor) GetMonitoringData() map[string]*types.ProgramTelemetry {
	tm.dataMutex.RLock()
	defer tm.dataMutex.RUnlock()

	data := make(map[string]*types.ProgramTelemetry)
	for artifactID, telemetry := range tm.monitoringData {
		data[artifactID] = telemetry
	}

	return data
}

// UpdateThresholds updates monitoring thresholds
func (tm *TelemetryMonitor) UpdateThresholds(thresholds TelemetryThresholds) {
	tm.thresholds = thresholds
	tm.logger.Info("Telemetry thresholds updated", "thresholds", thresholds)
}

// EmitThresholdAlert emits an alert when thresholds are exceeded
func (tm *TelemetryMonitor) EmitThresholdAlert(artifactID string, telemetry *types.ProgramTelemetry, threshold string, value interface{}) {
	alert := map[string]interface{}{
		"type":        "threshold_alert",
		"artifact_id": artifactID,
		"host_id":     tm.hostID,
		"threshold":   threshold,
		"value":       value,
		"telemetry":   telemetry,
		"timestamp":   time.Now().Format(time.RFC3339),
		"metadata": map[string]interface{}{
			"host_id":       tm.hostID,
			"agent_version": "1.0.0",
		},
	}

	data, err := json.Marshal(alert)
	if err != nil {
		tm.logger.Error("Failed to marshal threshold alert", "error", err)
		return
	}

	subject := "agent.alerts.threshold"
	if err := tm.nc.Publish(subject, data); err != nil {
		tm.logger.Error("Failed to publish threshold alert", "error", err)
		return
	}

	tm.logger.Debug("Emitted threshold alert",
		"subject", subject,
		"artifact_id", artifactID,
		"threshold", threshold)
}
