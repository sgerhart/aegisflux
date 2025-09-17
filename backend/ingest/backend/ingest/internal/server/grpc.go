package server

import (
	"context"
	"log/slog"
	"time"

	"aegisflux/backend/ingest/internal/health"
	"aegisflux/backend/ingest/internal/metrics"
	"aegisflux/backend/ingest/internal/nats"
	"aegisflux/backend/ingest/internal/validate"
	"aegisflux/backend/ingest/protos"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	metrics   *metrics.Metrics
	checker   *health.ServiceChecker
}

// NewIngestServer creates a new IngestServer instance
func NewIngestServer(natsURL string, logger *slog.Logger) (*IngestServer, error) {
	// Create metrics
	metricsInstance := metrics.NewMetrics()
	
	// Create health checker
	checker := health.NewServiceChecker(logger)
	
	// Create schema validator
	schemaValidator, err := validate.NewSchemaValidator(logger)
	if err != nil {
		return nil, err
	}
	checker.SetSchemaReady(true)

	// Create NATS publisher
	natsPublisher, err := nats.NewPublisher(natsURL, "events.raw", logger)
	if err != nil {
		return nil, err
	}
	checker.SetNATSReady(natsPublisher.IsReady())

	return &IngestServer{
		logger:    logger,
		validator: schemaValidator,
		publisher: natsPublisher,
		metrics:   metricsInstance,
		checker:   checker,
	}, nil
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

		// Extract host_id from metadata for logging
		hostID := "unknown"
		if h, exists := event.Metadata["host_id"]; exists {
			hostID = h
		}

		// Log event processing start
		s.logger.Info("Processing event", 
			"event_id", event.Id, 
			"event_type", event.Type, 
			"host_id", hostID)

		// Validate the event
		if err := s.validator.ValidateEvent(stream.Context(), event); err != nil {
			s.logger.Warn("Event validation failed", 
				"event_id", event.Id, 
				"event_type", event.Type, 
				"host_id", hostID, 
				"error", err)
			s.metrics.IncrementEventsInvalid()
			// Return InvalidArgument status for validation errors
			return status.Errorf(codes.InvalidArgument, "event validation failed: %v", err)
		}

		// Create context with timeout for publishing
		publishCtx, cancel := context.WithTimeout(stream.Context(), 2*time.Second)
		defer cancel()

		// Publish the event
		if err := s.publisher.PublishEvent(publishCtx, event); err != nil {
			s.logger.Error("Failed to publish event", 
				"event_id", event.Id, 
				"event_type", event.Type, 
				"host_id", hostID, 
				"error", err)
			s.metrics.IncrementNatsPublishErrors()
			// Return Unavailable status for publish failures
			return status.Errorf(codes.Unavailable, "failed to publish event: %v", err)
		}

		// Increment successful event counter
		s.metrics.IncrementEventsTotal()

		s.logger.Info("Event processed successfully", 
			"event_id", event.Id, 
			"event_type", event.Type, 
			"host_id", hostID)
	}

	// Return success acknowledgment
	return stream.SendAndClose(&protos.Ack{
		Ok:      true,
		Message: "Events processed successfully",
	})
}

// GetHealthChecker returns the health checker instance
func (s *IngestServer) GetHealthChecker() *health.ServiceChecker {
	return s.checker
}

// SetGRPCReady sets the gRPC readiness status
func (s *IngestServer) SetGRPCReady(ready bool) {
	s.checker.SetGRPCReady(ready)
}

// Close gracefully shuts down the server and closes connections
func (s *IngestServer) Close() error {
	s.logger.Info("Closing ingest server...")
	
	// Close NATS connection if it's a NATS publisher
	if natsPublisher, ok := s.publisher.(*nats.Publisher); ok {
		if err := natsPublisher.Close(); err != nil {
			s.logger.Error("Failed to close NATS connection", "error", err)
			return err
		}
		s.checker.SetNATSReady(false)
	}
	
	s.logger.Info("Ingest server closed")
	return nil
}


