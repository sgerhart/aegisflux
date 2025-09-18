package decision

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"aegisflux/backend/orchestrator/internal/rollout"
)

// RollbackHandler manages rollback operations for decision integration
type RollbackHandler struct {
	logger       *slog.Logger
	rolloutMgr   *rollout.BPFRolloutManager
	activeRollbacks map[string]*RollbackOperation
	rollbackMutex   sync.RWMutex
	callbacks        []RollbackCallback
	callbackMutex    sync.RWMutex
}

// RollbackOperation represents an active rollback operation
type RollbackOperation struct {
	OperationID     string            `json:"operation_id"`
	PlanID          string            `json:"plan_id"`
	ControlID       string            `json:"control_id"`
	DeploymentID    string            `json:"deployment_id"`
	Status          string            `json:"status"` // "initiated", "in_progress", "completed", "failed"
	Strategy        string            `json:"strategy"`
	Targets         []string          `json:"targets"`
	Reason          string            `json:"reason"`
	InitiatedAt     time.Time         `json:"initiated_at"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
	Error           string            `json:"error,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// RollbackCallback is a function called when rollback events occur
type RollbackCallback func(operationID string, operation *RollbackOperation)

// NewRollbackHandler creates a new rollback handler
func NewRollbackHandler(logger *slog.Logger, rolloutMgr *rollout.BPFRolloutManager) *RollbackHandler {
	return &RollbackHandler{
		logger:           logger,
		rolloutMgr:       rolloutMgr,
		activeRollbacks:  make(map[string]*RollbackOperation),
		callbacks:        make([]RollbackCallback, 0),
	}
}

// InitiateRollback initiates a rollback operation for a plan or control
func (rh *RollbackHandler) InitiateRollback(ctx context.Context, req RollbackRequest) (*RollbackOperation, error) {
	rh.logger.Info("Initiating rollback",
		"plan_id", req.PlanID,
		"control_ids", req.ControlIDs,
		"reason", req.Reason)

	operationID := fmt.Sprintf("rollback-%s-%d", req.PlanID, time.Now().Unix())

	// Create rollback operation
	operation := &RollbackOperation{
		OperationID:  operationID,
		PlanID:       req.PlanID,
		Status:       "initiated",
		Strategy:     string(req.Strategy),
		Reason:       req.Reason,
		InitiatedAt:  time.Now(),
		Metadata:     make(map[string]interface{}),
	}

	// Store operation
	rh.rollbackMutex.Lock()
	rh.activeRollbacks[operationID] = operation
	rh.rollbackMutex.Unlock()

	// Execute rollback
	go rh.executeRollback(ctx, operation, req)

	return operation, nil
}

// executeRollback executes the rollback operation
func (rh *RollbackHandler) executeRollback(ctx context.Context, operation *RollbackOperation, req RollbackRequest) {
	defer func() {
		completedAt := time.Now()
		operation.CompletedAt = &completedAt
		rh.notifyCallbacks(operation.OperationID, operation)
	}()

	rh.updateOperationStatus(operation.OperationID, "in_progress")

	// Get active deployments for the plan
	deployments, err := rh.getActiveDeploymentsForPlan(req.PlanID)
	if err != nil {
		operation.Status = "failed"
		operation.Error = fmt.Sprintf("Failed to get deployments: %v", err)
		rh.logger.Error("Failed to get deployments", "error", err)
		return
	}

	if len(deployments) == 0 {
		operation.Status = "completed"
		operation.Error = "No active deployments found"
		rh.logger.Info("No active deployments to rollback", "plan_id", req.PlanID)
		return
	}

	// Filter deployments by control IDs if specified
	if len(req.ControlIDs) > 0 {
		deployments = rh.filterDeploymentsByControls(deployments, req.ControlIDs)
	}

	// Execute rollback based on strategy
	switch req.Strategy {
	case RollbackStrategyImmediate:
		err = rh.executeImmediateRollback(ctx, operation, deployments)
	case RollbackStrategyGradual:
		err = rh.executeGradualRollback(ctx, operation, deployments)
	case RollbackStrategySelective:
		err = rh.executeSelectiveRollback(ctx, operation, deployments)
	default:
		err = fmt.Errorf("unknown rollback strategy: %v", req.Strategy)
	}

	if err != nil {
		operation.Status = "failed"
		operation.Error = err.Error()
		rh.logger.Error("Rollback execution failed", "error", err)
		return
	}

	operation.Status = "completed"
	rh.logger.Info("Rollback completed successfully", "operation_id", operation.OperationID)
}

// executeImmediateRollback executes immediate rollback of all deployments
func (rh *RollbackHandler) executeImmediateRollback(ctx context.Context, operation *RollbackOperation, deployments []*DeploymentResponse) error {
	rh.logger.Info("Executing immediate rollback", "operation_id", operation.OperationID)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for _, deployment := range deployments {
		wg.Add(1)
		go func(dep *DeploymentResponse) {
			defer wg.Done()

			err := rh.rollbackDeployment(ctx, dep)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to rollback deployment %s: %w", dep.DeploymentID, err))
				mu.Unlock()
			}
		}(deployment)
	}

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("rollback failed with %d errors: %v", len(errors), errors)
	}

	return nil
}

