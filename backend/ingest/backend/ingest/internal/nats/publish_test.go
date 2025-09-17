package nats

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"aegisflux/backend/ingest/protos"
	"github.com/nats-io/nats.go"
)

func TestPublisher_PublishEvent(t *testing.T) {
	// Create a test logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Try to connect to NATS, skip test if not available
	conn, err := nats.Connect("nats://localhost:4222", nats.Timeout(2*time.Second))
	if err != nil {
		t.Skip("NATS server not available, skipping test")
	}
	conn.Close()

	// Create publisher
	publisher, err := NewPublisher("nats://localhost:4222", "test.events", logger)
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	// Wait for connection to be ready
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()

	tests := []struct {
		name    string
		event   *protos.Event
		wantErr bool
	}{
		{
			name: "valid event with metadata",
			event: &protos.Event{
				Id:        "test-123",
				Type:      "security",
				Source:    "firewall",
				Timestamp: 1640995200000,
				Metadata: map[string]string{
					"host_id": "host-001",
					"user":    "admin",
				},
				Payload: []byte("test payload"),
			},
			wantErr: false,
		},
		{
			name: "valid event without metadata",
			event: &protos.Event{
				Id:        "test-456",
				Type:      "audit",
				Source:    "auth-service",
				Timestamp: 1640995200000,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := publisher.PublishEvent(ctx, tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("PublishEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPublisher_IsReady(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Test with invalid URL to ensure it fails fast
	_, err := NewPublisher("nats://invalid:4222", "test.events", logger)
	if err == nil {
		t.Error("Expected error for invalid NATS URL, got nil")
	}

	// Test with valid URL if NATS is available
	conn, err := nats.Connect("nats://localhost:4222", nats.Timeout(2*time.Second))
	if err != nil {
		t.Skip("NATS server not available, skipping readiness test")
	}
	conn.Close()

	publisher, err := NewPublisher("nats://localhost:4222", "test.events", logger)
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	// Wait for connection
	time.Sleep(100 * time.Millisecond)

	if !publisher.IsReady() {
		t.Error("Expected publisher to be ready")
	}
}

func TestPublisher_Concurrency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Check if NATS is available
	conn, err := nats.Connect("nats://localhost:4222", nats.Timeout(2*time.Second))
	if err != nil {
		t.Skip("NATS server not available, skipping concurrency test")
	}
	conn.Close()

	publisher, err := NewPublisher("nats://localhost:4222", "test.events", logger)
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	// Wait for connection
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	done := make(chan bool, 10)

	// Test concurrent publishing
	for i := 0; i < 10; i++ {
		go func(id int) {
			event := &protos.Event{
				Id:        fmt.Sprintf("test-%d", id),
				Type:      "security",
				Source:    "firewall",
				Timestamp: 1640995200000,
			}

			err := publisher.PublishEvent(ctx, event)
			if err != nil {
				t.Errorf("Concurrent publish failed: %v", err)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// MockPublisher is a mock implementation for testing without NATS
type MockPublisher struct {
	events []*protos.Event
	logger *slog.Logger
}

func NewMockPublisher(logger *slog.Logger) *MockPublisher {
	return &MockPublisher{
		events: make([]*protos.Event, 0),
		logger: logger,
	}
}

func (m *MockPublisher) PublishEvent(ctx context.Context, e *protos.Event) error {
	m.events = append(m.events, e)
	m.logger.Debug("Mock publisher received event", "event_id", e.Id)
	return nil
}

func (m *MockPublisher) IsReady() bool {
	return true
}

func (m *MockPublisher) Close() error {
	return nil
}

func (m *MockPublisher) GetEvents() []*protos.Event {
	return m.events
}

func TestMockPublisher(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	mock := NewMockPublisher(logger)
	ctx := context.Background()

	event := &protos.Event{
		Id:        "test-123",
		Type:      "security",
		Source:    "firewall",
		Timestamp: 1640995200000,
	}

	err := mock.PublishEvent(ctx, event)
	if err != nil {
		t.Errorf("Mock publish failed: %v", err)
	}

	events := mock.GetEvents()
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	if events[0].Id != event.Id {
		t.Errorf("Expected event ID %s, got %s", event.Id, events[0].Id)
	}
}
