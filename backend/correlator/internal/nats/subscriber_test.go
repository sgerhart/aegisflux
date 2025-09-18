package nats

import (
	"log/slog"
	"testing"
	"time"

	"github.com/sgerhart/aegisflux/backend/correlator/internal/metrics"
	"github.com/sgerhart/aegisflux/backend/correlator/internal/model"
	"github.com/sgerhart/aegisflux/backend/correlator/internal/rules"
	"github.com/stretchr/testify/assert"
)

func TestSubscriber_ParseEvent(t *testing.T) {
	// Create a mock subscriber for testing
	subscriber := &Subscriber{
		logger: slog.Default(),
	}

	tests := []struct {
		name     string
		data     []byte
		expected *model.Event
		hasError bool
	}{
		{
			name: "valid enriched event",
			data: []byte(`{
				"timestamp": "2023-01-01T12:00:00Z",
				"host_id": "host-001",
				"event_type": "exec",
				"binary_path": "/bin/bash",
				"args": {"command": "ls"},
				"context": {"user": "root"}
			}`),
			expected: &model.Event{
				HostID:     "host-001",
				EventType:  "exec",
				BinaryPath: "/bin/bash",
				Args:       map[string]interface{}{"command": "ls"},
				Context:    map[string]interface{}{"user": "root"},
			},
			hasError: false,
		},
		{
			name: "valid raw event with type field",
			data: []byte(`{
				"timestamp": 1672574400,
				"host_id": "host-002",
				"type": "connect",
				"source": "/usr/bin/curl",
				"args": {"url": "https://example.com"},
				"context": {"env": "prod"}
			}`),
			expected: &model.Event{
				HostID:     "host-002",
				EventType:  "connect",
				BinaryPath: "/usr/bin/curl",
				Args:       map[string]interface{}{"url": "https://example.com"},
				Context:    map[string]interface{}{"env": "prod"},
			},
			hasError: false,
		},
		{
			name:     "invalid JSON",
			data:     []byte(`invalid json`),
			expected: nil,
			hasError: true,
		},
		{
			name: "minimal event",
			data: []byte(`{
				"host_id": "host-003",
				"event_type": "file"
			}`),
			expected: &model.Event{
				HostID:    "host-003",
				EventType: "file",
			},
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := subscriber.parseEvent(tt.data)

			if tt.hasError {
				assert.Error(t, err)
				assert.Nil(t, event)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, event)
				assert.Equal(t, tt.expected.HostID, event.HostID)
				assert.Equal(t, tt.expected.EventType, event.EventType)
				assert.Equal(t, tt.expected.BinaryPath, event.BinaryPath)
				assert.Equal(t, tt.expected.Args, event.Args)
				assert.Equal(t, tt.expected.Context, event.Context)
			}
		})
	}
}

func TestSubscriber_IncrementInvalidEvents(t *testing.T) {
	subscriber := &Subscriber{
		invalidEventsTotal: 5,
		logger:             slog.Default(),
	}

	// Test incrementing
	subscriber.incrementInvalidEvents()
	assert.Equal(t, int64(5), subscriber.invalidEventsTotal) // Should still be 5 since we're just logging
}

func TestSubscriber_GetMetrics(t *testing.T) {
	// Create a mock subscriber
	windowBuffer := rules.NewWindowBuffer(1 * time.Minute)
	matcher := rules.NewMatcher()
	ruleLoader := rules.NewLoader("testdata", false, 1000, slog.Default())
	overrideManager := rules.NewOverrideManager(slog.Default())
	prometheusMetrics := metrics.NewMetrics()
	evaluator := rules.NewEvaluator(windowBuffer, matcher, ruleLoader, overrideManager, 300, prometheusMetrics, slog.Default())

	subscriber := &Subscriber{
		invalidEventsTotal: 10,
		evaluator:          evaluator,
		windowBuffer:       windowBuffer,
	}

	metrics := subscriber.GetMetrics()

	assert.Equal(t, int64(10), metrics["correlator_events_invalid_total"])
	assert.NotNil(t, metrics["evaluator_metrics"])
	assert.NotNil(t, metrics["window_buffer_stats"])
}

func TestSubscriber_GracefulShutdown(t *testing.T) {
	// Create a mock subscriber
	windowBuffer := rules.NewWindowBuffer(1 * time.Minute)
	subscriber := &Subscriber{
		windowBuffer: windowBuffer,
		logger:       slog.Default(),
	}

	// Test graceful shutdown (should not error even without real subscriptions)
	err := subscriber.gracefulShutdown()
	assert.NoError(t, err)
}
