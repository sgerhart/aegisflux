package rules

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

func TestLoader_LoadSnapshot(t *testing.T) {
	// Create temporary directory for test rules
	tempDir := t.TempDir()
	
	// Create test rule files
	rule1Content := `
apiVersion: aegisflux/v1
kind: CorrelationRule
metadata:
  id: "test-rule-1"
  name: "Test Rule 1"
  version: "1.0.0"
spec:
  enabled: true
  selectors:
    host_ids: ["host-001"]
  condition:
    window_seconds: 5
  outcome:
    severity: "high"
    confidence: 0.8
  ttl_seconds: 3600
`

	rule2Content := `
apiVersion: aegisflux/v1
kind: CorrelationRule
metadata:
  id: "test-rule-2"
  name: "Test Rule 2"
  version: "1.0.0"
spec:
  enabled: false
  selectors:
    host_ids: ["host-002"]
  condition:
    window_seconds: 10
  outcome:
    severity: "medium"
    confidence: 0.6
  ttl_seconds: 1800
`

	// Write test files
	err := os.WriteFile(filepath.Join(tempDir, "01-rule-1.yaml"), []byte(rule1Content), 0644)
	require.NoError(t, err)
	
	err = os.WriteFile(filepath.Join(tempDir, "02-rule-2.yaml"), []byte(rule2Content), 0644)
	require.NoError(t, err)
	
	// Create loader
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	loader := NewLoader(tempDir, false, 1000, logger)
	
	// Load snapshot
	snapshot, err := loader.LoadSnapshot()
	require.NoError(t, err)
	
	// Verify results
	assert.Len(t, snapshot.Rules, 1) // Only enabled rule should be loaded
	assert.Equal(t, "test-rule-1", snapshot.Rules[0].Metadata.ID)
	assert.True(t, snapshot.Rules[0].IsEnabled())
	assert.Equal(t, "high", snapshot.Rules[0].Spec.Outcome.Severity)
	assert.Equal(t, 0.8, snapshot.Rules[0].Spec.Outcome.Confidence)
}

func TestLoader_SkipDisabled(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create disabled rule
	disabledRuleContent := `
apiVersion: aegisflux/v1
kind: CorrelationRule
metadata:
  id: "disabled-rule"
  name: "Disabled Rule"
  version: "1.0.0"
spec:
  enabled: false
  selectors:
    host_ids: []
  condition:
    window_seconds: 5
  outcome:
    severity: "high"
    confidence: 0.8
  ttl_seconds: 3600
`
	
	err := os.WriteFile(filepath.Join(tempDir, "disabled.yaml"), []byte(disabledRuleContent), 0644)
	require.NoError(t, err)
	
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	loader := NewLoader(tempDir, false, 1000, logger)
	
	snapshot, err := loader.LoadSnapshot()
	require.NoError(t, err)
	
	// Disabled rule should be skipped
	assert.Len(t, snapshot.Rules, 0)
}

func TestLoader_FilenameOverride(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create two rules with same ID but different content
	rule1Content := `
apiVersion: aegisflux/v1
kind: CorrelationRule
metadata:
  id: "duplicate-rule"
  name: "First Rule"
  version: "1.0.0"
spec:
  enabled: true
  selectors:
    host_ids: []
  condition:
    window_seconds: 5
  outcome:
    severity: "low"
    confidence: 0.5
  ttl_seconds: 3600
`

	rule2Content := `
apiVersion: aegisflux/v1
kind: CorrelationRule
metadata:
  id: "duplicate-rule"
  name: "Second Rule (Override)"
  version: "2.0.0"
spec:
  enabled: true
  selectors:
    host_ids: []
  condition:
    window_seconds: 10
  outcome:
    severity: "high"
    confidence: 0.9
  ttl_seconds: 7200
`
	
	// Write files with different names (alphabetical order matters)
	err := os.WriteFile(filepath.Join(tempDir, "01-first.yaml"), []byte(rule1Content), 0644)
	require.NoError(t, err)
	
	err = os.WriteFile(filepath.Join(tempDir, "02-second.yaml"), []byte(rule2Content), 0644)
	require.NoError(t, err)
	
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	loader := NewLoader(tempDir, false, 1000, logger)
	
	snapshot, err := loader.LoadSnapshot()
	require.NoError(t, err)
	
	// Should have only one rule (the second one should override the first)
	assert.Len(t, snapshot.Rules, 1)
	assert.Equal(t, "duplicate-rule", snapshot.Rules[0].Metadata.ID)
	assert.Equal(t, "Second Rule (Override)", snapshot.Rules[0].Metadata.Name)
	assert.Equal(t, "2.0.0", snapshot.Rules[0].Metadata.Version)
	assert.Equal(t, "high", snapshot.Rules[0].Spec.Outcome.Severity)
	assert.Equal(t, 0.9, snapshot.Rules[0].Spec.Outcome.Confidence)
	assert.Equal(t, filepath.Join(tempDir, "02-second.yaml"), snapshot.Rules[0].SourceFile)
}

