package agents

import (
	"log/slog"
	"os"
	"testing"

	"aegisflux/backend/decision/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlannerAgent_createEnhancedFallbackDraft(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	planner := &PlannerAgent{
		runtime: nil, // We don't need runtime for this test
		logger:  logger,
	}

	tests := []struct {
		name                 string
		finding              map[string]any
		expectedTargets      []string
		expectedStrategy     model.StrategyMode
		hasControlIntents    bool
		hasConnectEvidence   bool
		expectedControlType  string
	}{
		{
			name: "connect_to_exec_finding_produces_control_intent",
			finding: map[string]any{
				"id":        "finding-connect-exec",
				"severity":  "high",
				"host_id":   "web-01",
				"rule_id":   "connect-to-exec",
				"evidence":  []string{"Network connection followed by process execution"},
				"confidence": 0.8,
			},
			expectedTargets:     []string{"web-01"},
			expectedStrategy:    model.StrategyModeSuggest,
			hasControlIntents:   true,
			hasConnectEvidence:  true,
			expectedControlType: "nft_drop",
		},
		{
			name: "connect_evidence_creates_minimal_nft_drop",
			finding: map[string]any{
				"id":        "finding-network",
				"severity":  "medium",
				"host_id":   "api-01",
				"rule_id":   "suspicious-network",
				"evidence":  []string{"Suspicious network activity detected", "Connection established"},
				"confidence": 0.7,
			},
			expectedTargets:     []string{"api-01"},
			expectedStrategy:    model.StrategyModeSuggest,
			hasControlIntents:   true,
			hasConnectEvidence:  true,
			expectedControlType: "nft_drop",
		},
		{
			name: "no_connect_evidence_no_controls",
			finding: map[string]any{
				"id":        "finding-file-access",
				"severity":  "low",
				"host_id":   "db-01",
				"rule_id":   "file-access",
				"evidence":  []string{"Unusual file access pattern", "Permission changes detected"},
				"confidence": 0.5,
			},
			expectedTargets:    []string{"db-01"},
			expectedStrategy:   model.StrategyModeSuggest,
			hasControlIntents:  false,
			hasConnectEvidence: false,
		},
		{
			name: "finding_without_host_id_fallback",
			finding: map[string]any{
				"id":        "finding-unknown",
				"severity":  "high",
				"rule_id":   "unknown-attack",
				"evidence":  []string{"Unknown attack pattern detected"},
				"confidence": 0.8,
			},
			expectedTargets:    []string{},
			expectedStrategy:   model.StrategyModeSuggest,
			hasControlIntents:  false,
			hasConnectEvidence: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			draft := planner.createEnhancedFallbackDraft(tt.finding)

			// Verify targets
			assert.Equal(t, tt.expectedTargets, draft.Targets, "Targets should match expected")

			// Verify strategy is always suggest for fallback
			assert.Equal(t, tt.expectedStrategy, draft.DesiredMode, "Strategy should be suggest for fallback")

			// Verify control intents based on connect evidence
			if tt.hasControlIntents {
				require.Greater(t, len(draft.ControlIntents), 0, "Should have control intents when connect evidence exists")
				
				controlIntent := draft.ControlIntents[0]
				assert.Equal(t, "suggest", controlIntent["action"], "Action should be suggest")
				assert.Equal(t, "finding_host", controlIntent["target"], "Target should be finding_host")
				assert.Equal(t, 3600, controlIntent["ttl_seconds"], "TTL should be 3600 seconds")
				
				if tt.hasConnectEvidence {
					assert.Equal(t, tt.expectedControlType, controlIntent["type"], "Type should be nft_drop for connect evidence")
					assert.Contains(t, controlIntent["reason"], "Minimal network control", "Reason should mention minimal network control")
				}
			} else {
				assert.Len(t, draft.ControlIntents, 0, "Should have no control intents when no connect evidence")
			}

			// Verify success criteria
			assert.NotNil(t, draft.SuccessCriteria, "Should have success criteria")
			assert.Equal(t, 0.95, draft.SuccessCriteria["min_success_rate"], "Min success rate should be 0.95")
			assert.Equal(t, 300, draft.SuccessCriteria["timeout_seconds"], "Timeout should be 300 seconds")

			// Verify notes
			assert.Contains(t, draft.Notes, "Fallback plan for finding:", "Notes should indicate fallback")
			assert.Contains(t, draft.Notes, tt.finding["id"], "Notes should contain finding ID")
			
			if tt.hasConnectEvidence {
				assert.Contains(t, draft.Notes, "Connect evidence detected", "Notes should mention connect evidence")
			}
		})
	}
}

