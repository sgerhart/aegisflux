package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"aegisflux/agents/local-agent/internal/types"
)

// Sender handles sending telemetry data to NATS
type Sender struct {
	logger         *slog.Logger
	nc             *nats.Conn
	subject        string
	hostID         string
	telemetryQueue chan types.TelemetryEvent
	stopChan       chan struct{}
}

// NewSender creates a new telemetry sender
func NewSender(logger *slog.Logger, nc *nats.Conn, subject, hostID string) *Sender {
	return &Sender{
		logger:         logger,
		nc:             nc,
		subject:        subject,
		hostID:         hostID,
		telemetryQueue: make(chan types.TelemetryEvent, 1000),
		stopChan:       make(chan struct{}),
	}
}

// Start starts the telemetry sender
func (s *Sender) Start(ctx context.Context) error {
	s.logger.Info("Starting telemetry sender", "subject", s.subject)

	go s.sendLoop(ctx)
	return nil
}

// Stop stops the telemetry sender
func (s *Sender) Stop() {
	s.logger.Info("Stopping telemetry sender")
	close(s.stopChan)
}

// SendTelemetry sends telemetry data for a program
func (s *Sender) SendTelemetry(program *types.LoadedProgram) error {
	telemetry := program.Telemetry
	telemetry.HostID = s.hostID
	telemetry.Timestamp = time.Now().Format(time.RFC3339)
	telemetry.Status = string(program.Status)

	event := types.TelemetryEvent{
		Type:      "program_telemetry",
		Timestamp: telemetry.Timestamp,
		Data:      telemetry,
		Metadata: map[string]string{
			"host_id":     s.hostID,
			"agent_version": "1.0.0",
		},
	}

	select {
	case s.telemetryQueue <- event:
		return nil
	default:
		return fmt.Errorf("telemetry queue is full")
	}
}

// SendProgramLoaded sends a program loaded event
func (s *Sender) SendProgramLoaded(program *types.LoadedProgram) error {
	event := types.TelemetryEvent{
		Type:      "program_loaded",
		Timestamp: time.Now().Format(time.RFC3339),
		Data: types.ProgramTelemetry{
			ArtifactID: program.ArtifactID,
			HostID:     s.hostID,
			Timestamp:  time.Now().Format(time.RFC3339),
			Status:     string(program.Status),
		},
		Metadata: map[string]string{
			"host_id":       s.hostID,
			"artifact_id":   program.ArtifactID,
			"program_name":  program.Name,
			"program_version": program.Version,
		},
	}

	select {
	case s.telemetryQueue <- event:
		return nil
	default:
		return fmt.Errorf("telemetry queue is full")
	}
}

// SendProgramUnloaded sends a program unloaded event
func (s *Sender) SendProgramUnloaded(artifactID string) error {
	event := types.TelemetryEvent{
		Type:      "program_unloaded",
		Timestamp: time.Now().Format(time.RFC3339),
		Data: types.ProgramTelemetry{
			ArtifactID: artifactID,
			HostID:     s.hostID,
			Timestamp:  time.Now().Format(time.RFC3339),
			Status:     string(types.ProgramStatusUnloaded),
		},
		Metadata: map[string]string{
			"host_id":     s.hostID,
			"artifact_id": artifactID,
		},
	}

	select {
	case s.telemetryQueue <- event:
		return nil
	default:
		return fmt.Errorf("telemetry queue is full")
	}
}

// SendProgramError sends a program error event
func (s *Sender) SendProgramError(artifactID, errorMsg string) error {
	event := types.TelemetryEvent{
		Type:      "program_error",
		Timestamp: time.Now().Format(time.RFC3339),
		Data: types.ProgramTelemetry{
			ArtifactID: artifactID,
			HostID:     s.hostID,
			Timestamp:  time.Now().Format(time.RFC3339),
			Status:     string(types.ProgramStatusFailed),
			Errors:     1,
		},
		Metadata: map[string]string{
			"host_id":     s.hostID,
			"artifact_id": artifactID,
			"error":       errorMsg,
		},
	}

	select {
	case s.telemetryQueue <- event:
		return nil
	default:
		return fmt.Errorf("telemetry queue is full")
	}
}

// SendAgentHeartbeat sends an agent heartbeat
func (s *Sender) SendAgentHeartbeat(loadedPrograms int) error {
	event := types.TelemetryEvent{
		Type:      "agent_heartbeat",
		Timestamp: time.Now().Format(time.RFC3339),
		Data: types.ProgramTelemetry{
			HostID:    s.hostID,
			Timestamp: time.Now().Format(time.RFC3339),
			Status:    "healthy",
		},
		Metadata: map[string]string{
			"host_id":         s.hostID,
			"loaded_programs": fmt.Sprintf("%d", loadedPrograms),
			"agent_version":   "1.0.0",
		},
	}

	select {
	case s.telemetryQueue <- event:
		return nil
	default:
		return fmt.Errorf("telemetry queue is full")
	}
}

// sendLoop is the main sending loop
func (s *Sender) sendLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Telemetry sender context cancelled")
			return

		case <-s.stopChan:
			s.logger.Info("Telemetry sender stopped")
			return

		case event := <-s.telemetryQueue:
			if err := s.publishEvent(event); err != nil {
				s.logger.Error("Failed to publish telemetry event",
					"error", err,
					"event_type", event.Type)
			}

		case <-ticker.C:
			// Send heartbeat
			if err := s.SendAgentHeartbeat(0); err != nil {
				s.logger.Debug("Failed to send heartbeat", "error", err)
			}
		}
	}
}

// publishEvent publishes a telemetry event to NATS
func (s *Sender) publishEvent(event types.TelemetryEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry event: %w", err)
	}

	if err := s.nc.Publish(s.subject, data); err != nil {
		return fmt.Errorf("failed to publish telemetry event: %w", err)
	}

	s.logger.Debug("Published telemetry event",
		"subject", s.subject,
		"event_type", event.Type,
		"artifact_id", event.Data.ArtifactID)

	return nil
}

// SendProgramRolledBack sends a program rolled back event
func (s *Sender) SendProgramRolledBack(artifactID string, reason string, threshold string, value interface{}) error {
	event := types.TelemetryEvent{
		Type:      "program_rolled_back",
		Timestamp: time.Now().Format(time.RFC3339),
		Data: types.ProgramTelemetry{
			ArtifactID: artifactID,
			HostID:     s.hostID,
			Timestamp:  time.Now().Format(time.RFC3339),
			Status:     "rolled_back",
		},
		Metadata: map[string]string{
			"host_id":         s.hostID,
			"artifact_id":     artifactID,
			"rollback_reason": reason,
			"threshold":       threshold,
			"value":           fmt.Sprintf("%v", value),
		},
	}

	select {
	case s.telemetryQueue <- event:
		return nil
	default:
		return fmt.Errorf("telemetry queue is full")
	}
}

// GetQueueSize returns the current telemetry queue size
func (s *Sender) GetQueueSize() int {
	return len(s.telemetryQueue)
}

// IsConnected returns whether the NATS connection is healthy
func (s *Sender) IsConnected() bool {
	return s.nc.IsConnected()
}