func TestRule_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rule    Rule
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid rule",
			rule: Rule{
				Metadata: RuleMetadata{
					ID:      "valid-rule",
					Name:    "Valid Rule",
					Version: "1.0.0",
				},
				Spec: RuleSpec{
					Enabled: true,
					Outcome: Outcome{
						Severity:   "high",
						Confidence: 0.8,
					},
					TTLSeconds: 3600,
				},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			rule: Rule{
				Metadata: RuleMetadata{
					Name:    "No ID Rule",
					Version: "1.0.0",
				},
				Spec: RuleSpec{
					Enabled: true,
					Outcome: Outcome{
						Severity:   "high",
						Confidence: 0.8,
					},
					TTLSeconds: 3600,
				},
			},
			wantErr: true,
			errMsg:  "metadata.id: rule ID is required",
		},
		{
			name: "missing name",
			rule: Rule{
				Metadata: RuleMetadata{
					ID:      "no-name-rule",
					Version: "1.0.0",
				},
				Spec: RuleSpec{
					Enabled: true,
					Outcome: Outcome{
						Severity:   "high",
						Confidence: 0.8,
					},
					TTLSeconds: 3600,
				},
			},
			wantErr: true,
			errMsg:  "metadata.name: rule name is required",
		},
		{
			name: "invalid severity",
			rule: Rule{
				Metadata: RuleMetadata{
					ID:      "invalid-severity-rule",
					Name:    "Invalid Severity Rule",
					Version: "1.0.0",
				},
				Spec: RuleSpec{
					Enabled: true,
					Outcome: Outcome{
						Severity:   "invalid",
						Confidence: 0.8,
					},
					TTLSeconds: 3600,
				},
			},
			wantErr: true,
			errMsg:  "spec.outcome.severity: invalid severity, must be low/medium/high/critical",
		},
		{
			name: "invalid confidence",
			rule: Rule{
				Metadata: RuleMetadata{
					ID:      "invalid-confidence-rule",
					Name:    "Invalid Confidence Rule",
					Version: "1.0.0",
				},
				Spec: RuleSpec{
					Enabled: true,
					Outcome: Outcome{
						Severity:   "high",
						Confidence: 1.5, // Invalid: > 1.0
					},
					TTLSeconds: 3600,
				},
			},
			wantErr: true,
			errMsg:  "spec.outcome.confidence: confidence must be between 0.0 and 1.0",
		},
		{
			name: "invalid TTL",
			rule: Rule{
				Metadata: RuleMetadata{
					ID:      "invalid-ttl-rule",
					Name:    "Invalid TTL Rule",
					Version: "1.0.0",
				},
				Spec: RuleSpec{
					Enabled: true,
					Outcome: Outcome{
						Severity:   "high",
						Confidence: 0.8,
					},
					TTLSeconds: -1, // Invalid: negative
				},
			},
			wantErr: true,
			errMsg:  "spec.ttl_seconds: TTL must be positive",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoader_GetSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a test rule
	ruleContent := `
apiVersion: aegisflux/v1
kind: CorrelationRule
metadata:
  id: "test-rule"
  name: "Test Rule"
  version: "1.0.0"
spec:
  enabled: true
  selectors:
    host_ids: []
  condition:
    window_seconds: 5
  outcome:
    severity: "high"
    confidence: 0.8
  ttl_seconds: 3600
`
	
	err := os.WriteFile(filepath.Join(tempDir, "test.yaml"), []byte(ruleContent), 0644)
	require.NoError(t, err)
	
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	loader := NewLoader(tempDir, false, 1000, logger)
	
	// Load initial snapshot
	_, err = loader.LoadSnapshot()
	require.NoError(t, err)
	
	// Get snapshot
	snapshot := loader.GetSnapshot()
	assert.Len(t, snapshot.Rules, 1)
	assert.Equal(t, "test-rule", snapshot.Rules[0].Metadata.ID)
	assert.Greater(t, snapshot.Version, int64(0))
}

func TestLoader_Subscribe(t *testing.T) {
	tempDir := t.TempDir()
	
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	loader := NewLoader(tempDir, false, 1000, logger)
	
	// Subscribe to changes
	ch := loader.Subscribe()
	
	// Should receive initial notification
	select {
	case <-ch:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected initial notification")
	}
	
	// Create a rule and load it
	ruleContent := `
apiVersion: aegisflux/v1
kind: CorrelationRule
metadata:
  id: "test-rule"
  name: "Test Rule"
  version: "1.0.0"
spec:
  enabled: true
  selectors:
    host_ids: []
  condition:
    window_seconds: 5
  outcome:
    severity: "high"
    confidence: 0.8
  ttl_seconds: 3600
`
	
	err := os.WriteFile(filepath.Join(tempDir, "test.yaml"), []byte(ruleContent), 0644)
	require.NoError(t, err)
	
	// Load snapshot should trigger notification
	_, err = loader.LoadSnapshot()
	require.NoError(t, err)
	
	// Should receive notification
	select {
	case <-ch:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected notification after loading")
	}
}
