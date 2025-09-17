package guardrails

import (
	"log/slog"
	"os"
	"testing"

	"aegisflux/backend/decision/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuardrails_EnforceToCanaryWithNeverBlock(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	guardrails := NewGuardrails(logger)

	t.Run("enforce_mode_with_never_block_label_goes_to_canary", func(t *testing.T) {
		// Set environment for never block labels
		originalEnv := os.Getenv("DECISION_NEVER_BLOCK_LABELS")
		os.Setenv("DECISION_NEVER_BLOCK_LABELS", "production,critical,database")
		defer func() {
			if originalEnv != "" {
				os.Setenv("DECISION_NEVER_BLOCK_LABELS", originalEnv)
			} else {
				os.Unsetenv("DECISION_NEVER_BLOCK_LABELS")
			}
		}()

		decision, err := guardrails.DecideStrategy("enforce", 3, []string{"production", "web"})

		require.NoError(t, err)
		assert.Equal(t, model.StrategyModeCanary, decision.Strategy, "Should downgrade enforce to canary with never block labels")
		assert.Contains(t, decision.Reasons, "Target has NEVER_BLOCK_LABELS", "Should mention never block labels in reasons")
		assert.Contains(t, decision.AppliedRules, "never_block_labels", "Should apply never_block_labels rule")
	})
}

func TestGuardrails_CanaryToSuggestWithZeroCanary(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	guardrails := NewGuardrails(logger)

	t.Run("canary_mode_with_zero_max_canary_goes_to_suggest", func(t *testing.T) {
		// Set max canary hosts to 0
		originalEnv := os.Getenv("DECISION_MAX_CANARY_HOSTS")
		os.Setenv("DECISION_MAX_CANARY_HOSTS", "0")
		defer func() {
			if originalEnv != "" {
				os.Setenv("DECISION_MAX_CANARY_HOSTS", originalEnv)
			} else {
				os.Unsetenv("DECISION_MAX_CANARY_HOSTS")
			}
		}()

		decision, err := guardrails.DecideStrategy("canary", 5, []string{"web"})

		require.NoError(t, err)
		assert.Equal(t, model.StrategyModeSuggest, decision.Strategy, "Should downgrade canary to suggest when max canary is 0")
		assert.Equal(t, 0, decision.CanarySize, "Canary size should be 0")
		assert.Contains(t, decision.Reasons, "Canary size is 0 in enforce/canary mode", "Should mention canary size reason")
		assert.Contains(t, decision.AppliedRules, "canary_size_zero", "Should apply canary_size_zero rule")
	})

	t.Run("enforce_mode_with_zero_max_canary_goes_to_suggest", func(t *testing.T) {
		// Set max canary hosts to 0
		originalEnv := os.Getenv("DECISION_MAX_CANARY_HOSTS")
		os.Setenv("DECISION_MAX_CANARY_HOSTS", "0")
		defer func() {
			if originalEnv != "" {
				os.Setenv("DECISION_MAX_CANARY_HOSTS", originalEnv)
			} else {
				os.Unsetenv("DECISION_MAX_CANARY_HOSTS")
			}
		}()

		decision, err := guardrails.DecideStrategy("enforce", 3, []string{"web"})

		require.NoError(t, err)
		assert.Equal(t, model.StrategyModeSuggest, decision.Strategy, "Should downgrade enforce to suggest when max canary is 0")
		assert.Equal(t, 0, decision.CanarySize, "Canary size should be 0")
		assert.Contains(t, decision.Reasons, "Canary size is 0 in enforce/canary mode", "Should mention canary size reason")
		assert.Contains(t, decision.AppliedRules, "canary_size_zero", "Should apply canary_size_zero rule")
	})
}

