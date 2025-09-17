package rules

import (
	"testing"
	"time"

	"github.com/aegisflux/correlator/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestEvaluator_EventMatchesPattern(t *testing.T) {
	evaluator := &Evaluator{}

	tests := []struct {
		name     string
		event    *model.Event
		pattern  EventPattern
		expected bool
	}{
		{
			name: "exact event type match",
			event: &model.Event{
				EventType:  "exec",
				BinaryPath: "/bin/bash",
			},
			pattern: EventPattern{
				EventType: "exec",
			},
			expected: true,
		},
		{
			name: "event type mismatch",
			event: &model.Event{
				EventType:  "connect",
				BinaryPath: "/bin/bash",
			},
			pattern: EventPattern{
				EventType: "exec",
			},
			expected: false,
		},
		{
			name: "binary path regex match",
			event: &model.Event{
				EventType:  "exec",
				BinaryPath: "/bin/bash",
			},
			pattern: EventPattern{
				EventType:       "exec",
				BinaryPathRegex: "^/bin/bash$",
			},
			expected: true,
		},
		{
			name: "binary path regex mismatch",
			event: &model.Event{
				EventType:  "exec",
				BinaryPath: "/usr/bin/python",
			},
			pattern: EventPattern{
				EventType:       "exec",
				BinaryPathRegex: "^/bin/bash$",
			},
			expected: false,
		},
		{
			name: "args pattern match",
			event: &model.Event{
				EventType: "exec",
				Args: map[string]interface{}{
					"command": "ls",
					"args":    []string{"-la"},
				},
			},
			pattern: EventPattern{
				EventType: "exec",
				Args: map[string]string{
					"command": "ls",
				},
			},
			expected: true,
		},
		{
			name: "args pattern mismatch",
			event: &model.Event{
				EventType: "exec",
				Args: map[string]interface{}{
					"command": "cat",
				},
			},
			pattern: EventPattern{
				EventType: "exec",
				Args: map[string]string{
					"command": "ls",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluator.eventMatchesPattern(tt.event, tt.pattern, 1*time.Minute)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_RenderTemplate(t *testing.T) {
	evaluator := &Evaluator{}

	event := &model.Event{
		HostID:     "host-001",
		EventType:  "exec",
		BinaryPath: "/bin/bash",
		Timestamp:  time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Args: map[string]interface{}{
			"command": "ls",
			"path":    "/tmp",
		},
		Context: map[string]interface{}{
			"user": "root",
			"env":  "prod",
		},
	}

	rule := Rule{
		Metadata: RuleMetadata{
			ID:      "test-rule",
			Name:    "Test Rule",
			Version: "1.0.0",
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "basic fields",
			template: "Event {event_type} on {host_id}",
			expected: "Event exec on host-001",
		},
		{
			name:     "args fields",
			template: "Command {args.command} in {args.path}",
			expected: "Command ls in /tmp",
		},
		{
			name:     "context fields",
			template: "User {context.user} in {context.env}",
			expected: "User root in prod",
		},
		{
			name:     "rule fields",
			template: "Rule {rule.id} ({rule.name})",
			expected: "Rule test-rule (Test Rule)",
		},
		{
			name:     "timestamp",
			template: "Event at {timestamp}",
			expected: "Event at 2023-01-01T12:00:00Z",
		},
		{
			name:     "mixed fields",
			template: "Rule {rule.id} triggered by {event_type} from {context.user}@{host_id}",
			expected: "Rule test-rule triggered by exec from root@host-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluator.renderTemplate(tt.template, event, rule)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHostLabelsCache(t *testing.T) {
	cache := NewHostLabelsCache(1) // 1 second TTL

	// Test set and get
	labels := []string{"role:web", "env:prod"}
	cache.set("host-001", labels)

	retrievedLabels, hit := cache.get("host-001")
	assert.True(t, hit)
	assert.Equal(t, labels, retrievedLabels)

	// Test TTL expiration
	time.Sleep(1100 * time.Millisecond)
	retrievedLabels, hit = cache.get("host-001")
	assert.False(t, hit)
	assert.Nil(t, retrievedLabels)

	// Test clear
	cache.set("host-002", []string{"role:db"})
	cache.clear()
	_, hit = cache.get("host-002")
	assert.False(t, hit)
}

func TestDedupeCache(t *testing.T) {
	cache := NewDedupeCache()

	// Test set and isWithinCooldown
	cache.set("key1", time.Now())
	assert.True(t, cache.isWithinCooldown("key1", 1*time.Minute))
	assert.False(t, cache.isWithinCooldown("key2", 1*time.Minute))

	// Test cooldown expiration
	time.Sleep(100 * time.Millisecond)
	assert.True(t, cache.isWithinCooldown("key1", 1*time.Minute))
	assert.False(t, cache.isWithinCooldown("key1", 50*time.Millisecond))

	// Test clear
	cache.clear()
	assert.False(t, cache.isWithinCooldown("key1", 1*time.Minute))
}

func TestEvaluationMetrics(t *testing.T) {
	evaluator := &Evaluator{
		metrics: &EvaluationMetrics{},
	}

	// Test initial metrics
	metrics := evaluator.GetMetrics()
	assert.Equal(t, int64(0), metrics["events_processed"])
	assert.Equal(t, int64(0), metrics["rules_evaluated"])
	assert.Equal(t, int64(0), metrics["findings_generated"])
	assert.Equal(t, int64(0), metrics["findings_deduplicated"])
	assert.Equal(t, int64(0), metrics["cache_hits"])
	assert.Equal(t, int64(0), metrics["cache_misses"])

	// Test incrementing metrics
	evaluator.metrics.incrementEventsProcessed()
	evaluator.metrics.incrementRulesEvaluated()
	evaluator.metrics.incrementFindingsGenerated()
	evaluator.metrics.incrementFindingsDeduplicated()
	evaluator.metrics.incrementCacheHits()
	evaluator.metrics.incrementCacheMisses()

	metrics = evaluator.GetMetrics()
	assert.Equal(t, int64(1), metrics["events_processed"])
	assert.Equal(t, int64(1), metrics["rules_evaluated"])
	assert.Equal(t, int64(1), metrics["findings_generated"])
	assert.Equal(t, int64(1), metrics["findings_deduplicated"])
	assert.Equal(t, int64(1), metrics["cache_hits"])
	assert.Equal(t, int64(1), metrics["cache_misses"])
}

func TestEvaluator_ExtractLabelsFromContext(t *testing.T) {
	evaluator := &Evaluator{}

	tests := []struct {
		name     string
		context  map[string]interface{}
		expected []string
	}{
		{
			name: "labels field",
			context: map[string]interface{}{
				"labels": "role:web env:prod",
			},
			expected: []string{"role:web", "env:prod"},
		},
		{
			name: "tags field",
			context: map[string]interface{}{
				"tags": "database,production",
			},
			expected: []string{"database", "production"},
		},
		{
			name: "key-value pairs",
			context: map[string]interface{}{
				"user": "admin",
				"env":  "prod",
			},
			expected: []string{"user:admin", "env:prod"},
		},
		{
			name: "mixed sources",
			context: map[string]interface{}{
				"labels": "role:web",
				"user":   "admin",
			},
			expected: []string{"role:web", "user:admin"},
		},
		{
			name:     "empty context",
			context:  map[string]interface{}{},
			expected: []string(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluator.extractLabelsFromContext(tt.context)
			if tt.name == "key-value pairs" {
				// For key-value pairs, order is not guaranteed due to map iteration
				assert.Len(t, result, len(tt.expected))
				for _, expectedLabel := range tt.expected {
					assert.Contains(t, result, expectedLabel)
				}
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
