package rules

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// RuleOverride represents an override for a rule
type RuleOverride struct {
	ID          string    `json:"id"`
	RuleID      string    `json:"rule_id"`
	Enabled     *bool     `json:"enabled,omitempty"`
	Severity    *string   `json:"severity,omitempty"`
	Confidence  *float64  `json:"confidence,omitempty"`
	TTLSeconds  *int      `json:"ttl_seconds,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Description string    `json:"description,omitempty"`
}

// RuleOverrideSummary represents a summary of a rule override
type RuleOverrideSummary struct {
	ID          string    `json:"id"`
	RuleID      string    `json:"rule_id"`
	Enabled     *bool     `json:"enabled,omitempty"`
	Severity    *string   `json:"severity,omitempty"`
	Confidence  *float64  `json:"confidence,omitempty"`
	TTLSeconds  *int      `json:"ttl_seconds,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description,omitempty"`
}

// RuleFileSummary represents a summary of a rule file
type RuleFileSummary struct {
	Filename     string `json:"filename"`
	RuleCount    int    `json:"rule_count"`
	EnabledCount int    `json:"enabled_count"`
	DisabledCount int   `json:"disabled_count"`
}

// RuleSummaryResponse represents the response for GET /rules
type RuleSummaryResponse struct {
	Files     []RuleFileSummary     `json:"files"`
	Overrides []RuleOverrideSummary `json:"overrides"`
}

// OverrideManager manages rule overrides in memory
type OverrideManager struct {
	mu        sync.RWMutex
	overrides map[string]*RuleOverride
	logger    *slog.Logger
	metrics   MetricsUpdater
}

// MetricsUpdater interface for updating metrics
type MetricsUpdater interface {
	SetRulesOverrides(count float64)
}

// NewOverrideManager creates a new override manager
func NewOverrideManager(logger *slog.Logger) *OverrideManager {
	return &OverrideManager{
		overrides: make(map[string]*RuleOverride),
		logger:    logger,
	}
}

// NewOverrideManagerWithMetrics creates a new override manager with metrics
func NewOverrideManagerWithMetrics(logger *slog.Logger, metrics MetricsUpdater) *OverrideManager {
	return &OverrideManager{
		overrides: make(map[string]*RuleOverride),
		logger:    logger,
		metrics:   metrics,
	}
}

// AddOverride adds a new rule override
func (om *OverrideManager) AddOverride(ruleID string, enabled *bool, severity *string, confidence *float64, ttlSeconds *int, description string) (*RuleOverride, error) {
	om.mu.Lock()
	defer om.mu.Unlock()

	// Validate inputs
	if ruleID == "" {
		return nil, fmt.Errorf("rule_id is required")
	}

	// Generate unique ID
	id := om.generateOverrideID(ruleID)

	now := time.Now()
	override := &RuleOverride{
		ID:          id,
		RuleID:      ruleID,
		Enabled:     enabled,
		Severity:    severity,
		Confidence:  confidence,
		TTLSeconds:  ttlSeconds,
		CreatedAt:   now,
		UpdatedAt:   now,
		Description: description,
	}

	om.overrides[id] = override

	om.logger.Info("Rule override added",
		"override_id", id,
		"rule_id", ruleID,
		"enabled", enabled,
		"severity", severity,
		"confidence", confidence,
		"ttl_seconds", ttlSeconds)

	// Update metrics
	if om.metrics != nil {
		om.metrics.SetRulesOverrides(float64(len(om.overrides)))
	}

	return override, nil
}

// RemoveOverride removes a rule override by ID
func (om *OverrideManager) RemoveOverride(id string) error {
	om.mu.Lock()
	defer om.mu.Unlock()

	if _, exists := om.overrides[id]; !exists {
		return fmt.Errorf("override not found: %s", id)
	}

	delete(om.overrides, id)

	om.logger.Info("Rule override removed", "override_id", id)
	
	// Update metrics
	if om.metrics != nil {
		om.metrics.SetRulesOverrides(float64(len(om.overrides)))
	}
	
	return nil
}

// GetOverride retrieves a rule override by ID
func (om *OverrideManager) GetOverride(id string) (*RuleOverride, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	override, exists := om.overrides[id]
	if !exists {
		return nil, fmt.Errorf("override not found: %s", id)
	}

	return override, nil
}

// ListOverrides returns all rule overrides
func (om *OverrideManager) ListOverrides() []RuleOverrideSummary {
	om.mu.RLock()
	defer om.mu.RUnlock()

	var summaries []RuleOverrideSummary
	for _, override := range om.overrides {
		summaries = append(summaries, RuleOverrideSummary{
			ID:          override.ID,
			RuleID:      override.RuleID,
			Enabled:     override.Enabled,
			Severity:    override.Severity,
			Confidence:  override.Confidence,
			TTLSeconds:  override.TTLSeconds,
			CreatedAt:   override.CreatedAt,
			Description: override.Description,
		})
	}

	return summaries
}

// GetOverridesForRule returns all overrides for a specific rule
func (om *OverrideManager) GetOverridesForRule(ruleID string) []*RuleOverride {
	om.mu.RLock()
	defer om.mu.RUnlock()

	var overrides []*RuleOverride
	for _, override := range om.overrides {
		if override.RuleID == ruleID {
			overrides = append(overrides, override)
		}
	}

	return overrides
}

// ApplyOverrides applies overrides to a rule
func (om *OverrideManager) ApplyOverrides(rule *Rule) *Rule {
	om.mu.RLock()
	defer om.mu.RUnlock()

	overrides := om.GetOverridesForRule(rule.Metadata.ID)
	if len(overrides) == 0 {
		return rule
	}

	// Create a copy of the rule
	modifiedRule := *rule

	// Apply the most recent override for each field
	// In a real system, you might want more sophisticated merge logic
	for _, override := range overrides {
		if override.Enabled != nil {
			modifiedRule.Spec.Enabled = *override.Enabled
		}
		if override.Severity != nil {
			modifiedRule.Spec.Outcome.Severity = *override.Severity
		}
		if override.Confidence != nil {
			modifiedRule.Spec.Outcome.Confidence = *override.Confidence
		}
		if override.TTLSeconds != nil {
			modifiedRule.Spec.TTLSeconds = *override.TTLSeconds
		}
	}

	return &modifiedRule
}

// generateOverrideID generates a unique ID for an override
func (om *OverrideManager) generateOverrideID(ruleID string) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("override-%s-%d", ruleID, timestamp)
}

// GetStats returns statistics about overrides
func (om *OverrideManager) GetStats() map[string]interface{} {
	om.mu.RLock()
	defer om.mu.RUnlock()

	enabledCount := 0
	disabledCount := 0

	for _, override := range om.overrides {
		if override.Enabled != nil {
			if *override.Enabled {
				enabledCount++
			} else {
				disabledCount++
			}
		}
	}

	return map[string]interface{}{
		"total_overrides": len(om.overrides),
		"enabled_count":   enabledCount,
		"disabled_count":  disabledCount,
	}
}

// ClearOverrides removes all overrides (for testing)
func (om *OverrideManager) ClearOverrides() {
	om.mu.Lock()
	defer om.mu.Unlock()

	om.overrides = make(map[string]*RuleOverride)
	om.logger.Info("All rule overrides cleared")
}

// AddOverrideFromJSON creates an override from JSON data
func (om *OverrideManager) AddOverrideFromJSON(data []byte) (*RuleOverride, error) {
	var request struct {
		RuleID      string   `json:"rule_id"`
		Enabled     *bool    `json:"enabled,omitempty"`
		Severity    *string  `json:"severity,omitempty"`
		Confidence  *float64 `json:"confidence,omitempty"`
		TTLSeconds  *int     `json:"ttl_seconds,omitempty"`
		Description string   `json:"description,omitempty"`
	}

	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("failed to parse override request: %w", err)
	}

	return om.AddOverride(
		request.RuleID,
		request.Enabled,
		request.Severity,
		request.Confidence,
		request.TTLSeconds,
		request.Description,
	)
}

// ValidateOverride validates an override request
func (om *OverrideManager) ValidateOverride(ruleID string, enabled *bool, severity *string, confidence *float64, ttlSeconds *int) error {
	if ruleID == "" {
		return fmt.Errorf("rule_id is required")
	}

	if severity != nil {
		validSeverities := []string{"low", "medium", "high", "critical"}
		valid := false
		for _, s := range validSeverities {
			if *severity == s {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid severity: %s (must be one of: low, medium, high, critical)", *severity)
		}
	}

	if confidence != nil {
		if *confidence < 0.0 || *confidence > 1.0 {
			return fmt.Errorf("confidence must be between 0.0 and 1.0, got: %f", *confidence)
		}
	}

	if ttlSeconds != nil {
		if *ttlSeconds < 0 {
			return fmt.Errorf("ttl_seconds must be non-negative, got: %d", *ttlSeconds)
		}
	}

	return nil
}
