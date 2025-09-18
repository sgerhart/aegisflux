package decision

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"aegisflux/backend/orchestrator/internal/ebpf"
	"aegisflux/backend/orchestrator/internal/rollout"
)

// ExampleDecisionIntegration demonstrates the complete decision integration system
func ExampleDecisionIntegration() error {
	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Connect to NATS
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()

	// Create eBPF orchestrator
	templatesDir := "/path/to/bpf-templates"
	outputDir := "/tmp/bpf-orchestrator"
	registryURL := "http://localhost:8084"
	authToken := "your-auth-token"
	orchestrator := ebpf.NewOrchestrator(logger, templatesDir, outputDir, registryURL, authToken)

	// Create rollout manager
	rolloutMgr := rollout.NewBPFRolloutManager(logger, nc, registryURL)

	// Create decision processor
	decisionURL := "http://localhost:8080"
	processor := NewDecisionProcessor(logger, nc, orchestrator, rolloutMgr, decisionURL)

	// Create rollout integration
	rolloutIntegration := NewDecisionRolloutIntegration(logger, nc, rolloutMgr, decisionURL)

	// Create rollback handler
	rollbackHandler := NewRollbackHandler(logger, rolloutMgr)

	// Create API server
	apiServer := NewDecisionAPIServer(logger, processor, rolloutIntegration)

	ctx := context.Background()

	// Start integration services
	if err := rolloutIntegration.StartIntegration(ctx); err != nil {
		return fmt.Errorf("failed to start rollout integration: %w", err)
	}

	logger.Info("Decision integration system started successfully",
		"api_server", apiServer != nil,
		"processor", processor != nil,
		"rollout_integration", rolloutIntegration != nil,
		"rollback_handler", rollbackHandler != nil)

	return nil
}

// ExamplePlanProcessing demonstrates plan processing workflow
func ExamplePlanProcessing() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()

	// Create components
	registryURL := "http://localhost:8084"
	orchestrator := ebpf.NewOrchestrator(logger, "/path/to/templates", "/tmp/output", registryURL, "token")
	rolloutMgr := rollout.NewBPFRolloutManager(logger, nc, registryURL)
	processor := NewDecisionProcessor(logger, nc, orchestrator, rolloutMgr, "http://localhost:8080")

	ctx := context.Background()

	// Example plan with eBPF controls
	plan := &Plan{
		ID:     "plan-123",
		Name:   "Security Policy Plan",
		Status: "active",
		Controls: []Control{
			{
				ID:     "control-1",
				Type:   "ebpf_drop_egress_by_cgroup",
				Target: "host-1",
				Parameters: map[string]interface{}{
					"dst_ip":    "8.8.8.8",
					"dst_port":  "53",
					"cgroup_id": "12345",
					"ttl":       "3600",
				},
				Status: "pending",
				Metadata: map[string]interface{}{
					"target_arch":   "x86_64",
					"kernel_version": "5.15.0",
				},
			},
			{
				ID:     "control-2",
				Type:   "ebpf_deny_syscall_for_cgroup",
				Target: "host-2",
				Parameters: map[string]interface{}{
					"cgroup_id":   "67890",
					"syscall":     "execve",
					"ttl":         "1800",
				},
				Status: "pending",
				Metadata: map[string]interface{}{
					"target_arch":   "x86_64",
					"kernel_version": "5.15.0",
				},
			},
		},
	}

	// Step 1: Process plan (render, pack, upload artifacts)
	logger.Info("Processing plan", "plan_id", plan.ID)
	
	response, err := processor.ProcessPlan(ctx, plan)
	if err != nil {
		return fmt.Errorf("failed to process plan: %w", err)
	}

	logger.Info("Plan processing completed",
		"plan_id", response.PlanID,
		"status", response.Status,
		"artifact_ids", response.ArtifactIDs)

	// Step 2: Deploy plan with canary strategy
	logger.Info("Deploying plan with canary strategy")
	
	deploymentReq := &DeploymentRequest{
		PlanID:          plan.ID,
		DeploymentType:  "canary",
		Targets:         []string{"host-1", "host-2"},
		TTL:             3600,
		Reason:          "Initial deployment",
	}

	deploymentResp, err := processor.DeployPlan(ctx, deploymentReq)
	if err != nil {
		return fmt.Errorf("failed to deploy plan: %w", err)
	}

	logger.Info("Plan deployment initiated",
		"deployment_id", deploymentResp.DeploymentID,
		"status", deploymentResp.Status,
		"deployed_controls", deploymentResp.DeployedControls)

	// Step 3: Monitor deployment status
	logger.Info("Monitoring deployment status")
	
	for i := 0; i < 10; i++ {
		status, err := processor.GetPlanStatus(ctx, plan.ID)
		if err != nil {
			logger.Error("Failed to get plan status", "error", err)
			continue
		}

		logger.Info("Plan status",
			"plan_id", status.PlanID,
			"status", status.Status,
			"active_controls", status.ActiveControls,
			"failed_controls", status.FailedControls)

		if status.Status == "completed" || status.Status == "failed" {
			break
		}

		time.Sleep(10 * time.Second)
	}

	return nil
}

