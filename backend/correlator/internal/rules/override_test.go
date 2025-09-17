package rules

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOverrideManager_AddOverride(t *testing.T) {
	logger := slog.Default()
	om := NewOverrideManager(logger)

	tests := []struct {
		name        string
		ruleID      string
		enabled     *bool
		severity    *string
		confidence  *float64
		ttlSeconds  *int
		description string
		expectError bool
	}{
		{
			name:        "valid override with all fields",
			ruleID:      "test-rule-1",
			enabled:     boolPtr(true),
			severity:    stringPtr("high"),
			confidence:  float64Ptr(0.8),
			ttlSeconds:  intPtr(3600),
			description: "Test override",
			expectError: false,
		},
		{
			name:        "valid override with minimal fields",
			ruleID:      "test-rule-2",
			description: "Minimal override",
			expectError: false,
		},
		{
			name:        "invalid rule ID",
			ruleID:      "",
			description: "Empty rule ID",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			override, err := om.AddOverride(
				tt.ruleID,
				tt.enabled,
				tt.severity,
				tt.confidence,
				tt.ttlSeconds,
				tt.description,
			)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, override)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, override)
				assert.Equal(t, tt.ruleID, override.RuleID)
				assert.Equal(t, tt.enabled, override.Enabled)
				assert.Equal(t, tt.severity, override.Severity)
				assert.Equal(t, tt.confidence, override.Confidence)
				assert.Equal(t, tt.ttlSeconds, override.TTLSeconds)
				assert.Equal(t, tt.description, override.Description)
				assert.NotEmpty(t, override.ID)
				assert.False(t, override.CreatedAt.IsZero())
				assert.False(t, override.UpdatedAt.IsZero())
			}
		})
	}
}

func TestOverrideManager_RemoveOverride(t *testing.T) {
	logger := slog.Default()
	om := NewOverrideManager(logger)

	// Add an override first
	override, err := om.AddOverride("test-rule", boolPtr(true), nil, nil, nil, "test")
	assert.NoError(t, err)
	assert.NotNil(t, override)

	overrideID := override.ID

	// Test removing existing override
	err = om.RemoveOverride(overrideID)
	assert.NoError(t, err)

	// Test removing non-existent override
	err = om.RemoveOverride("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "override not found")
}

func TestOverrideManager_GetOverride(t *testing.T) {
	logger := slog.Default()
	om := NewOverrideManager(logger)

	// Add an override
	originalOverride, err := om.AddOverride("test-rule", boolPtr(false), stringPtr("low"), nil, nil, "test")
	assert.NoError(t, err)

	// Test getting existing override
	retrievedOverride, err := om.GetOverride(originalOverride.ID)
	assert.NoError(t, err)
	assert.Equal(t, originalOverride, retrievedOverride)

	// Test getting non-existent override
	_, err = om.GetOverride("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "override not found")
}

func TestOverrideManager_ListOverrides(t *testing.T) {
	logger := slog.Default()
	om := NewOverrideManager(logger)

	// Initially should be empty
	overrides := om.ListOverrides()
	assert.Empty(t, overrides)

	// Add some overrides
	_, err := om.AddOverride("rule-1", boolPtr(true), nil, nil, nil, "override 1")
	assert.NoError(t, err)

	_, err = om.AddOverride("rule-2", boolPtr(false), stringPtr("high"), nil, nil, "override 2")
	assert.NoError(t, err)

	// Should now have 2 overrides
	overrides = om.ListOverrides()
	assert.Len(t, overrides, 2)

	// Check that summaries contain the right data
	ruleIDs := make(map[string]bool)
	for _, override := range overrides {
		ruleIDs[override.RuleID] = true
	}
	assert.True(t, ruleIDs["rule-1"])
	assert.True(t, ruleIDs["rule-2"])
}

func TestOverrideManager_GetOverridesForRule(t *testing.T) {
	logger := slog.Default()
	om := NewOverrideManager(logger)

	// Add overrides for different rules
	_, err := om.AddOverride("rule-1", boolPtr(true), nil, nil, nil, "override 1")
	assert.NoError(t, err)

	_, err = om.AddOverride("rule-1", boolPtr(false), nil, nil, nil, "override 1b")
	assert.NoError(t, err)

	_, err = om.AddOverride("rule-2", boolPtr(true), nil, nil, nil, "override 2")
	assert.NoError(t, err)

	// Test getting overrides for rule-1
	rule1Overrides := om.GetOverridesForRule("rule-1")
	assert.Len(t, rule1Overrides, 2)

	// Test getting overrides for rule-2
	rule2Overrides := om.GetOverridesForRule("rule-2")
	assert.Len(t, rule2Overrides, 1)

	// Test getting overrides for non-existent rule
	nonExistentOverrides := om.GetOverridesForRule("non-existent")
	assert.Empty(t, nonExistentOverrides)
}

func TestOverrideManager_ApplyOverrides(t *testing.T) {
	logger := slog.Default()
	om := NewOverrideManager(logger)

	// Create a test rule
	originalRule := &Rule{
		Metadata: RuleMetadata{
			ID: "test-rule",
		},
		Spec: RuleSpec{
			Enabled: true,
			Outcome: Outcome{
				Severity:   "medium",
				Confidence: 0.5,
			},
			TTLSeconds: 1800,
		},
	}

	// Test with no overrides
	modifiedRule := om.ApplyOverrides(originalRule)
	assert.Equal(t, originalRule, modifiedRule)

	// Add an override
	_, err := om.AddOverride("test-rule", boolPtr(false), stringPtr("high"), float64Ptr(0.8), intPtr(3600), "test override")
	assert.NoError(t, err)

	// Test with override applied
	modifiedRule = om.ApplyOverrides(originalRule)
	assert.NotEqual(t, originalRule, modifiedRule)
	assert.False(t, modifiedRule.Spec.Enabled)
	assert.Equal(t, "high", modifiedRule.Spec.Outcome.Severity)
	assert.Equal(t, 0.8, modifiedRule.Spec.Outcome.Confidence)
	assert.Equal(t, 3600, modifiedRule.Spec.TTLSeconds)
}

func TestOverrideManager_ValidateOverride(t *testing.T) {
	logger := slog.Default()
	om := NewOverrideManager(logger)

	tests := []struct {
		name        string
		ruleID      string
		enabled     *bool
		severity    *string
		confidence  *float64
		ttlSeconds  *int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid override",
			ruleID:      "test-rule",
			enabled:     boolPtr(true),
			severity:    stringPtr("high"),
			confidence:  float64Ptr(0.8),
			ttlSeconds:  intPtr(3600),
			expectError: false,
		},
		{
			name:        "empty rule ID",
			ruleID:      "",
			expectError: true,
			errorMsg:    "rule_id is required",
		},
		{
			name:        "invalid severity",
			ruleID:      "test-rule",
			severity:    stringPtr("invalid"),
			expectError: true,
			errorMsg:    "invalid severity",
		},
		{
			name:        "invalid confidence - too low",
			ruleID:      "test-rule",
			confidence:  float64Ptr(-0.1),
			expectError: true,
			errorMsg:    "confidence must be between 0.0 and 1.0",
		},
		{
			name:        "invalid confidence - too high",
			ruleID:      "test-rule",
			confidence:  float64Ptr(1.1),
			expectError: true,
			errorMsg:    "confidence must be between 0.0 and 1.0",
		},
		{
			name:        "invalid TTL",
			ruleID:      "test-rule",
			ttlSeconds:  intPtr(-1),
			expectError: true,
			errorMsg:    "ttl_seconds must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := om.ValidateOverride(
				tt.ruleID,
				tt.enabled,
				tt.severity,
				tt.confidence,
				tt.ttlSeconds,
			)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOverrideManager_AddOverrideFromJSON(t *testing.T) {
	logger := slog.Default()
	om := NewOverrideManager(logger)

	tests := []struct {
		name        string
		jsonData    string
		expectError bool
	}{
		{
			name: "valid JSON",
			jsonData: `{
				"rule_id": "test-rule",
				"enabled": true,
				"severity": "high",
				"confidence": 0.8,
				"ttl_seconds": 3600,
				"description": "Test override"
			}`,
			expectError: false,
		},
		{
			name: "minimal JSON",
			jsonData: `{
				"rule_id": "test-rule",
				"description": "Minimal override"
			}`,
			expectError: false,
		},
		{
			name:        "invalid JSON",
			jsonData:    `invalid json`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			override, err := om.AddOverrideFromJSON([]byte(tt.jsonData))

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, override)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, override)
				assert.Equal(t, "test-rule", override.RuleID)
			}
		})
	}
}

