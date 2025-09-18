package rollout

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

// ExampleRollout demonstrates how to use the BPF rollout system
func ExampleRollout() error {
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

	// Create rollout manager
	registryURL := "http://localhost:8084"
	rolloutMgr := NewBPFRolloutManager(logger, nc, registryURL)

	// Create telemetry monitor
	telemetryMon := NewTelemetryMonitor(logger, nc)

	// Start telemetry monitoring
	ctx := context.Background()
	if err := telemetryMon.StartMonitoring(ctx); err != nil {
		return fmt.Errorf("failed to start telemetry monitoring: %w", err)
	}
	defer telemetryMon.StopMonitoring()

	// Create rollback manager
	rollbackMgr := NewRollbackManager(logger, nc)

	// Create API server
	apiServer := NewAPIServer(logger, rolloutMgr, telemetryMon)
	
	// Use the variables to avoid unused variable warnings
	_ = rollbackMgr
	_ = apiServer

	// Example: Apply canary deployment
	request := ApplyRequest{
		PlanID:     "plan-123",
		Targets:    []string{"host-1", "host-2", "host-3", "host-4", "host-5"},
		ArtifactID: "artifact-456",
		TTL:        3600,
		Canary:     true,
	}

	response, err := rolloutMgr.ApplyCanary(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to apply canary deployment: %w", err)
	}

	logger.Info("Canary deployment initiated",
		"request_id", response.RequestID,
		"plan_id", response.PlanID,
		"targets_count", len(response.Targets))

	// Monitor rollout status
	for {
		status, err := rolloutMgr.GetRolloutStatus(response.RequestID)
		if err != nil {
			return fmt.Errorf("failed to get rollout status: %w", err)
		}

		logger.Info("Rollout status",
			"request_id", status.RequestID,
			"status", status.Status,
			"applied_at", status.AppliedAt)

		if status.Status == "success" || status.Status == "failed" || status.Status == "rollback" {
			break
		}

		time.Sleep(10 * time.Second)
	}

	return nil
}

// ExampleTelemetryMonitoring demonstrates telemetry monitoring
func ExampleTelemetryMonitoring() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()

	telemetryMon := NewTelemetryMonitor(logger, nc)

	// Add callbacks for violations and errors
	telemetryMon.AddCallback(func(targetID string, telemetry *TelemetryData) {
		logger.Info("Telemetry received",
			"target_id", targetID,
			"violations", telemetry.Violations,
			"errors", telemetry.Errors,
			"latency", telemetry.Latency)
	})

	// Watch for violations
	telemetryMon.WatchForViolations(5, func(targetID string, violations int) {
		logger.Warn("High violation count detected",
			"target_id", targetID,
			"violations", violations)
	})

	// Watch for errors
	telemetryMon.WatchForErrors(3, func(targetID string, errors int) {
		logger.Warn("High error count detected",
			"target_id", targetID,
			"errors", errors)
	})

	// Watch for high latency
	telemetryMon.WatchForLatency(500.0, func(targetID string, latency float64) {
		logger.Warn("High latency detected",
			"target_id", targetID,
			"latency", latency)
	})

	// Start monitoring
	ctx := context.Background()
	if err := telemetryMon.StartMonitoring(ctx); err != nil {
		return fmt.Errorf("failed to start telemetry monitoring: %w", err)
	}
	defer telemetryMon.StopMonitoring()

	// Monitor for some time
	time.Sleep(2 * time.Minute)

	// Get telemetry data
	allData := telemetryMon.GetAllTelemetryData()
	logger.Info("Telemetry summary", "targets_count", len(allData))

	// Get aggregated data for a specific target
	for targetID := range allData {
		aggregation, err := telemetryMon.GetTelemetryAggregation(targetID, 1*time.Hour)
		if err != nil {
			logger.Error("Failed to get aggregation", "target_id", targetID, "error", err)
			continue
		}

		logger.Info("Telemetry aggregation",
			"target_id", targetID,
			"total_violations", aggregation.TotalViolations,
			"total_errors", aggregation.TotalErrors,
			"average_latency", aggregation.AverageLatency)
	}

	return nil
}

