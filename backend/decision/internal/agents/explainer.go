package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// ExplainerAgent handles plan explanation and summarization
type ExplainerAgent struct {
	runtime *Runtime
	logger  *slog.Logger
}

// NewExplainerAgent creates a new explainer agent
func NewExplainerAgent(runtime *Runtime, logger *slog.Logger) *ExplainerAgent {
	return &ExplainerAgent{
		runtime: runtime,
		logger:  logger,
	}
}

// ExplainPlan generates a concise explanation of a plan for operators
func (e *ExplainerAgent) ExplainPlan(ctx context.Context, plan interface{}) (string, error) {
	e.logger.Info("Explaining plan for operators")

	// Convert plan to map for processing
	planMap, err := e.planToMap(plan)
	if err != nil {
		e.logger.Error("Failed to convert plan to map", "error", err)
		return "", fmt.Errorf("failed to convert plan to map: %w", err)
	}

	// Extract key information from plan
	summary := e.extractPlanSummary(planMap)
	
	// For now, use template explanation to avoid agent runtime issues
	// TODO: Implement proper LLM explanation when agent runtime is stable
	explanation := e.generateTemplateExplanation(summary)

	// Clean explanation to remove secrets and ensure proper formatting
	cleanExplanation := e.cleanExplanation(explanation)

	e.logger.Info("Plan explanation generated", "length", len(cleanExplanation))
	return cleanExplanation, nil
}

// planToMap converts a plan interface to a map for processing
func (e *ExplainerAgent) planToMap(plan interface{}) (map[string]interface{}, error) {
	// Try to convert to map
	if planMap, ok := plan.(map[string]interface{}); ok {
		return planMap, nil
	}

	// Try to convert from JSON if it's a byte slice
	if planBytes, ok := plan.([]byte); ok {
		var planMap map[string]interface{}
		if err := json.Unmarshal(planBytes, &planMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal plan JSON: %w", err)
		}
		return planMap, nil
	}

	// Try to convert from string
	if planStr, ok := plan.(string); ok {
		var planMap map[string]interface{}
		if err := json.Unmarshal([]byte(planStr), &planMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal plan string: %w", err)
		}
		return planMap, nil
	}

	return nil, fmt.Errorf("unsupported plan type: %T", plan)
}

// PlanSummary represents key information extracted from a plan
type PlanSummary struct {
	// Plan ID and basic info
	ID          string `json:"id"`
	Status      string `json:"status"`
	Strategy    string `json:"strategy"`
	
	// Targets and scope
	Targets     []string `json:"targets"`
	TargetCount int      `json:"target_count"`
	
	// Controls and actions
	ControlCount int      `json:"control_count"`
	Actions      []string `json:"actions"`
	
	// Risk and timing
	RiskLevel    string `json:"risk_level"`
	TTLSeconds   int    `json:"ttl_seconds"`
	
	// Success criteria
	SuccessCriteria []string `json:"success_criteria"`
	
	// Finding information
	FindingID    string `json:"finding_id"`
	FindingType  string `json:"finding_type"`
	Severity     string `json:"severity"`
}

