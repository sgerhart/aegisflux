package agents

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"aegisflux/backend/decision/internal/model"
)

// PlannerAgent handles plan drafting using LLM agents
type PlannerAgent struct {
	runtime *Runtime
	logger  *slog.Logger
}

// PlanDraft represents a draft plan structure
type PlanDraft struct {
	Targets        []string            `json:"targets"`
	ControlIntents []map[string]any    `json:"control_intents"`
	DesiredMode    model.StrategyMode  `json:"desired_mode"`
	SuccessCriteria map[string]any     `json:"success_criteria"`
	Notes          string              `json:"notes"`
}


// NewPlannerAgent creates a new planner agent
func NewPlannerAgent(runtime *Runtime, logger *slog.Logger) *PlannerAgent {
	return &PlannerAgent{
		runtime: runtime,
		logger:  logger,
	}
}

// BuildPlanDraft creates a plan draft from a security finding
func (p *PlannerAgent) BuildPlanDraft(ctx context.Context, finding map[string]any) (*PlanDraft, error) {
	p.logger.Info("Building plan draft", "finding_id", finding["id"], "severity", finding["severity"])

	// Check if LLM fallback mode is enabled
	if p.shouldUseFallback() {
		p.logger.Info("Using LLM fallback implementation due to configuration or errors")
		fallbackDraft := p.createEnhancedFallbackDraft(finding)
		
		// Validate and apply fallbacks
		draft := p.validateAndFallback(fallbackDraft, finding)

		p.logger.Info("Plan draft created successfully", 
			"targets", len(draft.Targets),
			"control_intents", len(draft.ControlIntents),
			"desired_mode", draft.DesiredMode)

		return &draft, nil
	}

	// If we reach here, LLM is available but we're using fallback for MVP
	p.logger.Info("Using fallback planner implementation for MVP")
	fallbackDraft := p.createFallbackDraft(finding)
	
	// Validate and apply fallbacks
	draft := p.validateAndFallback(fallbackDraft, finding)

	p.logger.Info("Plan draft created successfully", 
		"targets", len(draft.Targets),
		"control_intents", len(draft.ControlIntents),
		"desired_mode", draft.DesiredMode)

	return &draft, nil
}

// createFallbackDraft creates a fallback draft if LLM fails
func (p *PlannerAgent) createFallbackDraft(finding map[string]any) PlanDraft {
	// Extract host_id as target
	targets := []string{}
	if hostID, ok := finding["host_id"].(string); ok {
		targets = append(targets, hostID)
	}
	
	// Create basic control intent based on finding
	controlIntents := []map[string]any{}
	if severity, ok := finding["severity"].(string); ok {
		var action string
		switch severity {
		case "critical":
			action = "block"
		case "high":
			action = "suggest"
		default:
			action = "observe"
		}
		
		controlIntents = append(controlIntents, map[string]any{
			"action": action,
			"target": "finding_host",
			"reason": fmt.Sprintf("Security finding: %s", severity),
			"ttl_seconds": 3600,
		})
	}
	
	// Determine mode based on severity
	var desiredMode model.StrategyMode
	if severity, ok := finding["severity"].(string); ok {
		switch severity {
		case "critical":
			desiredMode = model.StrategyModeEnforce
		case "high":
			desiredMode = model.StrategyModeCanary
		case "medium":
			desiredMode = model.StrategyModeSuggest
		default:
			desiredMode = model.StrategyModeObserve
		}
	} else {
		desiredMode = model.StrategyModeSuggest
	}
	
	// Create success criteria
	successCriteria := map[string]any{
		"min_success_rate": 0.95,
		"timeout_seconds": 300,
	}
	
	// Generate notes
	notes := fmt.Sprintf("Fallback plan for finding: %v", finding["id"])
	if ruleID, ok := finding["rule_id"].(string); ok {
		notes += fmt.Sprintf(" (Rule: %s)", ruleID)
	}
	
	return PlanDraft{
		Targets:        targets,
		ControlIntents: controlIntents,
		DesiredMode:    desiredMode,
		SuccessCriteria: successCriteria,
		Notes:          notes,
	}
}