// ExampleRollback demonstrates rollback operations
func ExampleRollback() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()

	rollbackMgr := NewRollbackManager(logger, nc)

	// Add rollback callback
	rollbackMgr.AddCallback(func(requestID string, result *RollbackResult) {
		logger.Info("Rollback status update",
			"request_id", requestID,
			"status", result.Status,
			"strategy", result.Strategy)
	})

	// Example: Immediate rollback
	immediateReq := RollbackRequest{
		RequestID: "rollback-immediate-123",
		Strategy:  RollbackStrategyImmediate,
		Targets:   []string{"host-1", "host-2", "host-3"},
		Reason:    "High violation rate detected",
		Timeout:   5 * time.Minute,
	}

	result, err := rollbackMgr.InitiateRollback(context.Background(), immediateReq)
	if err != nil {
		return fmt.Errorf("failed to initiate immediate rollback: %w", err)
	}

	logger.Info("Immediate rollback initiated", "request_id", result.RequestID)

	// Example: Gradual rollback
	gradualReq := RollbackRequest{
		RequestID: "rollback-gradual-456",
		Strategy:  RollbackStrategyGradual,
		Targets:   []string{"host-1", "host-2", "host-3", "host-4", "host-5", "host-6"},
		Reason:    "Performance degradation detected",
		Timeout:   10 * time.Minute,
	}

	result, err = rollbackMgr.InitiateRollback(context.Background(), gradualReq)
	if err != nil {
		return fmt.Errorf("failed to initiate gradual rollback: %w", err)
	}

	logger.Info("Gradual rollback initiated", "request_id", result.RequestID)

	// Monitor rollback status
	for {
		status, err := rollbackMgr.GetRollbackStatus(result.RequestID)
		if err != nil {
			return fmt.Errorf("failed to get rollback status: %w", err)
		}

		logger.Info("Rollback status",
			"request_id", status.RequestID,
			"status", status.Status,
			"strategy", status.Strategy)

		if status.Status == "completed" || status.Status == "failed" {
			break
		}

		time.Sleep(5 * time.Second)
	}

	return nil
}

// ExampleAPIUsage demonstrates API usage
func ExampleAPIUsage() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()

	// Create managers
	registryURL := "http://localhost:8084"
	rolloutMgr := NewBPFRolloutManager(logger, nc, registryURL)
	telemetryMon := NewTelemetryMonitor(logger, nc)
	apiServer := NewAPIServer(logger, rolloutMgr, telemetryMon)

	// Start HTTP server (in a real implementation)
	logger.Info("API server would be started here", "server", apiServer)

	// Example API endpoints available:
	// POST /apply/ebpf - Apply BPF program
	// GET /apply/ebpf/{request_id} - Get rollout status
	// GET /apply/ebpf - List all rollouts
	// POST /apply/ebpf/{request_id}/rollback - Rollback rollout
	// GET /telemetry - Get telemetry data
	// GET /telemetry/{target_id} - Get target telemetry
	// GET /telemetry/aggregate/{target_id} - Get aggregated telemetry
	// GET /config/thresholds - Get violation thresholds
	// POST /config/thresholds - Set violation thresholds
	// POST /config/observation-window - Set observation window
	// GET /metrics - Get metrics

	return nil
}

// ExampleIntegration demonstrates full integration
func ExampleIntegration() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Connect to NATS
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()

	// Create all managers
	registryURL := "http://localhost:8084"
	rolloutMgr := NewBPFRolloutManager(logger, nc, registryURL)
	telemetryMon := NewTelemetryMonitor(logger, nc)
	rollbackMgr := NewRollbackManager(logger, nc)

	// Start telemetry monitoring
	ctx := context.Background()
	if err := telemetryMon.StartMonitoring(ctx); err != nil {
		return fmt.Errorf("failed to start telemetry monitoring: %w", err)
	}
	defer telemetryMon.StopMonitoring()

	// Set up telemetry callbacks for automatic rollback
	telemetryMon.WatchForViolations(10, func(targetID string, violations int) {
		logger.Warn("Automatic rollback triggered by violations",
			"target_id", targetID,
			"violations", violations)

		// Find active rollout for this target and initiate rollback
		rollouts := rolloutMgr.ListActiveRollouts()
		for _, rollout := range rollouts {
			for _, target := range rollout.Targets {
				if target.TargetID == targetID {
					rollbackReq := RollbackRequest{
						RequestID: fmt.Sprintf("auto-rollback-%d", time.Now().Unix()),
						Strategy:  RollbackStrategyImmediate,
						Targets:   []string{targetID},
						Reason:    fmt.Sprintf("Automatic rollback due to %d violations", violations),
					}

					if _, err := rollbackMgr.InitiateRollback(ctx, rollbackReq); err != nil {
						logger.Error("Failed to initiate automatic rollback", "error", err)
					}
					return
				}
			}
		}
	})

	// Create API server
	apiServer := NewAPIServer(logger, rolloutMgr, telemetryMon)

	logger.Info("BPF rollout system fully integrated and ready",
		"api_server", apiServer != nil,
		"rollout_manager", rolloutMgr != nil,
		"telemetry_monitor", telemetryMon != nil,
		"rollback_manager", rollbackMgr != nil)

	return nil
}