// extractPlanSummary extracts key information from a plan map
func (e *ExplainerAgent) extractPlanSummary(planMap map[string]interface{}) *PlanSummary {
	summary := &PlanSummary{}

	// Extract basic plan information
	if id, ok := planMap["id"].(string); ok {
		summary.ID = id
	}
	if status, ok := planMap["status"].(string); ok {
		summary.Status = status
	}

	// Extract strategy information
	if strategy, ok := planMap["strategy"].(map[string]interface{}); ok {
		if mode, ok := strategy["mode"].(string); ok {
			summary.Strategy = mode
		}
		if successCriteria, ok := strategy["success_criteria"].(map[string]interface{}); ok {
			// Extract success criteria details
			if minSuccessRate, ok := successCriteria["min_success_rate"].(float64); ok {
				summary.SuccessCriteria = append(summary.SuccessCriteria, 
					fmt.Sprintf("Min success rate: %.1f%%", minSuccessRate*100))
			}
			if timeoutSeconds, ok := successCriteria["timeout_seconds"].(float64); ok {
				summary.SuccessCriteria = append(summary.SuccessCriteria, 
					fmt.Sprintf("Timeout: %.0fs", timeoutSeconds))
			}
		}
	}

	// Extract targets
	if targets, ok := planMap["targets"].([]interface{}); ok {
		for _, target := range targets {
			if targetStr, ok := target.(string); ok {
				summary.Targets = append(summary.Targets, targetStr)
			}
		}
		summary.TargetCount = len(summary.Targets)
	}

	// Extract controls
	if controls, ok := planMap["controls"].([]interface{}); ok {
		summary.ControlCount = len(controls)
		for _, control := range controls {
			if controlMap, ok := control.(map[string]interface{}); ok {
				// Extract control actions
				if gates, ok := controlMap["gates"].([]interface{}); ok {
					for _, gate := range gates {
						if gateStr, ok := gate.(string); ok {
							summary.Actions = append(summary.Actions, gateStr)
						}
					}
				}
			}
		}
	}

	// Extract finding information
	if finding, ok := planMap["finding"].(map[string]interface{}); ok {
		if findingID, ok := finding["id"].(string); ok {
			summary.FindingID = findingID
		}
		if findingType, ok := finding["type"].(string); ok {
			summary.FindingType = findingType
		}
		if severity, ok := finding["severity"].(string); ok {
			summary.Severity = severity
		}
	}

	// Extract TTL
	if ttlSeconds, ok := planMap["ttl_seconds"].(float64); ok {
		summary.TTLSeconds = int(ttlSeconds)
	}

	// Determine risk level based on strategy and controls
	summary.RiskLevel = e.determineRiskLevel(summary)

	return summary
}

// determineRiskLevel determines the risk level based on plan characteristics
func (e *ExplainerAgent) determineRiskLevel(summary *PlanSummary) string {
	// High risk indicators
	if summary.Strategy == "enforce" || summary.Strategy == "aggressive" {
		return "high"
	}
	if summary.Severity == "critical" || summary.Severity == "high" {
		return "high"
	}
	if summary.ControlCount > 5 {
		return "high"
	}
	if summary.TTLSeconds < 300 { // Less than 5 minutes
		return "high"
	}

	// Medium risk indicators
	if summary.Strategy == "canary" || summary.Strategy == "balanced" {
		return "medium"
	}
	if summary.Severity == "medium" {
		return "medium"
	}
	if summary.ControlCount > 2 {
		return "medium"
	}

	// Default to low risk
	return "low"
}

// generateExplanation generates an explanation using LLM
func (e *ExplainerAgent) generateExplanation(ctx context.Context, summary *PlanSummary) (string, error) {
	// Create prompt for explainer
	prompt := e.createExplainerPrompt(summary)

	// Execute using explainer role
	result, err := e.runtime.ExecuteAgent(ctx, "explainer", prompt, map[string]any{
		"summary": summary,
	})
	if err != nil {
		return "", fmt.Errorf("failed to execute explainer agent: %w", err)
	}

	// Extract explanation from result
	if result.FinalDraft != nil {
		return *result.FinalDraft, nil
	}

	return "", fmt.Errorf("no explanation generated")
}

// createExplainerPrompt creates a prompt for the explainer agent
func (e *ExplainerAgent) createExplainerPrompt(summary *PlanSummary) string {
	return fmt.Sprintf(`Explain this security plan in 3-6 concise bullet points for operators:

Plan ID: %s
Strategy: %s
Targets: %s (%d targets)
Controls: %d control mechanisms
Risk Level: %s
Finding: %s (%s severity)
TTL: %d seconds

Focus on:
- What the plan does
- Why it's needed
- Key actions being taken
- Risk considerations

Keep it concise and operator-focused. No technical jargon.`, 
		summary.ID,
		summary.Strategy,
		strings.Join(summary.Targets, ", "),
		summary.TargetCount,
		summary.ControlCount,
		summary.RiskLevel,
		summary.FindingType,
		summary.Severity,
		summary.TTLSeconds)
}

