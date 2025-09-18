package rollout

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"log/slog"
)

// RollbackStrategy defines the rollback strategy
type RollbackStrategy int

const (
	// RollbackStrategyImmediate immediately rolls back all targets
	RollbackStrategyImmediate RollbackStrategy = iota
	// RollbackStrategyGradual rolls back targets gradually
	RollbackStrategyGradual
	// RollbackStrategySelective rolls back only failed targets
	RollbackStrategySelective
)

// RollbackRequest represents a rollback request
type RollbackRequest struct {
	RequestID     string           `json:"request_id"`
	Strategy      RollbackStrategy `json:"strategy"`
	Targets       []string         `json:"targets,omitempty"`
	Reason        string           `json:"reason"`
	Force         bool             `json:"force,omitempty"`
	Timeout       time.Duration    `json:"timeout,omitempty"`
}

// RollbackResult represents the result of a rollback operation
type RollbackResult struct {
	RequestID      string            `json:"request_id"`
	Status         string            `json:"status"` // "initiated", "in_progress", "completed", "failed"
	Strategy       RollbackStrategy  `json:"strategy"`
	TargetResults  []TargetRollback  `json:"target_results"`
	StartTime      time.Time         `json:"start_time"`
	EndTime        *time.Time        `json:"end_time,omitempty"`
	Reason         string            `json:"reason"`
	ErrorMessage   string            `json:"error_message,omitempty"`
}