// executeGradualRollback executes gradual rollback of deployments
func (rh *RollbackHandler) executeGradualRollback(ctx context.Context, operation *RollbackOperation, deployments []*DeploymentResponse) error {
	rh.logger.Info("Executing gradual rollback", "operation_id", operation.OperationID)

	batchSize := 3 // Roll back 3 deployments at a time
	for i := 0; i < len(deployments); i += batchSize {
		end := i + batchSize
		if end > len(deployments) {
			end = len(deployments)
		}

		batch := deployments[i:end]
		rh.logger.Info("Rolling back batch",
			"operation_id", operation.OperationID,
			"batch_start", i,
			"batch_end", end)

		var wg sync.WaitGroup
		var mu sync.Mutex
		var errors []error

		for _, deployment := range batch {
			wg.Add(1)
			go func(dep *DeploymentResponse) {
				defer wg.Done()

				err := rh.rollbackDeployment(ctx, dep)
				if err != nil {
					mu.Lock()
					errors = append(errors, fmt.Errorf("failed to rollback deployment %s: %w", dep.DeploymentID, err))
					mu.Unlock()
				}
			}(deployment)
		}

		wg.Wait()

		if len(errors) > 0 {
			return fmt.Errorf("batch rollback failed with %d errors: %v", len(errors), errors)
		}

		// Wait between batches (except for the last batch)
		if end < len(deployments) {
			time.Sleep(30 * time.Second)
		}
	}

	return nil
}

// executeSelectiveRollback executes selective rollback of failed deployments
func (rh *RollbackHandler) executeSelectiveRollback(ctx context.Context, operation *RollbackOperation, deployments []*DeploymentResponse) error {
	rh.logger.Info("Executing selective rollback", "operation_id", operation.OperationID)

	// Filter to only failed deployments
	failedDeployments := make([]*DeploymentResponse, 0)
	for _, deployment := range deployments {
		if deployment.Status == "failed" || deployment.Status == "rollback" {
			failedDeployments = append(failedDeployments, deployment)
		}
	}

	if len(failedDeployments) == 0 {
		rh.logger.Info("No failed deployments to rollback", "operation_id", operation.OperationID)
		return nil
	}

	rh.logger.Info("Selective rollback targets identified",
		"operation_id", operation.OperationID,
		"failed_count", len(failedDeployments))

	// Roll back failed deployments
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for _, deployment := range failedDeployments {
		wg.Add(1)
		go func(dep *DeploymentResponse) {
			defer wg.Done()

			err := rh.rollbackDeployment(ctx, dep)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to rollback deployment %s: %w", dep.DeploymentID, err))
				mu.Unlock()
			}
		}(deployment)
	}

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("selective rollback failed with %d errors: %v", len(errors), errors)
	}

	return nil
}

// rollbackDeployment rolls back a specific deployment
func (rh *RollbackHandler) rollbackDeployment(ctx context.Context, deployment *DeploymentResponse) error {
	rh.logger.Info("Rolling back deployment",
		"deployment_id", deployment.DeploymentID,
		"plan_id", deployment.PlanID,
		"targets", deployment.Targets)

	// Create rollback request for the rollout manager
	rollbackReq := rollout.RollbackRequest{
		RequestID: fmt.Sprintf("rollback-%s-%d", deployment.DeploymentID, time.Now().Unix()),
		Strategy:  rollout.RollbackStrategyImmediate,
		Targets:   deployment.Targets,
		Reason:    fmt.Sprintf("Rollback for deployment %s", deployment.DeploymentID),
	}

	// In a real implementation, you'd call the rollback manager
	// For now, we'll simulate the rollback
	rh.logger.Info("Simulating rollback for deployment", "deployment_id", deployment.DeploymentID)
	
	// Simulate rollback time
	time.Sleep(2 * time.Second)

	rh.logger.Info("Deployment rollback completed", "deployment_id", deployment.DeploymentID)
	return nil
}