// generateTemplateExplanation generates a template-based explanation
func (e *ExplainerAgent) generateTemplateExplanation(summary *PlanSummary) string {
	var bullets []string

	// Bullet 1: What the plan does
	action := "monitoring"
	if summary.Strategy == "enforce" {
		action = "enforcing"
	} else if summary.Strategy == "canary" {
		action = "testing"
	} else if summary.Strategy == "suggest" {
		action = "suggesting"
	}

	bullets = append(bullets, fmt.Sprintf("• %s security controls on %d target(s) (%s)", 
		strings.Title(action), summary.TargetCount, strings.Join(summary.Targets, ", ")))

	// Bullet 2: Why it's needed
	if summary.FindingType != "" {
		bullets = append(bullets, fmt.Sprintf("• Responding to %s finding (%s severity)", 
			summary.FindingType, summary.Severity))
	} else {
		bullets = append(bullets, fmt.Sprintf("• Addressing security finding (ID: %s)", summary.FindingID))
	}

	// Bullet 3: Key actions
	if len(summary.Actions) > 0 {
		actionSummary := strings.Join(summary.Actions[:min(3, len(summary.Actions))], ", ")
		if len(summary.Actions) > 3 {
			actionSummary += "..."
		}
		bullets = append(bullets, fmt.Sprintf("• Key actions: %s", actionSummary))
	} else {
		bullets = append(bullets, fmt.Sprintf("• Implementing %d control mechanisms", summary.ControlCount))
	}

	// Bullet 4: Risk considerations
	bullets = append(bullets, fmt.Sprintf("• Risk level: %s", strings.Title(summary.RiskLevel)))

	// Bullet 5: Timing
	if summary.TTLSeconds > 0 {
		if summary.TTLSeconds < 3600 {
			bullets = append(bullets, fmt.Sprintf("• TTL: %d minutes", summary.TTLSeconds/60))
		} else {
			bullets = append(bullets, fmt.Sprintf("• TTL: %.1f hours", float64(summary.TTLSeconds)/3600))
		}
	}

	// Bullet 6: Success criteria
	if len(summary.SuccessCriteria) > 0 {
		criteria := strings.Join(summary.SuccessCriteria[:min(2, len(summary.SuccessCriteria))], ", ")
		bullets = append(bullets, fmt.Sprintf("• Success criteria: %s", criteria))
	}

	return strings.Join(bullets, "\n")
}

// cleanExplanation removes secrets and ensures proper formatting
func (e *ExplainerAgent) cleanExplanation(explanation string) string {
	// Remove common secret patterns
	secretPatterns := []string{
		`(?i)password[:\s]*[^\s]+`,
		`(?i)token[:\s]*[^\s]+`,
		`(?i)key[:\s]*[^\s]+`,
		`(?i)secret[:\s]*[^\s]+`,
		`(?i)api[_\s]*key[:\s]*[^\s]+`,
		`(?i)auth[_\s]*token[:\s]*[^\s]+`,
		`[a-zA-Z0-9]{32,}`, // Long alphanumeric strings (likely hashes/tokens)
	}

	for _, pattern := range secretPatterns {
		re := regexp.MustCompile(pattern)
		explanation = re.ReplaceAllString(explanation, "[REDACTED]")
	}

	// Clean up formatting
	explanation = strings.TrimSpace(explanation)
	
	// Ensure bullet points are properly formatted
	lines := strings.Split(explanation, "\n")
	var cleanLines []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Ensure bullet points start with •
		if !strings.HasPrefix(line, "•") && !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "*") {
			line = "• " + line
		} else if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
			line = "•" + line[1:]
		}
		
		cleanLines = append(cleanLines, line)
	}

	return strings.Join(cleanLines, "\n")
}