// ExampleRollbackWorkflow demonstrates rollback workflow
func ExampleRollbackWorkflow() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()

	// Create components
	registryURL := "http://localhost:8084"
	rolloutMgr := rollout.NewBPFRolloutManager(logger, nc, registryURL)
	rollbackHandler := NewRollbackHandler(logger, rolloutMgr)

	ctx := context.Background()

	// Example: Immediate rollback
	logger.Info("Initiating immediate rollback")
	
	rollbackReq := RollbackRequest{
		PlanID:     "plan-123",
		Strategy:   RollbackStrategyImmediate,
		ControlIDs: []string{"control-1", "control-2"},
		Reason:     "High violation rate detected",
		Timeout:    5 * time.Minute,
	}

	operation, err := rollbackHandler.InitiateRollback(ctx, rollbackReq)
	if err != nil {
		return fmt.Errorf("failed to initiate rollback: %w", err)
	}

	logger.Info("Rollback operation initiated",
		"operation_id", operation.OperationID,
		"strategy", operation.Strategy,
		"reason", operation.Reason)

	// Monitor rollback status
	for i := 0; i < 10; i++ {
		status, err := rollbackHandler.GetRollbackOperation(operation.OperationID)
		if err != nil {
			logger.Error("Failed to get rollback status", "error", err)
			continue
		}

		logger.Info("Rollback status",
			"operation_id", status.OperationID,
			"status", status.Status,
			"reason", status.Reason)

		if status.Status == "completed" || status.Status == "failed" {
			break
		}

		time.Sleep(5 * time.Second)
	}

	// Example: Gradual rollback
	logger.Info("Initiating gradual rollback")
	
	gradualReq := RollbackRequest{
		PlanID:     "plan-456",
		Strategy:   RollbackStrategyGradual,
		Reason:     "Performance degradation detected",
		Timeout:    10 * time.Minute,
	}

	operation2, err := rollbackHandler.InitiateRollback(ctx, gradualReq)
	if err != nil {
		return fmt.Errorf("failed to initiate gradual rollback: %w", err)
	}

	logger.Info("Gradual rollback operation initiated",
		"operation_id", operation2.OperationID,
		"strategy", operation2.Strategy)

	return nil
}

// ExampleAPIIntegration demonstrates API integration
func ExampleAPIIntegration() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create API server components
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()

	registryURL := "http://localhost:8084"
	orchestrator := ebpf.NewOrchestrator(logger, "/path/to/templates", "/tmp/output", registryURL, "token")
	rolloutMgr := rollout.NewBPFRolloutManager(logger, nc, registryURL)
	processor := NewDecisionProcessor(logger, nc, orchestrator, rolloutMgr, "http://localhost:8080")
	rolloutIntegration := NewDecisionRolloutIntegration(logger, nc, rolloutMgr, "http://localhost:8080")

	apiServer := NewDecisionAPIServer(logger, processor, rolloutIntegration)

	logger.Info("Decision API server created successfully",
		"api_server", apiServer != nil)

	// Example API endpoints available:
	// POST /plans - Create a new plan
	// GET /plans/{plan_id} - Get plan details
	// PUT /plans/{plan_id} - Update plan
	// GET /plans/{plan_id}/status - Get plan status
	// POST /plans/{plan_id}/process - Process plan (render, pack, upload)
	// POST /plans/{plan_id}/controls - Add control to plan
	// GET /plans/{plan_id}/controls/{control_id} - Get control details
	// PUT /plans/{plan_id}/controls/{control_id} - Update control
	// DELETE /plans/{plan_id}/controls/{control_id} - Remove control
	// POST /plans/{plan_id}/deploy - Deploy plan
	// GET /plans/{plan_id}/deployments - List plan deployments
	// GET /deployments/{deployment_id} - Get deployment status
	// POST /deployments/{deployment_id}/rollback - Rollback deployment
	// GET /rollouts - List active rollouts
	// GET /rollouts/{request_id} - Get rollout status
	// POST /rollouts/{request_id}/rollback - Rollback rollout
	// GET /telemetry/plans/{plan_id} - Get plan telemetry
	// GET /telemetry/controls/{control_id} - Get control telemetry

	return nil
}

// ExampleTelemetryIntegration demonstrates telemetry integration
func ExampleTelemetryIntegration() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()

	// Create telemetry monitor
	telemetryMon := rollout.NewTelemetryMonitor(logger, nc)

	// Add callbacks for automatic actions
	telemetryMon.AddCallback(func(targetID string, telemetry *rollout.TelemetryData) {
		logger.Info("Telemetry received",
			"target_id", targetID,
			"violations", telemetry.Violations,
			"errors", telemetry.Errors,
			"latency", telemetry.Latency)
	})

	// Watch for high violations and trigger rollback
	telemetryMon.WatchForViolations(10, func(targetID string, violations int) {
		logger.Warn("High violation count detected - triggering rollback",
			"target_id", targetID,
			"violations", violations)

		// In a real implementation, you'd trigger automatic rollback
		// via the decision integration system
	})

	// Watch for high errors
	telemetryMon.WatchForErrors(5, func(targetID string, errors int) {
		logger.Warn("High error count detected",
			"target_id", targetID,
			"errors", errors)
	})

	// Start monitoring
	ctx := context.Background()
	if err := telemetryMon.StartMonitoring(ctx); err != nil {
		return fmt.Errorf("failed to start telemetry monitoring: %w", err)
	}
	defer telemetryMon.StopMonitoring()

	logger.Info("Telemetry monitoring started successfully")

	// Monitor for some time
	time.Sleep(2 * time.Minute)

	return nil
}
