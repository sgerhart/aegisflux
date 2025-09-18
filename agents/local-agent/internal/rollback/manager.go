package rollback

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"aegisflux/agents/local-agent/internal/types"
)

// RollbackReason represents the reason for a rollback
type RollbackReason string

const (
	RollbackReasonTelemetryThreshold RollbackReason = "telemetry_threshold"
	RollbackReasonVerifierFailure    RollbackReason = "verifier_failure"
	RollbackReasonHighCPU            RollbackReason = "high_cpu"
	RollbackReasonOrchestratorSignal RollbackReason = "orchestrator_signal"
	RollbackReasonManual             RollbackReason = "manual"
	RollbackReasonTTLExpired         RollbackReason = "ttl_expired"
)

// RollbackThresholds defines thresholds for automatic rollback
type RollbackThresholds struct {
	MaxErrors          int     `json:"max_errors"`
	MaxViolations      int     `json:"max_violations"`
	MaxCPUPercent      float64 `json:"max_cpu_percent"`
	MaxLatencyMs       float64 `json:"max_latency_ms"`
	MaxMemoryKB        int64   `json:"max_memory_kb"`
	VerifierFailure    bool    `json:"verifier_failure"`
	CheckIntervalSec   int     `json:"check_interval_sec"`
	RollbackDelaySec   int     `json:"rollback_delay_sec"`
}

