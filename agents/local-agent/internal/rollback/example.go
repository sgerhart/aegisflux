package rollback

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nats-io/nats.go"

	"aegisflux/agents/local-agent/internal/types"
)

// ExampleRollbackManager demonstrates rollback functionality
func ExampleRollbackManager() error {
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

	// Create rollback manager with thresholds
	thresholds := RollbackThresholds{
		MaxErrors:          5,
		MaxViolations:      50,
		MaxCPUPercent:      70.0,
		MaxLatencyMs:       500.0,
		MaxMemoryKB:        50 * 1024, // 50MB
		VerifierFailure:    true,
		CheckIntervalSec:   10,
		RollbackDelaySec:   2,
	}

	rollbackManager := NewRollbackManager(logger, nc, "host-001", thresholds)

	ctx := context.Background()

	// Start rollback manager
	if err := rollbackManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start rollback manager: %w", err)
	}
	defer rollbackManager.Stop()

	// Add callback to handle rollback events
	rollbackManager.AddCallback(func(event RollbackEvent, program *types.LoadedProgram) {
		logger.Info("Rollback callback triggered",
			"artifact_id", event.ArtifactID,
			"reason", event.Reason,
			"threshold", event.Threshold,
			"status", event.Status)
	})

	// Register some test programs
	programs := []*types.LoadedProgram{
		{
			ArtifactID: "artifact-1",
			Name:       "drop_egress_by_cgroup",
			Version:    "1.0.0",
			Status:     types.ProgramStatusRunning,
			LoadedAt:   time.Now(),
			TTL:        3600 * time.Second,
			Telemetry: types.ProgramTelemetry{
				ArtifactID:       "artifact-1",
				HostID:           "host-001",
				Timestamp:        time.Now().Format(time.RFC3339),
				Status:           string(types.ProgramStatusRunning),
				CPUPercent:       50.0,
				MemoryKB:         1024,
				PacketsProcessed: 1000,
				Violations:       0,
				Errors:           0,
				LatencyMs:        10.0,
			},
		},
		{
			ArtifactID: "artifact-2",
			Name:       "deny_syscall_for_cgroup",
			Version:    "1.0.0",
			Status:     types.ProgramStatusRunning,
			LoadedAt:   time.Now(),
			TTL:        3600 * time.Second,
			Telemetry: types.ProgramTelemetry{
				ArtifactID:       "artifact-2",
				HostID:           "host-001",
				Timestamp:        time.Now().Format(time.RFC3339),
				Status:           string(types.ProgramStatusRunning),
				CPUPercent:       90.0, // This should trigger rollback
				MemoryKB:         1024,
				PacketsProcessed: 1000,
				Violations:       0,
				Errors:           0,
				LatencyMs:        10.0,
			},
		},
	}

	// Register programs
	for _, program := range programs {
		rollbackManager.RegisterProgram(program)
	}

	// Simulate telemetry updates
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for i := 0; i < 10; i++ {
			select {
			case <-ticker.C:
				// Update telemetry for artifact-2 to trigger rollback
				telemetry := types.ProgramTelemetry{
					ArtifactID:       "artifact-2",
					HostID:           "host-001",
					Timestamp:        time.Now().Format(time.RFC3339),
					Status:           string(types.ProgramStatusRunning),
					CPUPercent:       95.0 + float64(i), // Increasing CPU usage
					MemoryKB:         1024,
					PacketsProcessed: 1000,
					Violations:       int64(i * 10), // Increasing violations
					Errors:           int64(i),      // Increasing errors
					LatencyMs:        100.0,
				}

				rollbackManager.UpdateTelemetry("artifact-2", telemetry)

				// Also update artifact-1 with normal telemetry
				normalTelemetry := types.ProgramTelemetry{
					ArtifactID:       "artifact-1",
					HostID:           "host-001",
					Timestamp:        time.Now().Format(time.RFC3339),
					Status:           string(types.ProgramStatusRunning),
					CPUPercent:       30.0,
					MemoryKB:         1024,
					PacketsProcessed: 1000,
					Violations:       0,
					Errors:           0,
					LatencyMs:        5.0,
				}

				rollbackManager.UpdateTelemetry("artifact-1", normalTelemetry)

			case <-ctx.Done():
				return
			}
		}
	}()

	// Simulate orchestrator rollback signal
	go func() {
		time.Sleep(5 * time.Second)
		
		// Send orchestrator rollback signal
		rollbackSignal := map[string]interface{}{
			"artifact_id": "artifact-1",
			"reason":      "manual_rollback",
			"timestamp":   time.Now().Format(time.RFC3339),
		}
		
		// Use the signal to avoid unused variable warning
		_ = rollbackSignal

		subject := "orchestrator.rollback.host-001"
		nc.Publish(subject, []byte(fmt.Sprintf(`{"artifact_id":"artifact-1","reason":"manual_rollback"}`)))
		
		logger.Info("Sent orchestrator rollback signal", "artifact_id", "artifact-1")
	}()

	// Run for a while to see rollback events
	time.Sleep(30 * time.Second)

	return nil
}

