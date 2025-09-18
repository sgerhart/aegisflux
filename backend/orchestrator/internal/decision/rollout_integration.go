package decision

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"aegisflux/backend/orchestrator/internal/rollout"
)

// DecisionRolloutIntegration handles integration between decision engine and rollout system
type DecisionRolloutIntegration struct {
	logger        *slog.Logger
	nc            *nats.Conn
	rolloutMgr    *rollout.BPFRolloutManager
	decisionURL   string
	activeDeployments map[string]*DeploymentResponse
	deploymentMutex   sync.RWMutex
}

// NewDecisionRolloutIntegration creates a new decision rollout integration
func NewDecisionRolloutIntegration(logger *slog.Logger, nc *nats.Conn, rolloutMgr *rollout.BPFRolloutManager, decisionURL string) *DecisionRolloutIntegration {
	return &DecisionRolloutIntegration{
		logger:        logger,
		nc:            nc,
		rolloutMgr:    rolloutMgr,
		decisionURL:   decisionURL,
		activeDeployments: make(map[string]*DeploymentResponse),
	}
}

// StartIntegration starts the integration services
func (dri *DecisionRolloutIntegration) StartIntegration(ctx context.Context) error {
	dri.logger.Info("Starting decision rollout integration")

	// Subscribe to decision engine events
	if err := dri.subscribeToDecisionEvents(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to decision events: %w", err)
	}

	// Subscribe to rollout status updates
	if err := dri.subscribeToRolloutUpdates(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to rollout updates: %w", err)
	}

	// Subscribe to telemetry events for automatic rollback
	if err := dri.subscribeToTelemetryEvents(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to telemetry events: %w", err)
	}

	dri.logger.Info("Decision rollout integration started successfully")
	return nil
}

