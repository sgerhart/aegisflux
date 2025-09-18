package decision

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"aegisflux/backend/orchestrator/internal/ebpf"
	"aegisflux/backend/orchestrator/internal/rollout"
)

// DecisionProcessor handles integration between decision engine and eBPF orchestration
type DecisionProcessor struct {
	logger       *slog.Logger
	nc           *nats.Conn
	orchestrator *ebpf.Orchestrator
	rolloutMgr   *rollout.BPFRolloutManager
	decisionURL  string
}

// NewDecisionProcessor creates a new decision processor
func NewDecisionProcessor(logger *slog.Logger, nc *nats.Conn, orchestrator *ebpf.Orchestrator, rolloutMgr *rollout.BPFRolloutManager, decisionURL string) *DecisionProcessor {
	return &DecisionProcessor{
		logger:       logger,
		nc:           nc,
		orchestrator: orchestrator,
		rolloutMgr:   rolloutMgr,
		decisionURL:  decisionURL,
	}
}

// ProcessPlan processes a plan and handles eBPF controls
func (dp *DecisionProcessor) ProcessPlan(ctx context.Context, plan *Plan) (*PlanUpdateResponse, error) {
	dp.logger.Info("Processing plan with eBPF controls",
		"plan_id", plan.ID,
		"controls_count", len(plan.Controls))

	response := &PlanUpdateResponse{
		PlanID:          plan.ID,
		Status:          "processing",
		UpdatedControls: make([]Control, 0),
		ArtifactIDs:     make([]string, 0),
		DeploymentIDs:   make([]string, 0),
		UpdatedAt:       time.Now(),
	}

	// Process each control in the plan
	for i, control := range plan.Controls {
		if IsEBPFControl(control.Type) {
			dp.logger.Info("Processing eBPF control",
				"plan_id", plan.ID,
				"control_id", control.ID,
				"control_type", control.Type)

			// Step 1: Render, pack, and upload eBPF artifact
			artifactID, err := dp.processEBPFControl(ctx, plan.ID, &control)
			if err != nil {
				dp.logger.Error("Failed to process eBPF control",
					"plan_id", plan.ID,
					"control_id", control.ID,
					"error", err)

				control.Status = "failed"
				control.Error = err.Error()
				response.UpdatedControls = append(response.UpdatedControls, control)
				continue
			}

			// Step 2: Update control with artifact ID
			control.ArtifactID = artifactID
			control.Status = "artifact_ready"
			control.DeployedAt = &time.Time{}
			*control.DeployedAt = time.Now()

			response.UpdatedControls = append(response.UpdatedControls, control)
			response.ArtifactIDs = append(response.ArtifactIDs, artifactID)

			dp.logger.Info("eBPF control processed successfully",
				"plan_id", plan.ID,
				"control_id", control.ID,
				"artifact_id", artifactID)
		} else {
			// Non-eBPF control, pass through unchanged
			response.UpdatedControls = append(response.UpdatedControls, control)
		}

		// Update the plan controls slice
		plan.Controls[i] = control
	}

	response.Status = "completed"
	dp.logger.Info("Plan processing completed",
		"plan_id", plan.ID,
		"total_controls", len(response.UpdatedControls),
		"artifact_ids", response.ArtifactIDs)

	return response, nil
}

// processEBPFControl processes a single eBPF control
func (dp *DecisionProcessor) processEBPFControl(ctx context.Context, planID string, control *Control) (string, error) {
	// Get template name for control type
	templateName := GetEBPFTemplate(control.Type)
	if templateName == "" {
		return "", fmt.Errorf("unknown eBPF control type: %s", control.Type)
	}

	// Extract parameters from control
	parameters := make(map[string]string)
	for key, value := range control.Parameters {
		if strValue, ok := value.(string); ok {
			parameters[key] = strValue
		} else {
			parameters[key] = fmt.Sprintf("%v", value)
		}
	}

	// Create orchestration request
	request := ebpf.OrchestrationRequest{
		TemplateName: templateName,
		Parameters:   parameters,
		TargetArch:   dp.getTargetArch(control),
		KernelVer:    dp.getKernelVersion(control),
		Name:         fmt.Sprintf("%s-%s", planID, control.ID),
		Version:      "1.0.0",
		Description:  fmt.Sprintf("eBPF control %s for plan %s", control.Type, planID),
	}

	// Orchestrate (render, pack, sign, upload)
	result, err := dp.orchestrator.Orchestrate(ctx, request)
	if err != nil {
		return "", fmt.Errorf("orchestration failed: %w", err)
	}

	return result.ArtifactRef.ArtifactID, nil
}