func TestOverrideManager_GetStats(t *testing.T) {
	logger := slog.Default()
	om := NewOverrideManager(logger)

	// Initially should have zero stats
	stats := om.GetStats()
	assert.Equal(t, 0, stats["total_overrides"])
	assert.Equal(t, 0, stats["enabled_count"])
	assert.Equal(t, 0, stats["disabled_count"])

	// Add some overrides
	_, err := om.AddOverride("rule-1", boolPtr(true), nil, nil, nil, "enabled override")
	assert.NoError(t, err)

	_, err = om.AddOverride("rule-2", boolPtr(false), nil, nil, nil, "disabled override")
	assert.NoError(t, err)

	_, err = om.AddOverride("rule-3", nil, nil, nil, nil, "no enabled field")
	assert.NoError(t, err)

	// Check stats
	stats = om.GetStats()
	assert.Equal(t, 3, stats["total_overrides"])
	assert.Equal(t, 1, stats["enabled_count"])
	assert.Equal(t, 1, stats["disabled_count"])
}

func TestOverrideManager_ClearOverrides(t *testing.T) {
	logger := slog.Default()
	om := NewOverrideManager(logger)

	// Add some overrides
	_, err := om.AddOverride("rule-1", boolPtr(true), nil, nil, nil, "override 1")
	assert.NoError(t, err)

	_, err = om.AddOverride("rule-2", boolPtr(false), nil, nil, nil, "override 2")
	assert.NoError(t, err)

	// Verify they exist
	overrides := om.ListOverrides()
	assert.Len(t, overrides, 2)

	// Clear all overrides
	om.ClearOverrides()

	// Verify they're gone
	overrides = om.ListOverrides()
	assert.Empty(t, overrides)

	stats := om.GetStats()
	assert.Equal(t, 0, stats["total_overrides"])
}

// Helper functions for creating pointers
func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	return &s
}

func float64Ptr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