// ExampleThresholdViolations demonstrates different threshold violations
func ExampleThresholdViolations() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()

	// Create rollback manager with strict thresholds
	thresholds := RollbackThresholds{
		MaxErrors:          3,
		MaxViolations:      10,
		MaxCPUPercent:      60.0,
		MaxLatencyMs:       100.0,
		MaxMemoryKB:        10 * 1024, // 10MB
		VerifierFailure:    true,
		CheckIntervalSec:   5,
		RollbackDelaySec:   1,
	}

	rollbackManager := NewRollbackManager(logger, nc, "host-002", thresholds)

	ctx := context.Background()
	if err := rollbackManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start rollback manager: %w", err)
	}
	defer rollbackManager.Stop()

	// Test different threshold violations
	testCases := []struct {
		name     string
		artifact string
		telemetry types.ProgramTelemetry
		expected string
	}{
		{
			name:     "Error threshold violation",
			artifact: "artifact-errors",
			telemetry: types.ProgramTelemetry{
				ArtifactID:       "artifact-errors",
				HostID:           "host-002",
				Timestamp:        time.Now().Format(time.RFC3339),
				Status:           string(types.ProgramStatusRunning),
				CPUPercent:       30.0,
				MemoryKB:         1024,
				PacketsProcessed: 1000,
				Violations:       0,
				Errors:           5, // Exceeds threshold of 3
				LatencyMs:        10.0,
			},
			expected: "max_errors",
		},
		{
			name:     "Violation threshold violation",
			artifact: "artifact-violations",
			telemetry: types.ProgramTelemetry{
				ArtifactID:       "artifact-violations",
				HostID:           "host-002",
				Timestamp:        time.Now().Format(time.RFC3339),
				Status:           string(types.ProgramStatusRunning),
				CPUPercent:       30.0,
				MemoryKB:         1024,
				PacketsProcessed: 1000,
				Violations:       15, // Exceeds threshold of 10
				Errors:           0,
				LatencyMs:        10.0,
			},
			expected: "max_violations",
		},
		{
			name:     "CPU threshold violation",
			artifact: "artifact-cpu",
			telemetry: types.ProgramTelemetry{
				ArtifactID:       "artifact-cpu",
				HostID:           "host-002",
				Timestamp:        time.Now().Format(time.RFC3339),
				Status:           string(types.ProgramStatusRunning),
				CPUPercent:       80.0, // Exceeds threshold of 60.0
				MemoryKB:         1024,
				PacketsProcessed: 1000,
				Violations:       0,
				Errors:           0,
				LatencyMs:        10.0,
			},
			expected: "max_cpu_percent",
		},
		{
			name:     "Latency threshold violation",
			artifact: "artifact-latency",
			telemetry: types.ProgramTelemetry{
				ArtifactID:       "artifact-latency",
				HostID:           "host-002",
				Timestamp:        time.Now().Format(time.RFC3339),
				Status:           string(types.ProgramStatusRunning),
				CPUPercent:       30.0,
				MemoryKB:         1024,
				PacketsProcessed: 1000,
				Violations:       0,
				Errors:           0,
				LatencyMs:        200.0, // Exceeds threshold of 100.0
			},
			expected: "max_latency_ms",
		},
		{
			name:     "Memory threshold violation",
			artifact: "artifact-memory",
			telemetry: types.ProgramTelemetry{
				ArtifactID:       "artifact-memory",
				HostID:           "host-002",
				Timestamp:        time.Now().Format(time.RFC3339),
				Status:           string(types.ProgramStatusRunning),
				CPUPercent:       30.0,
				MemoryKB:         20 * 1024, // Exceeds threshold of 10MB
				PacketsProcessed: 1000,
				Violations:       0,
				Errors:           0,
				LatencyMs:        10.0,
			},
			expected: "max_memory_kb",
		},
		{
			name:     "Verifier failure",
			artifact: "artifact-verifier",
			telemetry: types.ProgramTelemetry{
				ArtifactID:       "artifact-verifier",
				HostID:           "host-002",
				Timestamp:        time.Now().Format(time.RFC3339),
				Status:           string(types.ProgramStatusRunning),
				CPUPercent:       30.0,
				MemoryKB:         1024,
				PacketsProcessed: 1000,
				Violations:       0,
				Errors:           0,
				LatencyMs:        10.0,
				VerifierMsg:      stringPtr("verification failed: invalid program"),
			},
			expected: "verifier_failure",
		},
	}

	// Add callback to track rollback events
	var rollbackEvents []RollbackEvent
	rollbackManager.AddCallback(func(event RollbackEvent, program *types.LoadedProgram) {
		rollbackEvents = append(rollbackEvents, event)
		logger.Info("Rollback event received",
			"artifact_id", event.ArtifactID,
			"reason", event.Reason,
			"threshold", event.Threshold,
			"value", event.Value)
	})

	// Test each case
	for _, testCase := range testCases {
		logger.Info("Testing threshold violation",
			"name", testCase.name,
			"artifact", testCase.artifact,
			"expected", testCase.expected)

		// Create and register program
		program := &types.LoadedProgram{
			ArtifactID: testCase.artifact,
			Name:       testCase.artifact,
			Version:    "1.0.0",
			Status:     types.ProgramStatusRunning,
			LoadedAt:   time.Now(),
			TTL:        3600 * time.Second,
			Telemetry:  testCase.telemetry,
		}

		rollbackManager.RegisterProgram(program)

		// Update telemetry to trigger threshold check
		rollbackManager.UpdateTelemetry(testCase.artifact, testCase.telemetry)

		// Wait a bit for processing
		time.Sleep(2 * time.Second)

		// Unregister program
		rollbackManager.UnregisterProgram(testCase.artifact)
	}

	// Wait for all events to be processed
	time.Sleep(5 * time.Second)

	logger.Info("Rollback test completed",
		"total_events", len(rollbackEvents))

	return nil
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}