// getActiveDeploymentsForPlan gets active deployments for a plan
func (rh *RollbackHandler) getActiveDeploymentsForPlan(planID string) ([]*DeploymentResponse, error) {
	// In a real implementation, you'd get this from the rollout integration
	// For now, return placeholder data
	deployments := []*DeploymentResponse{
		{
			PlanID:        planID,
			DeploymentID:  "deploy-1",
			Status:        "active",
			Targets:       []string{"host-1", "host-2"},
			DeployedAt:    time.Now().Add(-1 * time.Hour),
		},
		{
			PlanID:        planID,
			DeploymentID:  "deploy-2",
			Status:        "failed",
			Targets:       []string{"host-3"},
			DeployedAt:    time.Now().Add(-30 * time.Minute),
		},
	}

	return deployments, nil
}

// filterDeploymentsByControls filters deployments by control IDs
func (rh *RollbackHandler) filterDeploymentsByControls(deployments []*DeploymentResponse, controlIDs []string) []*DeploymentResponse {
	filtered := make([]*DeploymentResponse, 0)
	
	for _, deployment := range deployments {
		// In a real implementation, you'd check if the deployment is for one of the specified controls
		// For now, we'll include all deployments
		filtered = append(filtered, deployment)
	}

	return filtered
}

// updateOperationStatus updates the status of a rollback operation
func (rh *RollbackHandler) updateOperationStatus(operationID, status string) {
	rh.rollbackMutex.Lock()
	defer rh.rollbackMutex.Unlock()

	if operation, exists := rh.activeRollbacks[operationID]; exists {
		operation.Status = status
		rh.notifyCallbacks(operationID, operation)
	}
}

// notifyCallbacks notifies all registered callbacks
func (rh *RollbackHandler) notifyCallbacks(operationID string, operation *RollbackOperation) {
	rh.callbackMutex.RLock()
	defer rh.callbackMutex.RUnlock()

	for _, callback := range rh.callbacks {
		go callback(operationID, operation)
	}
}

// AddCallback adds a rollback callback
func (rh *RollbackHandler) AddCallback(callback RollbackCallback) {
	rh.callbackMutex.Lock()
	defer rh.callbackMutex.Unlock()
	rh.callbacks = append(rh.callbacks, callback)
}

// GetRollbackOperation retrieves a rollback operation
func (rh *RollbackHandler) GetRollbackOperation(operationID string) (*RollbackOperation, error) {
	rh.rollbackMutex.RLock()
	defer rh.rollbackMutex.RUnlock()

	operation, exists := rh.activeRollbacks[operationID]
	if !exists {
		return nil, fmt.Errorf("rollback operation not found: %s", operationID)
	}

	return operation, nil
}

// ListRollbackOperations returns all rollback operations
func (rh *RollbackHandler) ListRollbackOperations() []*RollbackOperation {
	rh.rollbackMutex.RLock()
	defer rh.rollbackMutex.RUnlock()

	operations := make([]*RollbackOperation, 0, len(rh.activeRollbacks))
	for _, operation := range rh.activeRollbacks {
		operations = append(operations, operation)
	}

	return operations
}

// CleanupCompletedOperations removes completed rollback operations older than the specified duration
func (rh *RollbackHandler) CleanupCompletedOperations(olderThan time.Duration) {
	rh.rollbackMutex.Lock()
	defer rh.rollbackMutex.Unlock()

	cutoffTime := time.Now().Add(-olderThan)

	for operationID, operation := range rh.activeRollbacks {
		if operation.Status == "completed" && operation.CompletedAt != nil && operation.CompletedAt.Before(cutoffTime) {
			delete(rh.activeRollbacks, operationID)
			rh.logger.Debug("Cleaned up completed rollback operation", "operation_id", operationID)
		}
	}
}