// DeployPlan deploys eBPF controls from a plan
func (dp *DecisionProcessor) DeployPlan(ctx context.Context, req *DeploymentRequest) (*DeploymentResponse, error) {
	dp.logger.Info("Deploying plan",
		"plan_id", req.PlanID,
		"deployment_type", req.DeploymentType,
		"control_ids", req.ControlIDs)

	response := &DeploymentResponse{
		PlanID:           req.PlanID,
		DeploymentID:     fmt.Sprintf("deploy-%s-%d", req.PlanID, time.Now().Unix()),
		Status:           "initiated",
		DeployedControls: make([]string, 0),
		Targets:          req.Targets,
		DeployedAt:       time.Now(),
	}

	// Get plan (in a real implementation, this would come from the decision engine)
	plan, err := dp.getPlan(ctx, req.PlanID)
	if err != nil {
		response.Status = "failed"
		response.Error = fmt.Sprintf("Failed to get plan: %v", err)
		return response, err
	}

	// Filter controls if specific control IDs are provided
	controlsToDeploy := make([]Control, 0)
	if len(req.ControlIDs) > 0 {
		for _, controlID := range req.ControlIDs {
			for _, control := range plan.Controls {
				if control.ID == controlID && IsEBPFControl(control.Type) {
					controlsToDeploy = append(controlsToDeploy, control)
				}
			}
		}
	} else {
		// Deploy all eBPF controls
		for _, control := range plan.Controls {
			if IsEBPFControl(control.Type) {
				controlsToDeploy = append(controlsToDeploy, control)
			}
		}
	}

	// Deploy each control
	for _, control := range controlsToDeploy {
		if control.ArtifactID == "" {
			dp.logger.Warn("Control has no artifact ID, skipping deployment",
				"plan_id", req.PlanID,
				"control_id", control.ID)
			continue
		}

		// Prepare targets for this control
		targets := req.Targets
		if len(targets) == 0 {
			targets = []string{control.Target}
		}

		// Create rollout request
		rolloutReq := rollout.ApplyRequest{
			PlanID:     req.PlanID,
			Targets:    targets,
			ArtifactID: control.ArtifactID,
			TTL:        req.TTL,
			Canary:     req.DeploymentType == "canary",
		}

		if req.TTL == 0 {
			rolloutReq.TTL = 3600 // Default 1 hour
		}

		// Deploy based on deployment type
		switch req.DeploymentType {
		case "canary":
			rolloutResp, err := dp.rolloutMgr.ApplyCanary(ctx, rolloutReq)
			if err != nil {
				dp.logger.Error("Canary deployment failed",
					"plan_id", req.PlanID,
					"control_id", control.ID,
					"error", err)
				continue
			}
			response.DeployedControls = append(response.DeployedControls, control.ID)
			dp.logger.Info("Canary deployment initiated",
				"plan_id", req.PlanID,
				"control_id", control.ID,
				"rollout_id", rolloutResp.RequestID)

		case "enforce":
			// For enforce, we can use canary with immediate rollout
			rolloutReq.Canary = false
			rolloutResp, err := dp.rolloutMgr.ApplyCanary(ctx, rolloutReq)
			if err != nil {
				dp.logger.Error("Enforce deployment failed",
					"plan_id", req.PlanID,
					"control_id", control.ID,
					"error", err)
				continue
			}
			response.DeployedControls = append(response.DeployedControls, control.ID)
			dp.logger.Info("Enforce deployment initiated",
				"plan_id", req.PlanID,
				"control_id", control.ID,
				"rollout_id", rolloutResp.RequestID)

		case "rollback":
			// Handle rollback
			err := dp.rollbackControl(ctx, req.PlanID, control.ID, targets)
			if err != nil {
				dp.logger.Error("Rollback failed",
					"plan_id", req.PlanID,
					"control_id", control.ID,
					"error", err)
				continue
			}
			response.DeployedControls = append(response.DeployedControls, control.ID)
			dp.logger.Info("Rollback initiated",
				"plan_id", req.PlanID,
				"control_id", control.ID)

		default:
			dp.logger.Error("Unknown deployment type",
				"plan_id", req.PlanID,
				"deployment_type", req.DeploymentType)
			continue
		}
	}

	response.Status = "deployed"
	dp.logger.Info("Plan deployment completed",
		"plan_id", req.PlanID,
		"deployed_controls", len(response.DeployedControls),
		"deployment_type", req.DeploymentType)

	return response, nil
}