func TestPlannerAgent_shouldUseFallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	planner := &PlannerAgent{
		runtime: nil,
		logger:  logger,
	}

	t.Run("fallback_mode_environment_variable", func(t *testing.T) {
		// Test with LLM_FALLBACK_MODE=true
		originalEnv := os.Getenv("LLM_FALLBACK_MODE")
		os.Setenv("LLM_FALLBACK_MODE", "true")
		defer func() {
			if originalEnv != "" {
				os.Setenv("LLM_FALLBACK_MODE", originalEnv)
			} else {
				os.Unsetenv("LLM_FALLBACK_MODE")
			}
		}()

		result := planner.shouldUseFallback()
		assert.True(t, result, "Should use fallback when LLM_FALLBACK_MODE=true")
	})

	t.Run("fallback_mode_disabled", func(t *testing.T) {
		// Test with LLM_FALLBACK_MODE=false or unset
		originalEnv := os.Getenv("LLM_FALLBACK_MODE")
		os.Setenv("LLM_FALLBACK_MODE", "false")
		defer func() {
			if originalEnv != "" {
				os.Setenv("LLM_FALLBACK_MODE", originalEnv)
			} else {
				os.Unsetenv("LLM_FALLBACK_MODE")
			}
		}()

		// Since runtime is nil, this should return true (no client available)
		result := planner.shouldUseFallback()
		assert.True(t, result, "Should use fallback when runtime is not available")
	})
}

func TestConnectEvidenceDetection(t *testing.T) {
	tests := []struct {
		name     string
		evidence []string
		expected bool
	}{
		{
			name:     "connect_keyword",
			evidence: []string{"Network connection established"},
			expected: true,
		},
		{
			name:     "connection_keyword",
			evidence: []string{"TCP connection detected"},
			expected: true,
		},
		{
			name:     "network_keyword", 
			evidence: []string{"Network traffic anomaly"},
			expected: true,
		},
		{
			name:     "multiple_evidence_with_connect",
			evidence: []string{"File access", "Network connection", "Process execution"},
			expected: true,
		},
		{
			name:     "no_connect_evidence",
			evidence: []string{"File access pattern", "Memory corruption", "Process execution"},
			expected: false,
		},
		{
			name:     "empty_evidence",
			evidence: []string{},
			expected: false,
		},
		{
			name:     "case_insensitive_connect",
			evidence: []string{"CONNECTION established"},
			expected: true,
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	planner := &PlannerAgent{
		runtime: nil,
		logger:  logger,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finding := map[string]any{
				"id":       "test-finding",
				"host_id":  "test-host",
				"evidence": tt.evidence,
			}

			draft := planner.createEnhancedFallbackDraft(finding)

			hasControls := len(draft.ControlIntents) > 0
			assert.Equal(t, tt.expected, hasControls, "Connect evidence detection should match expected")

			if tt.expected {
				// Verify the control intent has nft_drop type
				require.Len(t, draft.ControlIntents, 1, "Should have exactly one control intent")
				controlIntent := draft.ControlIntents[0]
				assert.Equal(t, "nft_drop", controlIntent["type"], "Should create nft_drop control for connect evidence")
			}
		})
	}
}
