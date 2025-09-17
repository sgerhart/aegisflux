package validate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"aegisflux/backend/ingest/protos"
)

func TestSchemaValidator_ValidateEvent(t *testing.T) {
	// Create a test logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	// Create validator
	validator, err := NewSchemaValidator(logger)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name    string
		event   *protos.Event
		wantErr bool
	}{
		{
			name: "valid event with all fields",
			event: &protos.Event{
				Id:        "test-123",
				Type:      "security",
				Source:    "firewall",
				Timestamp: 1640995200000,
				Metadata: map[string]string{
					"host_id": "host-001",
					"user":    "admin",
				},
				Payload: []byte("dGVzdCBwYXlsb2Fk"), // base64 encoded "test payload"
			},
			wantErr: false,
		},
		{
			name: "valid event with minimal fields",
			event: &protos.Event{
				Id:        "test-456",
				Type:      "audit",
				Source:    "auth-service",
				Timestamp: 1640995200000,
			},
			wantErr: false,
		},
		{
			name: "missing required id",
			event: &protos.Event{
				Type:      "security",
				Source:    "firewall",
				Timestamp: 1640995200000,
			},
			wantErr: true,
		},
		{
			name: "missing required type",
			event: &protos.Event{
				Id:        "test-789",
				Source:    "firewall",
				Timestamp: 1640995200000,
			},
			wantErr: true,
		},
		{
			name: "missing required source",
			event: &protos.Event{
				Id:        "test-101",
				Type:      "security",
				Timestamp: 1640995200000,
			},
			wantErr: true,
		},
		{
			name: "zero timestamp (invalid)",
			event: &protos.Event{
				Id:        "test-102",
				Type:      "security",
				Source:    "firewall",
				Timestamp: 0, // Zero timestamp should be invalid
			},
			wantErr: true,
		},
		{
			name: "invalid event type",
			event: &protos.Event{
				Id:        "test-103",
				Type:      "invalid-type",
				Source:    "firewall",
				Timestamp: 1640995200000,
			},
			wantErr: true,
		},
		{
			name: "negative timestamp",
			event: &protos.Event{
				Id:        "test-104",
				Type:      "security",
				Source:    "firewall",
				Timestamp: -1,
			},
			wantErr: true,
		},
		{
			name: "empty id",
			event: &protos.Event{
				Id:        "",
				Type:      "security",
				Source:    "firewall",
				Timestamp: 1640995200000,
			},
			wantErr: true,
		},
		{
			name: "empty type",
			event: &protos.Event{
				Id:        "test-105",
				Type:      "",
				Source:    "firewall",
				Timestamp: 1640995200000,
			},
			wantErr: true,
		},
		{
			name: "empty source",
			event: &protos.Event{
				Id:        "test-106",
				Type:      "security",
				Source:    "",
				Timestamp: 1640995200000,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateEvent(ctx, tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSchemaValidator_Concurrency(t *testing.T) {
	// Create a test logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	// Create validator
	validator, err := NewSchemaValidator(logger)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	ctx := context.Background()
	
	// Test concurrent access
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			event := &protos.Event{
				Id:        fmt.Sprintf("test-%d", id),
				Type:      "security",
				Source:    "firewall",
				Timestamp: 1640995200000,
			}
			
			err := validator.ValidateEvent(ctx, event)
			if err != nil {
				t.Errorf("Concurrent validation failed: %v", err)
			}
			
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestSchemaValidator_ReloadSchema(t *testing.T) {
	// Create a test logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	// Create validator
	validator, err := NewSchemaValidator(logger)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Test reloading schema
	err = validator.ReloadSchema()
	if err != nil {
		t.Errorf("ReloadSchema() error = %v", err)
	}
}
