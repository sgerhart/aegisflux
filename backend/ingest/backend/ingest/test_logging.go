package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"aegisflux/backend/ingest/internal/metrics"
	"aegisflux/backend/ingest/internal/validate"
	"aegisflux/backend/ingest/protos"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	
	// Create validator and metrics
	validator, err := validate.NewSchemaValidator(logger)
	if err != nil {
		logger.Error("Failed to create validator", "error", err)
		os.Exit(1)
	}
	
	metrics := metrics.NewMetrics()
	
	// Test events
	events := []*protos.Event{
		{
			Id:        "test-001",
			Type:      "security",
			Source:    "firewall",
			Timestamp: 1640995200000,
			Metadata: map[string]string{
				"host_id": "host-001",
				"user":    "admin",
			},
		},
		{
			Id:        "test-002",
			Type:      "invalid-type", // This should fail validation
			Source:    "firewall",
			Timestamp: 1640995200000,
			Metadata: map[string]string{
				"host_id": "host-002",
			},
		},
		{
			Id:        "test-003",
			Type:      "audit",
			Source:    "auth-service",
			Timestamp: 1640995200000,
			Metadata: map[string]string{
				"host_id": "host-003",
			},
		},
	}
	
	ctx := context.Background()
	
	for _, event := range events {
		// Extract host_id for logging
		hostID := "unknown"
		if h, exists := event.Metadata["host_id"]; exists {
			hostID = h
		}
		
		// Log event processing start
		logger.Info("Processing event", 
			"event_id", event.Id, 
			"event_type", event.Type, 
			"host_id", hostID)
		
		// Validate the event
		if err := validator.ValidateEvent(ctx, event); err != nil {
			logger.Warn("Event validation failed", 
				"event_id", event.Id, 
				"event_type", event.Type, 
				"host_id", hostID, 
				"error", err)
			metrics.IncrementEventsInvalid()
			continue
		}
		
		// Simulate successful processing
		metrics.IncrementEventsTotal()
		
		logger.Info("Event processed successfully", 
			"event_id", event.Id, 
			"event_type", event.Type, 
			"host_id", hostID)
		
		time.Sleep(100 * time.Millisecond)
	}
	
	logger.Info("Logging test completed")
}

