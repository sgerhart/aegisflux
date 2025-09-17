package server

import (
	"context"
	"log/slog"

	"aegisflux/backend/ingest/protos"
)

// Validator interface for event validation
type Validator interface {
	ValidateEvent(ctx context.Context, e *protos.Event) error
}

// Publisher interface for event publishing
type Publisher interface {
	PublishEvent(ctx context.Context, e *protos.Event) error
}

// IngestServer implements the gRPC Ingest service
type IngestServer struct {
	protos.UnimplementedIngestServer
	logger    *slog.Logger
	validator Validator
	publisher Publisher
}

// NewIngestServer creates a new IngestServer instance
func NewIngestServer(logger *slog.Logger) *IngestServer {
	return &IngestServer{
		logger:    logger,
		validator: &stubValidator{},
		publisher: &stubPublisher{},
	}
}

// PostEvents handles streaming events
func (s *IngestServer) PostEvents(stream protos.Ingest_PostEventsServer) error {
	s.logger.Info("Starting event stream processing")

	eventCount := 0
	for {
		event, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				s.logger.Info("Event stream closed by client", "events_processed", eventCount)
				break
			}
			s.logger.Error("Error receiving event from stream", "error", err)
			return err
		}

		eventCount++

		// Validate the event
		if err := s.validator.ValidateEvent(stream.Context(), event); err != nil {
			s.logger.Error("Event validation failed", "event_id", event.Id, "error", err)
			// Continue processing other events even if one fails validation
			continue
		}

		// Publish the event
		if err := s.publisher.PublishEvent(stream.Context(), event); err != nil {
			s.logger.Error("Failed to publish event", "event_id", event.Id, "error", err)
			// Continue processing other events even if one fails to publish
			continue
		}

		s.logger.Debug("Event processed successfully", "event_id", event.Id, "type", event.Type)
	}

	// Return success acknowledgment
	return stream.SendAndClose(&protos.Ack{
		Ok:      true,
		Message: "Events processed successfully",
	})
}

// stubValidator is a stub implementation of Validator
type stubValidator struct{}

func (v *stubValidator) ValidateEvent(ctx context.Context, e *protos.Event) error {
	// TODO: Implement actual validation logic
	return nil
}

// stubPublisher is a stub implementation of Publisher
type stubPublisher struct{}

func (p *stubPublisher) PublishEvent(ctx context.Context, e *protos.Event) error {
	// TODO: Implement actual NATS publishing logic
	return nil
}