// TargetRollback represents the rollback result for a specific target
type TargetRollback struct {
	TargetID     string    `json:"target_id"`
	Status       string    `json:"status"` // "pending", "rolling_back", "completed", "failed"
	StartTime    time.Time `json:"start_time"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	Error        string    `json:"error,omitempty"`
	ArtifactID   string    `json:"artifact_id,omitempty"`
}

// RollbackManager manages rollback operations
type RollbackManager struct {
	logger         *slog.Logger
	nc             *nats.Conn
	activeRollbacks map[string]*RollbackResult
	rollbackMutex  sync.RWMutex
	callbacks      []RollbackCallback
	callbackMutex  sync.RWMutex
}

// RollbackCallback is a function called when rollback events occur
type RollbackCallback func(requestID string, result *RollbackResult)

// NewRollbackManager creates a new rollback manager
func NewRollbackManager(logger *slog.Logger, nc *nats.Conn) *RollbackManager {
	return &RollbackManager{
		logger:          logger,
		nc:              nc,
		activeRollbacks: make(map[string]*RollbackResult),
		callbacks:       make([]RollbackCallback, 0),
	}
}

// InitiateRollback initiates a rollback operation
func (rm *RollbackManager) InitiateRollback(ctx context.Context, req RollbackRequest) (*RollbackResult, error) {
	rm.logger.Info("Initiating rollback",
		"request_id", req.RequestID,
		"strategy", req.Strategy,
		"targets_count", len(req.Targets),
		"reason", req.Reason)

	// Create rollback result
	result := &RollbackResult{
		RequestID:     req.RequestID,
		Status:        "initiated",
		Strategy:      req.Strategy,
		StartTime:     time.Now(),
		Reason:        req.Reason,
		TargetResults: make([]TargetRollback, len(req.Targets)),
	}

	// Initialize target results
	for i, targetID := range req.Targets {
		result.TargetResults[i] = TargetRollback{
			TargetID:  targetID,
			Status:    "pending",
			StartTime: time.Now(),
		}
	}

	// Store active rollback
	rm.rollbackMutex.Lock()
	rm.activeRollbacks[req.RequestID] = result
	rm.rollbackMutex.Unlock()

	// Execute rollback based on strategy
	switch req.Strategy {
	case RollbackStrategyImmediate:
		go rm.executeImmediateRollback(ctx, req, result)
	case RollbackStrategyGradual:
		go rm.executeGradualRollback(ctx, req, result)
	case RollbackStrategySelective:
		go rm.executeSelectiveRollback(ctx, req, result)
	default:
		return nil, fmt.Errorf("unknown rollback strategy: %d", req.Strategy)
	}

	return result, nil
}

// executeImmediateRollback executes immediate rollback of all targets
func (rm *RollbackManager) executeImmediateRollback(ctx context.Context, req RollbackRequest, result *RollbackResult) {
	rm.logger.Info("Executing immediate rollback", "request_id", req.RequestID)

	rm.updateRollbackStatus(req.RequestID, "in_progress")

	// Roll back all targets simultaneously
	var wg sync.WaitGroup
	for i := range result.TargetResults {
		wg.Add(1)
		go func(targetIdx int) {
			defer wg.Done()
			rm.rollbackTarget(ctx, req.RequestID, &result.TargetResults[targetIdx])
		}(i)
	}

	wg.Wait()

	// Check if all rollbacks completed successfully
	allSuccess := true
	for _, target := range result.TargetResults {
		if target.Status != "completed" {
			allSuccess = false
			break
		}
	}

	if allSuccess {
		rm.updateRollbackStatus(req.RequestID, "completed")
		rm.logger.Info("Immediate rollback completed successfully", "request_id", req.RequestID)
	} else {
		rm.updateRollbackStatus(req.RequestID, "failed")
		rm.logger.Error("Immediate rollback failed", "request_id", req.RequestID)
	}

	endTime := time.Now()
	rm.setRollbackEndTime(req.RequestID, &endTime)
}

// executeGradualRollback executes gradual rollback of targets
func (rm *RollbackManager) executeGradualRollback(ctx context.Context, req RollbackRequest, result *RollbackResult) {
	rm.logger.Info("Executing gradual rollback", "request_id", req.RequestID)

	rm.updateRollbackStatus(req.RequestID, "in_progress")

	// Roll back targets in batches with delay between batches
	batchSize := 3 // Roll back 3 targets at a time
	if len(result.TargetResults) < batchSize {
		batchSize = len(result.TargetResults)
	}

	for i := 0; i < len(result.TargetResults); i += batchSize {
		end := i + batchSize
		if end > len(result.TargetResults) {
			end = len(result.TargetResults)
		}

		rm.logger.Info("Rolling back batch",
			"request_id", req.RequestID,
			"batch_start", i,
			"batch_end", end)

		// Roll back batch
		var wg sync.WaitGroup
		for j := i; j < end; j++ {
			wg.Add(1)
			go func(targetIdx int) {
				defer wg.Done()
				rm.rollbackTarget(ctx, req.RequestID, &result.TargetResults[targetIdx])
			}(j)
		}

		wg.Wait()

		// Wait between batches (except for the last batch)
		if end < len(result.TargetResults) {
			time.Sleep(30 * time.Second)
		}
	}

	// Check overall success
	allSuccess := true
	for _, target := range result.TargetResults {
		if target.Status != "completed" {
			allSuccess = false
			break
		}
	}

	if allSuccess {
		rm.updateRollbackStatus(req.RequestID, "completed")
		rm.logger.Info("Gradual rollback completed successfully", "request_id", req.RequestID)
	} else {
		rm.updateRollbackStatus(req.RequestID, "failed")
		rm.logger.Error("Gradual rollback failed", "request_id", req.RequestID)
	}

	endTime := time.Now()
	rm.setRollbackEndTime(req.RequestID, &endTime)
}

// executeSelectiveRollback executes selective rollback of only failed targets
func (rm *RollbackManager) executeSelectiveRollback(ctx context.Context, req RollbackRequest, result *RollbackResult) {
	rm.logger.Info("Executing selective rollback", "request_id", req.RequestID)

	rm.updateRollbackStatus(req.RequestID, "in_progress")

	// Identify failed targets (this would typically come from telemetry data)
	failedTargets := make([]*TargetRollback, 0)
	for i := range result.TargetResults {
		// In a real implementation, you'd check telemetry or status
		// For now, we'll assume all targets need rollback
		failedTargets = append(failedTargets, &result.TargetResults[i])
	}

	rm.logger.Info("Selective rollback targets identified",
		"request_id", req.RequestID,
		"failed_count", len(failedTargets))

	// Roll back only failed targets
	var wg sync.WaitGroup
	for _, target := range failedTargets {
		wg.Add(1)
		go func(t *TargetRollback) {
			defer wg.Done()
			rm.rollbackTarget(ctx, req.RequestID, t)
		}(target)
	}

	wg.Wait()

	// Check success
	allSuccess := true
	for _, target := range failedTargets {
		if target.Status != "completed" {
			allSuccess = false
			break
		}
	}

	if allSuccess {
		rm.updateRollbackStatus(req.RequestID, "completed")
		rm.logger.Info("Selective rollback completed successfully", "request_id", req.RequestID)
	} else {
		rm.updateRollbackStatus(req.RequestID, "failed")
		rm.logger.Error("Selective rollback failed", "request_id", req.RequestID)
	}

	endTime := time.Now()
	rm.setRollbackEndTime(req.RequestID, &endTime)
}

// rollbackTarget rolls back a specific target
func (rm *RollbackManager) rollbackTarget(ctx context.Context, requestID string, target *TargetRollback) {
	rm.logger.Info("Rolling back target",
		"request_id", requestID,
		"target_id", target.TargetID)

	target.Status = "rolling_back"

	// Create rollback action message
	action := map[string]interface{}{
		"request_id":  requestID,
		"action":      "rollback_ebpf",
		"target_id":   target.TargetID,
		"artifact_id": target.ArtifactID,
		"timestamp":   time.Now(),
	}

	actionJSON, err := json.Marshal(action)
	if err != nil {
		target.Status = "failed"
		target.Error = fmt.Sprintf("Failed to marshal rollback action: %v", err)
		rm.logger.Error("Failed to marshal rollback action",
			"request_id", requestID,
			"target_id", target.TargetID,
			"error", err)
		return
	}

	// Publish rollback action
	subject := "actions.rollback.ebpf"
	if err := rm.nc.Publish(subject, actionJSON); err != nil {
		target.Status = "failed"
		target.Error = fmt.Sprintf("Failed to publish rollback action: %v", err)
		rm.logger.Error("Failed to publish rollback action",
			"request_id", requestID,
			"target_id", target.TargetID,
			"error", err)
		return
	}

	// Wait for rollback confirmation (in a real implementation, you'd wait for agent response)
	time.Sleep(5 * time.Second)

	target.Status = "completed"
	endTime := time.Now()
	target.EndTime = &endTime

	rm.logger.Info("Target rollback completed",
		"request_id", requestID,
		"target_id", target.TargetID)
}

// updateRollbackStatus updates the rollback status
func (rm *RollbackManager) updateRollbackStatus(requestID, status string) {
	rm.rollbackMutex.Lock()
	defer rm.rollbackMutex.Unlock()

	if rollback, exists := rm.activeRollbacks[requestID]; exists {
		rollback.Status = status
		rm.notifyCallbacks(requestID, rollback)
	}
}

// setRollbackEndTime sets the end time for a rollback
func (rm *RollbackManager) setRollbackEndTime(requestID string, endTime *time.Time) {
	rm.rollbackMutex.Lock()
	defer rm.rollbackMutex.Unlock()

	if rollback, exists := rm.activeRollbacks[requestID]; exists {
		rollback.EndTime = endTime
		rm.notifyCallbacks(requestID, rollback)
	}
}

// notifyCallbacks notifies all registered callbacks
func (rm *RollbackManager) notifyCallbacks(requestID string, result *RollbackResult) {
	rm.callbackMutex.RLock()
	defer rm.callbackMutex.RUnlock()

	for _, callback := range rm.callbacks {
		go callback(requestID, result)
	}
}

// AddCallback adds a rollback callback
func (rm *RollbackManager) AddCallback(callback RollbackCallback) {
	rm.callbackMutex.Lock()
	defer rm.callbackMutex.Unlock()
	rm.callbacks = append(rm.callbacks, callback)
}

// GetRollbackStatus retrieves the status of a rollback
func (rm *RollbackManager) GetRollbackStatus(requestID string) (*RollbackResult, error) {
	rm.rollbackMutex.RLock()
	defer rm.rollbackMutex.RUnlock()

	rollback, exists := rm.activeRollbacks[requestID]
	if !exists {
		return nil, fmt.Errorf("rollback not found: %s", requestID)
	}

	return rollback, nil
}

// ListActiveRollbacks returns all active rollbacks
func (rm *RollbackManager) ListActiveRollbacks() []*RollbackResult {
	rm.rollbackMutex.RLock()
	defer rm.rollbackMutex.RUnlock()

	rollbacks := make([]*RollbackResult, 0, len(rm.activeRollbacks))
	for _, rollback := range rm.activeRollbacks {
		rollbacks = append(rollbacks, rollback)
	}

	return rollbacks
}

// CleanupCompletedRollbacks removes completed rollbacks older than the specified duration
func (rm *RollbackManager) CleanupCompletedRollbacks(olderThan time.Duration) {
	rm.rollbackMutex.Lock()
	defer rm.rollbackMutex.Unlock()

	cutoffTime := time.Now().Add(-olderThan)

	for requestID, rollback := range rm.activeRollbacks {
		if rollback.Status == "completed" && rollback.EndTime != nil && rollback.EndTime.Before(cutoffTime) {
			delete(rm.activeRollbacks, requestID)
			rm.logger.Debug("Cleaned up completed rollback", "request_id", requestID)
		}
	}
}