func TestGuardrails_CombinedRules(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	guardrails := NewGuardrails(logger)

	t.Run("enforce_with_never_block_and_zero_canary_goes_to_suggest", func(t *testing.T) {
		// Set both never block labels and zero max canary
		originalNeverBlock := os.Getenv("DECISION_NEVER_BLOCK_LABELS")
		originalMaxCanary := os.Getenv("DECISION_MAX_CANARY_HOSTS")
		
		os.Setenv("DECISION_NEVER_BLOCK_LABELS", "production,critical")
		os.Setenv("DECISION_MAX_CANARY_HOSTS", "0")
		
		defer func() {
			if originalNeverBlock != "" {
				os.Setenv("DECISION_NEVER_BLOCK_LABELS", originalNeverBlock)
			} else {
				os.Unsetenv("DECISION_NEVER_BLOCK_LABELS")
			}
			if originalMaxCanary != "" {
				os.Setenv("DECISION_MAX_CANARY_HOSTS", originalMaxCanary)
			} else {
				os.Unsetenv("DECISION_MAX_CANARY_HOSTS")
			}
		}()

		decision, err := guardrails.DecideStrategy("enforce", 3, []string{"production", "web"})

		require.NoError(t, err)
		assert.Equal(t, model.StrategyModeSuggest, decision.Strategy, "Should downgrade enforce to suggest with both never block and zero canary")
		assert.Equal(t, 0, decision.CanarySize, "Canary size should be 0")
		
		// Should have both rules applied
		assert.Contains(t, decision.AppliedRules, "never_block_labels", "Should apply never_block_labels rule")
		assert.Contains(t, decision.AppliedRules, "canary_size_zero", "Should apply canary_size_zero rule")
		
		// Should have reasons for both
		neverBlockReason := false
		canarySizeReason := false
		for _, reason := range decision.Reasons {
			if contains(reason, "NEVER_BLOCK_LABELS") {
				neverBlockReason = true
			}
			if contains(reason, "Canary size is 0") {
				canarySizeReason = true
			}
		}
		assert.True(t, neverBlockReason, "Should have never block reason")
		assert.True(t, canarySizeReason, "Should have canary size reason")
	})
}

func TestGuardrails_EdgeCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	guardrails := NewGuardrails(logger)

	tests := []struct {
		name        string
		desired     string
		numTargets  int
		hostLabels  []string
		envVars     map[string]string
		expected    model.StrategyMode
		expectedTTL int
	}{
		{
			name:       "suggest_mode_unchanged_with_never_block",
			desired:    "suggest",
			numTargets: 2,
			hostLabels: []string{"production"},
			envVars: map[string]string{
				"DECISION_NEVER_BLOCK_LABELS": "production",
			},
			expected:    model.StrategyModeSuggest,
			expectedTTL: 3600,
		},
		{
			name:       "observe_mode_unchanged_with_never_block",
			desired:    "observe",
			numTargets: 1,
			hostLabels: []string{"critical"},
			envVars: map[string]string{
				"DECISION_NEVER_BLOCK_LABELS": "critical",
			},
			expected:    model.StrategyModeObserve,
			expectedTTL: 3600,
		},
		{
			name:       "custom_ttl_preserved",
			desired:    "suggest",
			numTargets: 1,
			hostLabels: []string{"web"},
			envVars: map[string]string{
				"DECISION_DEFAULT_TTL_SECONDS": "7200",
			},
			expected:    model.StrategyModeSuggest,
			expectedTTL: 7200,
		},
		{
			name:       "zero_targets_handled",
			desired:    "canary",
			numTargets: 0,
			hostLabels: []string{"web"},
			envVars: map[string]string{
				"DECISION_MAX_CANARY_HOSTS": "5",
			},
			expected:    model.StrategyModeCanary,
			expectedTTL: 3600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original environment
			originalEnv := make(map[string]string)
			for key, value := range tt.envVars {
				originalEnv[key] = os.Getenv(key)
				os.Setenv(key, value)
			}

			// Restore environment after test
			defer func() {
				for key, originalValue := range originalEnv {
					if originalValue != "" {
						os.Setenv(key, originalValue)
					} else {
						os.Unsetenv(key)
					}
				}
			}()

			decision, err := guardrails.DecideStrategy(tt.desired, tt.numTargets, tt.hostLabels)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, decision.Strategy, "Strategy should match expected")
			assert.Equal(t, tt.expectedTTL, decision.TTLSeconds, "TTL should match expected")
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr || 
			 containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