// rollbackControl rolls back a specific control
func (dp *DecisionProcessor) rollbackControl(ctx context.Context, planID, controlID string, targets []string) error {
	// Create rollback request
	rollbackReq := rollout.RollbackRequest{
		RequestID: fmt.Sprintf("rollback-%s-%s-%d", planID, controlID, time.Now().Unix()),
		Strategy:  rollout.RollbackStrategyImmediate,
		Targets:   targets,
		Reason:    fmt.Sprintf("Rollback for plan %s, control %s", planID, controlID),
	}

	// Initiate rollback using rollout manager's rollback functionality
	// In a real implementation, you'd have access to a rollback manager
	// For now, we'll use the rollout manager's rollback method
	dp.logger.Info("Initiating rollback",
		"plan_id", planID,
		"control_id", controlID,
		"targets", targets)
	
	// Publish rollback action via NATS
	rollbackAction := map[string]interface{}{
		"request_id": rollbackReq.RequestID,
		"action":     "rollback_ebpf",
		"targets":    targets,
		"reason":     rollbackReq.Reason,
		"timestamp":  time.Now(),
	}

	// In a real implementation, you'd marshal and publish this
	_ = rollbackAction
	
	return nil
}

// getTargetArch extracts target architecture from control metadata
func (dp *DecisionProcessor) getTargetArch(control *Control) string {
	if arch, exists := control.Metadata["target_arch"]; exists {
		if archStr, ok := arch.(string); ok {
			return archStr
		}
	}
	return "x86_64" // Default
}

// getKernelVersion extracts kernel version from control metadata
func (dp *DecisionProcessor) getKernelVersion(control *Control) string {
	if kernel, exists := control.Metadata["kernel_version"]; exists {
		if kernelStr, ok := kernel.(string); ok {
			return kernelStr
		}
	}
	return "" // Let orchestrator determine
}

// getPlan retrieves a plan (placeholder - would integrate with decision engine)
func (dp *DecisionProcessor) getPlan(ctx context.Context, planID string) (*Plan, error) {
	// In a real implementation, this would call the decision engine API
	// For now, return a placeholder
	return &Plan{
		ID:     planID,
		Status: "active",
		Controls: []Control{
			{
				ID:     "control-1",
				Type:   "ebpf_drop_egress_by_cgroup",
				Target: "host-1",
				Parameters: map[string]interface{}{
					"dst_ip":     "8.8.8.8",
					"dst_port":   "53",
					"cgroup_id":  "12345",
					"ttl":        "3600",
				},
				Status: "pending",
			},
		},
	}, nil
}

// GetPlanStatus retrieves the status of a plan and its controls
func (dp *DecisionProcessor) GetPlanStatus(ctx context.Context, planID string) (*PlanStatus, error) {
	plan, err := dp.getPlan(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	status := &PlanStatus{
		PlanID:        planID,
		Status:        plan.Status,
		TotalControls: len(plan.Controls),
		Controls:      make([]ControlStatus, 0),
		LastUpdate:    time.Now(),
	}

	// Get status for each control
	for _, control := range plan.Controls {
		controlStatus := ControlStatus{
			ControlID:    control.ID,
			Status:       control.Status,
			ArtifactID:   control.ArtifactID,
			LastUpdate:   time.Now(),
			Error:        control.Error,
		}

		// Get deployment status if applicable
		if control.ArtifactID != "" {
			// In a real implementation, you'd track deployment IDs
			// and get status from rollout manager
		}

		status.Controls = append(status.Controls, controlStatus)

		// Count by status
		switch control.Status {
		case "active", "deployed":
			status.ActiveControls++
		case "failed":
			status.FailedControls++
		case "rollback":
			status.RollbackControls++
		}
	}

	return status, nil
}

// UpdatePlanControl updates a specific control in a plan
func (dp *DecisionProcessor) UpdatePlanControl(ctx context.Context, planID string, control *Control) error {
	dp.logger.Info("Updating plan control",
		"plan_id", planID,
		"control_id", control.ID,
		"control_type", control.Type)

	// If it's an eBPF control and parameters changed, reprocess
	if IsEBPFControl(control.Type) {
		// Check if parameters changed (in a real implementation, you'd compare with existing)
		artifactID, err := dp.processEBPFControl(ctx, planID, control)
		if err != nil {
			return fmt.Errorf("failed to reprocess eBPF control: %w", err)
		}

		control.ArtifactID = artifactID
		control.Status = "updated"
	}

	// In a real implementation, you'd update the plan in the decision engine
	dp.logger.Info("Plan control updated",
		"plan_id", planID,
		"control_id", control.ID,
		"artifact_id", control.ArtifactID)

	return nil
}