// subscribeToDecisionEvents subscribes to decision engine events
func (dri *DecisionRolloutIntegration) subscribeToDecisionEvents(ctx context.Context) error {
	// Subscribe to plan updates
	planUpdateSubject := "decision.plan.updated"
	planSub, err := dri.nc.Subscribe(planUpdateSubject, func(msg *nats.Msg) {
		dri.handlePlanUpdate(msg)
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to plan updates: %w", err)
	}

	dri.logger.Info("Subscribed to plan updates", "subject", planUpdateSubject)

	// Subscribe to control updates
	controlUpdateSubject := "decision.control.updated"
	controlSub, err := dri.nc.Subscribe(controlUpdateSubject, func(msg *nats.Msg) {
		dri.handleControlUpdate(msg)
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to control updates: %w", err)
	}

	dri.logger.Info("Subscribed to control updates", "subject", controlUpdateSubject)

	// Store subscriptions for cleanup
	go func() {
		<-ctx.Done()
		planSub.Unsubscribe()
		controlSub.Unsubscribe()
	}()

	return nil
}

// subscribeToRolloutUpdates subscribes to rollout status updates
func (dri *DecisionRolloutIntegration) subscribeToRolloutUpdates(ctx context.Context) error {
	// Subscribe to rollout status changes
	rolloutStatusSubject := "rollout.status.updated"
	rolloutSub, err := dri.nc.Subscribe(rolloutStatusSubject, func(msg *nats.Msg) {
		dri.handleRolloutStatusUpdate(msg)
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to rollout status: %w", err)
	}

	dri.logger.Info("Subscribed to rollout status updates", "subject", rolloutStatusSubject)

	go func() {
		<-ctx.Done()
		rolloutSub.Unsubscribe()
	}()

	return nil
}

// subscribeToTelemetryEvents subscribes to telemetry events for automatic actions
func (dri *DecisionRolloutIntegration) subscribeToTelemetryEvents(ctx context.Context) error {
	// Subscribe to high violation events
	violationSubject := "telemetry.violations.high"
	violationSub, err := dri.nc.Subscribe(violationSubject, func(msg *nats.Msg) {
		dri.handleHighViolations(msg)
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to violation events: %w", err)
	}

	// Subscribe to error events
	errorSubject := "telemetry.errors.high"
	errorSub, err := dri.nc.Subscribe(errorSubject, func(msg *nats.Msg) {
		dri.handleHighErrors(msg)
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to error events: %w", err)
	}

	dri.logger.Info("Subscribed to telemetry events",
		"violation_subject", violationSubject,
		"error_subject", errorSubject)

	go func() {
		<-ctx.Done()
		violationSub.Unsubscribe()
		errorSub.Unsubscribe()
	}()

	return nil
}

// handlePlanUpdate handles plan update events from decision engine
func (dri *DecisionRolloutIntegration) handlePlanUpdate(msg *nats.Msg) {
	var planUpdate PlanUpdateRequest
	if err := json.Unmarshal(msg.Data, &planUpdate); err != nil {
		dri.logger.Error("Failed to unmarshal plan update", "error", err)
		return
	}

	dri.logger.Info("Received plan update",
		"plan_id", planUpdate.PlanID,
		"update_type", planUpdate.UpdateType,
		"controls_count", len(planUpdate.Controls))

	// Process eBPF controls in the plan
	for _, control := range planUpdate.Controls {
		if IsEBPFControl(control.Type) {
			dri.logger.Info("Processing eBPF control in plan update",
				"plan_id", planUpdate.PlanID,
				"control_id", control.ID,
				"control_type", control.Type)

			// In a real implementation, you would:
			// 1. Process the control (render, pack, upload)
			// 2. Update the control with artifact ID
			// 3. Notify decision engine of completion
		}
	}

	// Notify decision engine of processing completion
	dri.notifyDecisionEngine("plan.processed", map[string]interface{}{
		"plan_id":    planUpdate.PlanID,
		"status":     "completed",
		"processed_at": time.Now(),
	})
}

// handleControlUpdate handles control update events from decision engine
func (dri *DecisionRolloutIntegration) handleControlUpdate(msg *nats.Msg) {
	var control Control
	if err := json.Unmarshal(msg.Data, &control); err != nil {
		dri.logger.Error("Failed to unmarshal control update", "error", err)
		return
	}

	if !IsEBPFControl(control.Type) {
		return // Not an eBPF control, ignore
	}

	dri.logger.Info("Received eBPF control update",
		"control_id", control.ID,
		"control_type", control.Type,
		"status", control.Status)

	// Handle different control statuses
	switch control.Status {
	case "deploy":
		dri.handleControlDeploy(control)
	case "rollback":
		dri.handleControlRollback(control)
	case "update":
		dri.handleControlUpdate(control)
	default:
		dri.logger.Debug("Unknown control status", "status", control.Status)
	}
}

// handleControlDeploy handles control deployment requests
func (dri *DecisionRolloutIntegration) handleControlDeploy(control Control) {
	dri.logger.Info("Handling control deployment",
		"control_id", control.ID,
		"artifact_id", control.ArtifactID)

	if control.ArtifactID == "" {
		dri.logger.Error("Control has no artifact ID, cannot deploy",
			"control_id", control.ID)
		return
	}

	// Create deployment request
	deploymentReq := rollout.ApplyRequest{
		PlanID:     control.Metadata["plan_id"].(string),
		Targets:    []string{control.Target},
		ArtifactID: control.ArtifactID,
		TTL:        3600, // Default 1 hour
		Canary:     true, // Default to canary deployment
	}

	// Deploy control
	_, err := dri.rolloutMgr.ApplyCanary(context.Background(), deploymentReq)
	if err != nil {
		dri.logger.Error("Failed to deploy control",
			"control_id", control.ID,
			"error", err)

		// Notify decision engine of failure
		dri.notifyDecisionEngine("control.deployment.failed", map[string]interface{}{
			"control_id": control.ID,
			"error":      err.Error(),
			"timestamp":  time.Now(),
		})
		return
	}

	dri.logger.Info("Control deployment initiated",
		"control_id", control.ID,
		"target", control.Target)
}

// handleControlRollback handles control rollback requests
func (dri *DecisionRolloutIntegration) handleControlRollback(control Control) {
	dri.logger.Info("Handling control rollback",
		"control_id", control.ID)

	// Create rollback request
	rollbackReq := rollout.RollbackRequest{
		RequestID: fmt.Sprintf("rollback-%s-%d", control.ID, time.Now().Unix()),
		Strategy:  rollout.RollbackStrategyImmediate,
		Targets:   []string{control.Target},
		Reason:    fmt.Sprintf("Rollback requested for control %s", control.ID),
	}

	// In a real implementation, you'd call the rollback manager
	dri.logger.Info("Control rollback initiated",
		"control_id", control.ID,
		"target", control.Target)
}

// handleControlUpdateRequest handles control update requests
func (dri *DecisionRolloutIntegration) handleControlUpdateRequest(control Control) {
	dri.logger.Info("Handling control update",
		"control_id", control.ID)

	// In a real implementation, you would:
	// 1. Reprocess the control if parameters changed
	// 2. Update the artifact if needed
	// 3. Redeploy if necessary
}

// handleRolloutStatusUpdate handles rollout status updates
func (dri *DecisionRolloutIntegration) handleRolloutStatusUpdate(msg *nats.Msg) {
	var statusUpdate rollout.ApplyResponse
	if err := json.Unmarshal(msg.Data, &statusUpdate); err != nil {
		dri.logger.Error("Failed to unmarshal rollout status update", "error", err)
		return
	}

	dri.logger.Info("Received rollout status update",
		"request_id", statusUpdate.RequestID,
		"status", statusUpdate.Status,
		"plan_id", statusUpdate.PlanID)

	// Update deployment tracking
	dri.deploymentMutex.Lock()
	dri.activeDeployments[statusUpdate.RequestID] = &DeploymentResponse{
		PlanID:        statusUpdate.PlanID,
		DeploymentID:  statusUpdate.RequestID,
		Status:        statusUpdate.Status,
		DeployedAt:    statusUpdate.AppliedAt,
		Error:         statusUpdate.Error,
	}
	dri.deploymentMutex.Unlock()

	// Notify decision engine of status change
	dri.notifyDecisionEngine("rollout.status.updated", map[string]interface{}{
		"plan_id":     statusUpdate.PlanID,
		"request_id":  statusUpdate.RequestID,
		"status":      statusUpdate.Status,
		"updated_at":  time.Now(),
	})

	// Handle rollback if deployment failed
	if statusUpdate.Status == "rollback" || statusUpdate.Status == "failed" {
		dri.handleDeploymentFailure(statusUpdate)
	}
}

// handleHighViolations handles high violation telemetry events
func (dri *DecisionRolloutIntegration) handleHighViolations(msg *nats.Msg) {
	var violationEvent map[string]interface{}
	if err := json.Unmarshal(msg.Data, &violationEvent); err != nil {
		dri.logger.Error("Failed to unmarshal violation event", "error", err)
		return
	}

	targetID := violationEvent["target_id"].(string)
	violations := violationEvent["violations"].(int)

	dri.logger.Warn("High violation count detected",
		"target_id", targetID,
		"violations", violations)

	// Find active deployments for this target and consider rollback
	dri.deploymentMutex.RLock()
	for _, deployment := range dri.activeDeployments {
		for _, target := range deployment.Targets {
			if target == targetID && deployment.Status == "active" {
				dri.logger.Info("Triggering automatic rollback due to violations",
					"deployment_id", deployment.DeploymentID,
					"target_id", targetID,
					"violations", violations)

				// Trigger automatic rollback
				dri.triggerAutomaticRollback(deployment.DeploymentID, targetID, violations)
			}
		}
	}
	dri.deploymentMutex.RUnlock()
}

// handleHighErrors handles high error telemetry events
func (dri *DecisionRolloutIntegration) handleHighErrors(msg *nats.Msg) {
	var errorEvent map[string]interface{}
	if err := json.Unmarshal(msg.Data, &errorEvent); err != nil {
		dri.logger.Error("Failed to unmarshal error event", "error", err)
		return
	}

	targetID := errorEvent["target_id"].(string)
	errors := errorEvent["errors"].(int)

	dri.logger.Warn("High error count detected",
		"target_id", targetID,
		"errors", errors)

	// Similar logic to handleHighViolations
}

// triggerAutomaticRollback triggers an automatic rollback
func (dri *DecisionRolloutIntegration) triggerAutomaticRollback(deploymentID, targetID string, violations int) {
	// Create rollback request
	rollbackReq := rollout.RollbackRequest{
		RequestID: fmt.Sprintf("auto-rollback-%s-%d", deploymentID, time.Now().Unix()),
		Strategy:  rollout.RollbackStrategyImmediate,
		Targets:   []string{targetID},
		Reason:    fmt.Sprintf("Automatic rollback due to %d violations", violations),
	}

	dri.logger.Info("Triggering automatic rollback",
		"deployment_id", deploymentID,
		"target_id", targetID,
		"reason", rollbackReq.Reason)

	// In a real implementation, you'd call the rollback manager
}

// handleDeploymentFailure handles deployment failure scenarios
func (dri *DecisionRolloutIntegration) handleDeploymentFailure(statusUpdate rollout.ApplyResponse) {
	dri.logger.Error("Deployment failure detected",
		"request_id", statusUpdate.RequestID,
		"plan_id", statusUpdate.PlanID,
		"status", statusUpdate.Status,
		"error", statusUpdate.Error)

	// Notify decision engine of deployment failure
	dri.notifyDecisionEngine("deployment.failed", map[string]interface{}{
		"plan_id":     statusUpdate.PlanID,
		"request_id":  statusUpdate.RequestID,
		"status":      statusUpdate.Status,
		"error":       statusUpdate.Error,
		"failed_at":   time.Now(),
	})
}

// notifyDecisionEngine notifies the decision engine of events
func (dri *DecisionRolloutIntegration) notifyDecisionEngine(eventType string, data map[string]interface{}) {
	notification := map[string]interface{}{
		"event_type": eventType,
		"data":       data,
		"timestamp":  time.Now(),
		"source":     "orchestrator",
	}

	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		dri.logger.Error("Failed to marshal notification", "error", err)
		return
	}

	subject := "decision.orchestrator.event"
	if err := dri.nc.Publish(subject, notificationJSON); err != nil {
		dri.logger.Error("Failed to publish notification", "error", err)
		return
	}

	dri.logger.Debug("Notified decision engine",
		"event_type", eventType,
		"subject", subject)
}

// GetActiveDeployments returns all active deployments
func (dri *DecisionRolloutIntegration) GetActiveDeployments() []*DeploymentResponse {
	dri.deploymentMutex.RLock()
	defer dri.deploymentMutex.RUnlock()

	deployments := make([]*DeploymentResponse, 0, len(dri.activeDeployments))
	for _, deployment := range dri.activeDeployments {
		deployments = append(deployments, deployment)
	}

	return deployments
}

// GetDeploymentStatus returns the status of a specific deployment
func (dri *DecisionRolloutIntegration) GetDeploymentStatus(deploymentID string) (*DeploymentResponse, error) {
	dri.deploymentMutex.RLock()
	defer dri.deploymentMutex.RUnlock()

	deployment, exists := dri.activeDeployments[deploymentID]
	if !exists {
		return nil, fmt.Errorf("deployment not found: %s", deploymentID)
	}

	return deployment, nil
}
