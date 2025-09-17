package model

import (
	"time"
)

// StrategyMode defines the type of strategy to use
type StrategyMode string

const (
	StrategyModeObserve      StrategyMode = "observe"
	StrategyModeSuggest      StrategyMode = "suggest"
	StrategyModeCanary       StrategyMode = "canary"
	StrategyModeEnforce      StrategyMode = "enforce"
	StrategyModeConservative StrategyMode = "conservative"
	StrategyModeBalanced     StrategyMode = "balanced"
	StrategyModeAggressive   StrategyMode = "aggressive"
)

// PlanStatus defines the current status of a plan
type PlanStatus string

const (
	PlanStatusPending   PlanStatus = "pending"
	PlanStatusProposed  PlanStatus = "proposed"
	PlanStatusActive    PlanStatus = "active"
	PlanStatusCompleted PlanStatus = "completed"
	PlanStatusFailed    PlanStatus = "failed"
	PlanStatusCancelled PlanStatus = "cancelled"
)

// SuccessCriteria defines what constitutes success for a plan
type SuccessCriteria struct {
	// Minimum success rate (0.0 to 1.0)
	MinSuccessRate float64 `json:"min_success_rate"`
	// Maximum acceptable failure rate (0.0 to 1.0)
	MaxFailureRate float64 `json:"max_failure_rate"`
	// Timeout in seconds
	TimeoutSeconds int `json:"timeout_seconds"`
	// Required metrics to track
	RequiredMetrics []string `json:"required_metrics"`
}

// Rollback defines rollback strategy
type Rollback struct {
	// Whether rollback is enabled
	Enabled bool `json:"enabled"`
	// Rollback triggers
	Triggers []string `json:"triggers"`
	// Rollback timeout in seconds
	TimeoutSeconds int `json:"timeout_seconds"`
	// Rollback actions
	Actions []string `json:"actions"`
}

// Control defines control mechanisms
type Control struct {
	// Control ID
	ID string `json:"id"`
	// Control type
	Type string `json:"type"`
	// Control description
	Description string `json:"description"`
	// Whether manual approval is required
	ManualApproval bool `json:"manual_approval"`
	// Control gates
	Gates []string `json:"gates"`
	// Escalation procedures
	Escalation []string `json:"escalation"`
}

// Strategy defines the overall strategy for a plan
type Strategy struct {
	// Strategy mode
	Mode StrategyMode `json:"mode"`
	// Success criteria
	SuccessCriteria SuccessCriteria `json:"success_criteria"`
	// Rollback configuration
	Rollback Rollback `json:"rollback"`
	// Control mechanisms
	Control Control `json:"control"`
	// Strategy description
	Description string `json:"description"`
}

// Plan represents a complete decision plan
type Plan struct {
	// Unique plan ID
	ID string `json:"id"`
	// Finding ID that triggered this plan
	FindingID string `json:"finding_id"`
	// Plan name
	Name string `json:"name"`
	// Plan description
	Description string `json:"description"`
	// Current status
	Status PlanStatus `json:"status"`
	// Strategy configuration
	Strategy Strategy `json:"strategy"`
	// Plan steps/actions
	Steps []string `json:"steps"`
	// Plan metadata
	Metadata map[string]interface{} `json:"metadata"`
	// Creation timestamp
	CreatedAt time.Time `json:"created_at"`
	// Last updated timestamp
	UpdatedAt time.Time `json:"updated_at"`
	// Expiration timestamp
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// Finding details
	Finding Finding `json:"finding"`
	// Target hosts/services
	Targets []string `json:"targets"`
	// Control mechanisms
	Controls []Control `json:"controls"`
	// TTL in seconds
	TTLSeconds int `json:"ttl_seconds"`
	// Plan notes/explanation
	Notes string `json:"notes"`
}

// CreatePlanRequest represents a request to create a new plan
type CreatePlanRequest struct {
	// Finding ID to base the plan on
	FindingID *string `json:"finding_id,omitempty"`
	// Inline finding data (alternative to finding_id)
	Finding *map[string]interface{} `json:"finding,omitempty"`
	// Override strategy mode
	StrategyMode *StrategyMode `json:"strategy_mode,omitempty"`
	// Additional metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CreatePlanResponse represents the response to a plan creation request
type CreatePlanResponse struct {
	// Created plan
	Plan Plan `json:"plan"`
	// Success message
	Message string `json:"message"`
}

// Finding represents a finding from the correlator
type Finding struct {
	ID          string                 `json:"id"`
	Severity    string                 `json:"severity"`
	Confidence  float64                `json:"confidence"`
	Status      string                 `json:"status"`
	HostID      string                 `json:"host_id"`
	CVE         string                 `json:"cve,omitempty"`
	Evidence    []string               `json:"evidence"`
	Timestamp   time.Time              `json:"ts"`
	RuleID      string                 `json:"rule_id"`
	TTLSeconds  int                    `json:"ttl_seconds"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
