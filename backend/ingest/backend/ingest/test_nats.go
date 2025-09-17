package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"aegisflux/backend/ingest/internal/nats"
	"aegisflux/backend/ingest/protos"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	
	// Test NATS publisher
	publisher, err := nats.NewPublisher("nats://localhost:4222", "events.raw", logger)
	if err != nil {
		logger.Error("Failed to create publisher", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	// Wait for connection
	time.Sleep(100 * time.Millisecond)

	if !publisher.IsReady() {
		logger.Error("Publisher not ready")
		os.Exit(1)
	}

	// Test publishing an event
	event := &protos.Event{
		Id:        "test-123",
		Type:      "security",
		Source:    "firewall",
		Timestamp: 1640995200000,
		Metadata: map[string]string{
			"host_id": "host-001",
			"user":    "admin",
		},
		Payload: []byte("test payload"),
	}

	ctx := context.Background()
	err = publisher.PublishEvent(ctx, event)
	if err != nil {
		logger.Error("Failed to publish event", "error", err)
		os.Exit(1)
	}

	logger.Info("Event published successfully", "event_id", event.Id)
}