// RollbackEvent represents a rollback event
type RollbackEvent struct {
	Type          string         `json:"type"`
	ArtifactID    string         `json:"artifact_id"`
	HostID        string         `json:"host_id"`
	Reason        RollbackReason `json:"reason"`
	Threshold     string         `json:"threshold,omitempty"`
	Value         interface{}    `json:"value,omitempty"`
	Timestamp     string         `json:"timestamp"`
	Status        string         `json:"status"` // "initiated", "in_progress", "completed", "failed"
	Error         string         `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// RollbackManager manages rollback operations and monitoring
type RollbackManager struct {
	logger         *slog.Logger
	nc             *nats.Conn
	hostID         string
	thresholds     RollbackThresholds
	loadedPrograms map[string]*types.LoadedProgram
	programsMutex  sync.RWMutex
	rollbackQueue  chan RollbackEvent
	stopChan       chan struct{}
	callbacks      []RollbackCallback
	callbackMutex  sync.RWMutex
}

// RollbackCallback is a function called when rollback events occur
type RollbackCallback func(event RollbackEvent, program *types.LoadedProgram)

// NewRollbackManager creates a new rollback manager
func NewRollbackManager(logger *slog.Logger, nc *nats.Conn, hostID string, thresholds RollbackThresholds) *RollbackManager {
	return &RollbackManager{
		logger:         logger,
		nc:             nc,
		hostID:         hostID,
		thresholds:     thresholds,
		loadedPrograms: make(map[string]*types.LoadedProgram),
		rollbackQueue:  make(chan RollbackEvent, 100),
		stopChan:       make(chan struct{}),
		callbacks:      make([]RollbackCallback, 0),
	}
}

// Start starts the rollback manager
func (rm *RollbackManager) Start(ctx context.Context) error {
	rm.logger.Info("Starting rollback manager",
		"thresholds", rm.thresholds)

	// Subscribe to orchestrator signals
	if err := rm.subscribeToOrchestratorSignals(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to orchestrator signals: %w", err)
	}

	// Start monitoring loop
	go rm.monitoringLoop(ctx)

	// Start rollback processing loop
	go rm.rollbackProcessingLoop(ctx)

	return nil
}

// Stop stops the rollback manager
func (rm *RollbackManager) Stop() {
	rm.logger.Info("Stopping rollback manager")
	close(rm.stopChan)
}

// RegisterProgram registers a loaded program for monitoring
func (rm *RollbackManager) RegisterProgram(program *types.LoadedProgram) {
	rm.programsMutex.Lock()
	defer rm.programsMutex.Unlock()
	
	rm.loadedPrograms[program.ArtifactID] = program
	rm.logger.Debug("Program registered for rollback monitoring",
		"artifact_id", program.ArtifactID)
}

// UnregisterProgram unregisters a program from monitoring
func (rm *RollbackManager) UnregisterProgram(artifactID string) {
	rm.programsMutex.Lock()
	defer rm.programsMutex.Unlock()
	
	delete(rm.loadedPrograms, artifactID)
	rm.logger.Debug("Program unregistered from rollback monitoring",
		"artifact_id", artifactID)
}

// UpdateTelemetry updates telemetry data for a program
func (rm *RollbackManager) UpdateTelemetry(artifactID string, telemetry types.ProgramTelemetry) {
	rm.programsMutex.Lock()
	defer rm.programsMutex.Unlock()
	
	program, exists := rm.loadedPrograms[artifactID]
	if !exists {
		return
	}
	
	program.Telemetry = telemetry
	
	// Check thresholds immediately
	if rm.shouldRollback(program) {
		reason, threshold, value := rm.getRollbackReason(program)
		rm.triggerRollback(artifactID, reason, threshold, value)
	}
}

// RequestRollback manually requests a rollback for a specific program
func (rm *RollbackManager) RequestRollback(artifactID string, reason RollbackReason, metadata map[string]interface{}) error {
	rm.programsMutex.RLock()
	program, exists := rm.loadedPrograms[artifactID]
	rm.programsMutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("program not found: %s", artifactID)
	}
	
	// Use program variable to avoid unused variable warning
	_ = program
	
	rm.triggerRollback(artifactID, reason, "", nil)
	return nil
}

// triggerRollback triggers a rollback for a program
func (rm *RollbackManager) triggerRollback(artifactID string, reason RollbackReason, threshold string, value interface{}) {
	event := RollbackEvent{
		ArtifactID: artifactID,
		HostID:     rm.hostID,
		Reason:     reason,
		Threshold:  threshold,
		Value:      value,
		Timestamp:  time.Now().Format(time.RFC3339),
		Status:     "initiated",
		Metadata: map[string]interface{}{
			"host_id":     rm.hostID,
			"agent_version": "1.0.0",
		},
	}
	
	select {
	case rm.rollbackQueue <- event:
		rm.logger.Info("Rollback triggered",
			"artifact_id", artifactID,
			"reason", reason,
			"threshold", threshold)
	default:
		rm.logger.Error("Rollback queue is full",
			"artifact_id", artifactID,
			"reason", reason)
	}
}

// shouldRollback checks if a program should be rolled back based on thresholds
func (rm *RollbackManager) shouldRollback(program *types.LoadedProgram) bool {
	telemetry := program.Telemetry
	
	// Check error threshold
	if telemetry.Errors > int64(rm.thresholds.MaxErrors) {
		return true
	}
	
	// Check violation threshold
	if telemetry.Violations > int64(rm.thresholds.MaxViolations) {
		return true
	}
	
	// Check CPU threshold
	if telemetry.CPUPercent > rm.thresholds.MaxCPUPercent {
		return true
	}
	
	// Check latency threshold
	if telemetry.LatencyMs > rm.thresholds.MaxLatencyMs {
		return true
	}
	
	// Check memory threshold
	if telemetry.MemoryKB > rm.thresholds.MaxMemoryKB {
		return true
	}
	
	// Check verifier failure
	if rm.thresholds.VerifierFailure && telemetry.VerifierMsg != nil && *telemetry.VerifierMsg != "" {
		return true
	}
	
	return false
}

// getRollbackReason determines the specific reason for rollback
func (rm *RollbackManager) getRollbackReason(program *types.LoadedProgram) (RollbackReason, string, interface{}) {
	telemetry := program.Telemetry
	
	if telemetry.Errors > int64(rm.thresholds.MaxErrors) {
		return RollbackReasonTelemetryThreshold, "max_errors", telemetry.Errors
	}
	
	if telemetry.Violations > int64(rm.thresholds.MaxViolations) {
		return RollbackReasonTelemetryThreshold, "max_violations", telemetry.Violations
	}
	
	if telemetry.CPUPercent > rm.thresholds.MaxCPUPercent {
		return RollbackReasonHighCPU, "max_cpu_percent", telemetry.CPUPercent
	}
	
	if telemetry.LatencyMs > rm.thresholds.MaxLatencyMs {
		return RollbackReasonTelemetryThreshold, "max_latency_ms", telemetry.LatencyMs
	}
	
	if telemetry.MemoryKB > rm.thresholds.MaxMemoryKB {
		return RollbackReasonTelemetryThreshold, "max_memory_kb", telemetry.MemoryKB
	}
	
	if rm.thresholds.VerifierFailure && telemetry.VerifierMsg != nil && *telemetry.VerifierMsg != "" {
		return RollbackReasonVerifierFailure, "verifier_failure", *telemetry.VerifierMsg
	}
	
	return RollbackReasonTelemetryThreshold, "unknown", nil
}

// monitoringLoop continuously monitors programs for threshold violations
func (rm *RollbackManager) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(rm.thresholds.CheckIntervalSec) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			rm.logger.Info("Rollback monitoring context cancelled")
			return
			
		case <-rm.stopChan:
			rm.logger.Info("Rollback monitoring stopped")
			return
			
		case <-ticker.C:
			rm.checkAllPrograms()
		}
	}
}

// checkAllPrograms checks all registered programs for threshold violations
func (rm *RollbackManager) checkAllPrograms() {
	rm.programsMutex.RLock()
	programs := make([]*types.LoadedProgram, 0, len(rm.loadedPrograms))
	for _, program := range rm.loadedPrograms {
		programs = append(programs, program)
	}
	rm.programsMutex.RUnlock()
	
	for _, program := range programs {
		if rm.shouldRollback(program) {
			reason, threshold, value := rm.getRollbackReason(program)
			rm.triggerRollback(program.ArtifactID, reason, threshold, value)
		}
	}
}

// rollbackProcessingLoop processes rollback events
func (rm *RollbackManager) rollbackProcessingLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			rm.logger.Info("Rollback processing context cancelled")
			return
			
		case <-rm.stopChan:
			rm.logger.Info("Rollback processing stopped")
			return
			
		case event := <-rm.rollbackQueue:
			rm.processRollbackEvent(event)
		}
	}
}

// processRollbackEvent processes a single rollback event
func (rm *RollbackManager) processRollbackEvent(event RollbackEvent) {
	rm.logger.Info("Processing rollback event",
		"artifact_id", event.ArtifactID,
		"reason", event.Reason,
		"threshold", event.Threshold)
	
	event.Status = "in_progress"
	
	// Get program information
	rm.programsMutex.RLock()
	program, exists := rm.loadedPrograms[event.ArtifactID]
	rm.programsMutex.RUnlock()
	
	if !exists {
		event.Status = "failed"
		event.Error = "program not found"
		rm.logger.Error("Program not found for rollback",
			"artifact_id", event.ArtifactID)
	} else {
		// Notify callbacks
		rm.notifyCallbacks(event, program)
		
		event.Status = "completed"
		rm.logger.Info("Rollback event processed",
			"artifact_id", event.ArtifactID,
			"reason", event.Reason)
	}
	
	// Emit rollback status
	rm.emitRollbackStatus(event)
}

// subscribeToOrchestratorSignals subscribes to orchestrator rollback signals
func (rm *RollbackManager) subscribeToOrchestratorSignals(ctx context.Context) error {
	// Subscribe to rollback signals from orchestrator
	subject := fmt.Sprintf("orchestrator.rollback.%s", rm.hostID)
	
	subscription, err := rm.nc.Subscribe(subject, func(msg *nats.Msg) {
		rm.handleOrchestratorSignal(msg)
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to orchestrator signals: %w", err)
	}
	
	rm.logger.Info("Subscribed to orchestrator rollback signals",
		"subject", subject)
	
	// Cleanup subscription on context cancellation
	go func() {
		<-ctx.Done()
		subscription.Unsubscribe()
	}()
	
	return nil
}

// handleOrchestratorSignal handles rollback signals from orchestrator
func (rm *RollbackManager) handleOrchestratorSignal(msg *nats.Msg) {
	var signal map[string]interface{}
	if err := json.Unmarshal(msg.Data, &signal); err != nil {
		rm.logger.Error("Failed to unmarshal orchestrator signal", "error", err)
		return
	}
	
	artifactID, ok := signal["artifact_id"].(string)
	if !ok {
		rm.logger.Error("Invalid orchestrator signal: missing artifact_id")
		return
	}
	
	reason := signal["reason"].(string)
	if reason == "" {
		reason = "orchestrator_signal"
	}
	
	rm.logger.Info("Received orchestrator rollback signal",
		"artifact_id", artifactID,
		"reason", reason)
	
	rm.triggerRollback(artifactID, RollbackReasonOrchestratorSignal, "", nil)
}

// emitRollbackStatus emits rollback status to NATS
func (rm *RollbackManager) emitRollbackStatus(event RollbackEvent) {
	event.Type = "rollback_status"
	event.HostID = rm.hostID
	
	data, err := json.Marshal(event)
	if err != nil {
		rm.logger.Error("Failed to marshal rollback status", "error", err)
		return
	}
	
	subject := "agent.rollback.status"
	if err := rm.nc.Publish(subject, data); err != nil {
		rm.logger.Error("Failed to publish rollback status", "error", err)
		return
	}
	
	rm.logger.Debug("Emitted rollback status",
		"subject", subject,
		"artifact_id", event.ArtifactID,
		"status", event.Status)
}

// AddCallback adds a rollback callback
func (rm *RollbackManager) AddCallback(callback RollbackCallback) {
	rm.callbackMutex.Lock()
	defer rm.callbackMutex.Unlock()
	rm.callbacks = append(rm.callbacks, callback)
}

// notifyCallbacks notifies all registered callbacks
func (rm *RollbackManager) notifyCallbacks(event RollbackEvent, program *types.LoadedProgram) {
	rm.callbackMutex.RLock()
	defer rm.callbackMutex.RUnlock()
	
	for _, callback := range rm.callbacks {
		go callback(event, program)
	}
}

// GetRollbackHistory returns recent rollback events (for debugging)
func (rm *RollbackManager) GetRollbackHistory() []RollbackEvent {
	// In a real implementation, you'd store rollback history
	// For now, return empty slice
	return []RollbackEvent{}
}

// UpdateThresholds updates rollback thresholds
func (rm *RollbackManager) UpdateThresholds(thresholds RollbackThresholds) {
	rm.thresholds = thresholds
	rm.logger.Info("Rollback thresholds updated", "thresholds", thresholds)
}