// validateAndFallback validates the draft and applies fallbacks
func (p *PlannerAgent) validateAndFallback(draft PlanDraft, finding map[string]any) PlanDraft {
	// Ensure targets are not empty
	if len(draft.Targets) == 0 {
		if hostID, ok := finding["host_id"].(string); ok {
			draft.Targets = []string{hostID}
		} else {
			draft.Targets = []string{"unknown-host"}
		}
		p.logger.Warn("Empty targets, using fallback", "fallback", draft.Targets)
	}
	
	// Ensure control intents are not empty
	if len(draft.ControlIntents) == 0 {
		draft.ControlIntents = []map[string]any{
			{
				"action": "observe",
				"target": "finding_host",
				"reason": "Default fallback control",
				"ttl_seconds": 3600,
			},
		}
		p.logger.Warn("Empty control intents, using fallback")
	}
	
	// Limit control intents to reasonable number
	if len(draft.ControlIntents) > 3 {
		draft.ControlIntents = draft.ControlIntents[:3]
		p.logger.Warn("Too many control intents, limiting to 3")
	}
	
	// Validate desired mode
	validModes := []model.StrategyMode{
		model.StrategyModeObserve, model.StrategyModeSuggest, model.StrategyModeCanary, 
		model.StrategyModeEnforce, model.StrategyModeConservative, model.StrategyModeBalanced, model.StrategyModeAggressive,
	}
	
	validMode := false
	for _, mode := range validModes {
		if draft.DesiredMode == mode {
			validMode = true
			break
		}
	}
	
	if !validMode {
		draft.DesiredMode = model.StrategyModeSuggest
		p.logger.Warn("Invalid desired mode, using fallback", "fallback", draft.DesiredMode)
	}
	
	// Ensure success criteria has required fields
	if draft.SuccessCriteria == nil {
		draft.SuccessCriteria = map[string]any{
			"min_success_rate": 0.95,
			"timeout_seconds": 300,
		}
		p.logger.Warn("Empty success criteria, using fallback")
	}
	
	return draft
}

// shouldUseFallback determines if LLM fallback mode should be used
func (p *PlannerAgent) shouldUseFallback() bool {
	// Check environment variable for LLM fallback mode
	if fallbackMode := os.Getenv("LLM_FALLBACK_MODE"); fallbackMode == "true" {
		p.logger.Info("LLM fallback mode enabled via environment variable")
		return true
	}
	
	// Check if LLM router is available (has clients)
	if p.runtime == nil || p.runtime.router == nil {
		p.logger.Info("Runtime or router not available, using fallback")
		return true
	}
	
	client, err := p.runtime.router.ClientFor("planner")
	if err != nil || client == nil {
		p.logger.Info("LLM client not available, using fallback", "error", err)
		return true
	}
	
	return false
}

// createEnhancedFallbackDraft creates an enhanced fallback draft with minimal controls
func (p *PlannerAgent) createEnhancedFallbackDraft(finding map[string]any) PlanDraft {
	// Extract host_id as target
	targets := []string{}
	if hostID, ok := finding["host_id"].(string); ok {
		targets = append(targets, hostID)
	}
	
	// Create minimal control intent - check for connect evidence
	controlIntents := []map[string]any{}
	
	// Check if there's connect evidence
	hasConnectEvidence := false
	if evidence, ok := finding["evidence"].([]string); ok {
		for _, ev := range evidence {
			if contains(ev, "connect") || contains(ev, "connection") || contains(ev, "network") {
				hasConnectEvidence = true
				break
			}
		}
	}
	
	// If connect evidence exists, create minimal nft_drop control
	if hasConnectEvidence {
		controlIntents = append(controlIntents, map[string]any{
			"action":      "suggest",
			"target":      "finding_host",
			"reason":      "Minimal network control for connect evidence",
			"ttl_seconds": 3600,
			"type":        "nft_drop",
		})
	}
	
	// Always use suggest strategy for fallback
	desiredMode := model.StrategyModeSuggest
	
	// Create success criteria
	successCriteria := map[string]any{
		"min_success_rate": 0.95,
		"timeout_seconds": 300,
	}
	
	// Generate minimal notes
	notes := fmt.Sprintf("Fallback plan for finding: %v", finding["id"])
	if hasConnectEvidence {
		notes += " (Connect evidence detected - minimal nft_drop control applied)"
	}
	
	return PlanDraft{
		Targets:        targets,
		ControlIntents: controlIntents,
		DesiredMode:    desiredMode,
		SuccessCriteria: successCriteria,
		Notes:          notes,
	}
}

// contains checks if a string contains a substring (case insensitive)
func contains(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return len(s) >= len(substr) && 
		(len(s) == len(substr) && s == substr || 
		 strings.Contains(s, substr))
}
