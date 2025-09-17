package agents

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"aegisflux/backend/decision/internal/model"
)

// PolicyWriterAgent handles policy generation from control intents
type PolicyWriterAgent struct {
	runtime *Runtime
	logger  *slog.Logger
}

// NewPolicyWriterAgent creates a new policy writer agent
func NewPolicyWriterAgent(runtime *Runtime, logger *slog.Logger) *PolicyWriterAgent {
	return &PolicyWriterAgent{
		runtime: runtime,
		logger:  logger,
	}
}

// PolicyControl represents a policy control with artifacts
type PolicyControl struct {
	// Control mode (always "simulate" for policy writer)
	Mode string `json:"mode"`
	// Control scope (pid, cgroup, host)
	Scope string `json:"scope"`
	// Scope identifier (actual pid, cgroup path, host ID)
	ScopeID string `json:"scope_id"`
	// TTL in seconds
	TTLSeconds int `json:"ttl_seconds"`
	// Policy artifacts (nftables, Cilium, etc.)
	Artifacts []PolicyArtifact `json:"artifacts"`
	// Control description
	Description string `json:"description"`
	// Original intent that generated this control
	OriginalIntent map[string]any `json:"original_intent"`
	// Creation timestamp
	CreatedAt time.Time `json:"created_at"`
}

// PolicyArtifact represents a policy artifact
type PolicyArtifact struct {
	// Artifact type (nftables, cilium, ebpf, etc.)
	Type string `json:"type"`
	// Artifact content
	Content string `json:"content"`
	// Artifact metadata
	Metadata map[string]any `json:"metadata"`
	// Preview flag
	IsPreview bool `json:"is_preview"`
}

// GenerateControls generates policy controls from control intents
func (p *PolicyWriterAgent) GenerateControls(ctx context.Context, controlIntents []map[string]any) ([]PolicyControl, error) {
	p.logger.Info("Generating policy controls", "intent_count", len(controlIntents))

	var controls []PolicyControl

	for i, intent := range controlIntents {
		p.logger.Debug("Processing control intent", "index", i, "intent", intent)

		// Generate control for this intent
		control, err := p.generateControlFromIntent(ctx, intent)
		if err != nil {
			p.logger.Error("Failed to generate control from intent", "intent", intent, "error", err)
			continue
		}

		controls = append(controls, *control)
	}

	p.logger.Info("Policy controls generated successfully", "control_count", len(controls))
	return controls, nil
}

// generateControlFromIntent generates a single control from an intent
func (p *PolicyWriterAgent) generateControlFromIntent(ctx context.Context, intent map[string]any) (*PolicyControl, error) {
	// Extract basic intent information
	action, _ := intent["action"].(string)
	target, _ := intent["target"].(string)
	reason, _ := intent["reason"].(string)
	ttlSeconds := p.extractTTL(intent)
	intentType, _ := intent["type"].(string)

	// Determine scope and scope ID
	scope, scopeID := p.determineScope(intent)

	// Create base control
	control := &PolicyControl{
		Mode:           "simulate",
		Scope:          scope,
		ScopeID:        scopeID,
		TTLSeconds:     ttlSeconds,
		OriginalIntent: intent,
		CreatedAt:      time.Now(),
		Description:    fmt.Sprintf("Policy control for %s on %s: %s", action, target, reason),
	}

	// Generate artifacts based on intent type
	var artifacts []PolicyArtifact

	// Always call policy.compile for standard intents
	if intentType != "ebpf_mitigate" {
		policyArtifacts, err := p.compilePolicy(ctx, intent)
		if err != nil {
			p.logger.Warn("Failed to compile policy, using fallback", "error", err)
			policyArtifacts = p.generateFallbackPolicy(intent)
		}
		artifacts = append(artifacts, policyArtifacts...)
	}

	// Call ebpf.template_suggest for eBPF mitigation intents
	if intentType == "ebpf_mitigate" {
		ebpfArtifacts, err := p.suggestEBPFTemplate(ctx, intent)
		if err != nil {
			p.logger.Warn("Failed to suggest eBPF template, using fallback", "error", err)
			ebpfArtifacts = p.generateFallbackEBPF(intent)
		}
		artifacts = append(artifacts, ebpfArtifacts...)
	}

	// If no artifacts were generated, create a fallback
	if len(artifacts) == 0 {
		artifacts = p.generateFallbackPolicy(intent)
	}

	control.Artifacts = artifacts

	return control, nil
}

// extractTTL extracts TTL from intent or uses default
func (p *PolicyWriterAgent) extractTTL(intent map[string]any) int {
	// Check if TTL is specified in intent
	if ttl, ok := intent["ttl_seconds"].(int); ok {
		return ttl
	}
	if ttl, ok := intent["ttl_seconds"].(float64); ok {
		return int(ttl)
	}

	// Use environment variable default
	defaultTTL := p.getDefaultTTL()
	return defaultTTL
}

// getDefaultTTL gets the default TTL from environment variable
func (p *PolicyWriterAgent) getDefaultTTL() int {
	if ttlStr := os.Getenv("DECISION_DEFAULT_TTL_SECONDS"); ttlStr != "" {
		if ttl, err := strconv.Atoi(ttlStr); err == nil {
			return ttl
		}
	}
	// Default to 1 hour if not specified
	return 3600
}

// determineScope determines the scope and scope ID from intent
func (p *PolicyWriterAgent) determineScope(intent map[string]any) (scope, scopeID string) {
	// Check for explicit scope information
	if scopeVal, ok := intent["scope"].(string); ok {
		scope = scopeVal
	}
	if scopeIDVal, ok := intent["scope_id"].(string); ok {
		scopeID = scopeIDVal
		return scope, scopeID
	}

	// Try to infer scope from other fields
	if pid, ok := intent["pid"].(string); ok {
		scope = "pid"
		scopeID = pid
		return scope, scopeID
	}
	if pid, ok := intent["pid"].(int); ok {
		scope = "pid"
		scopeID = fmt.Sprintf("%d", pid)
		return scope, scopeID
	}

	if cgroup, ok := intent["cgroup"].(string); ok {
		scope = "cgroup"
		scopeID = cgroup
		return scope, scopeID
	}

	if hostID, ok := intent["host_id"].(string); ok {
		scope = "host"
		scopeID = hostID
		return scope, scopeID
	}

	// Default to host scope with unknown ID
	scope = "host"
	scopeID = "unknown"
	return scope, scopeID
}

// compilePolicy calls the policy.compile tool
func (p *PolicyWriterAgent) compilePolicy(ctx context.Context, intent map[string]any) ([]PolicyArtifact, error) {
	p.logger.Debug("Compiling policy", "intent", intent)

	// Execute policy compilation using agent runtime
	result, err := p.runtime.ExecuteAgent(ctx, "policywriter", 
		fmt.Sprintf("Compile policy for intent: %v", intent), 
		map[string]any{"intent": intent})
	if err != nil {
		return nil, fmt.Errorf("failed to execute policy compilation: %w", err)
	}

	// Extract policy artifacts from tool results
	var artifacts []PolicyArtifact
	for _, toolResult := range result.ToolResults {
		if toolResult.Function == "policy.compile" && toolResult.Error == nil {
			if result, ok := toolResult.Result["artifacts"].([]interface{}); ok {
				for _, artifact := range result {
					if artifactMap, ok := artifact.(map[string]interface{}); ok {
						artifacts = append(artifacts, PolicyArtifact{
							Type:      artifactMap["type"].(string),
							Content:   artifactMap["rules"].(string),
							Metadata:  artifactMap["metadata"].(map[string]any),
							IsPreview: true,
						})
					}
				}
			}
		}
	}

	return artifacts, nil
}

// suggestEBPFTemplate calls the ebpf.template_suggest tool
func (p *PolicyWriterAgent) suggestEBPFTemplate(ctx context.Context, intent map[string]any) ([]PolicyArtifact, error) {
	p.logger.Debug("Suggesting eBPF template", "intent", intent)

	// Extract template hint from intent
	templateHint := "ebpf_mitigate"
	if hint, ok := intent["template_hint"].(string); ok {
		templateHint = hint
	}

	// Execute eBPF template suggestion using agent runtime
	result, err := p.runtime.ExecuteAgent(ctx, "policywriter", 
		fmt.Sprintf("Suggest eBPF template for: %s", templateHint), 
		map[string]any{"intent": intent, "template_hint": templateHint})
	if err != nil {
		return nil, fmt.Errorf("failed to execute eBPF template suggestion: %w", err)
	}

	// Extract eBPF artifacts from tool results
	var artifacts []PolicyArtifact
	for _, toolResult := range result.ToolResults {
		if toolResult.Function == "ebpf.template_suggest" && toolResult.Error == nil {
			if template, ok := toolResult.Result["template"].(map[string]interface{}); ok {
				artifacts = append(artifacts, PolicyArtifact{
					Type:      "ebpf",
					Content:   template["template"].(string),
					Metadata:  template["metadata"].(map[string]any),
					IsPreview: true,
				})
			}
		}
	}

	return artifacts, nil
}

// generateFallbackPolicy generates fallback policy artifacts
func (p *PolicyWriterAgent) generateFallbackPolicy(intent map[string]any) []PolicyArtifact {
	action, _ := intent["action"].(string)
	target, _ := intent["target"].(string)

	// Generate nftables fallback
	nftablesContent := fmt.Sprintf(`
# Fallback nftables policy
table inet security {
    chain input {
        type filter hook input priority 0;
        policy accept;
        
        # %s traffic from %s
        ip saddr %s counter %s
    }
}`, action, target, target, action)

	// Generate Cilium fallback
	ciliumContent := fmt.Sprintf(`
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: fallback-policy
spec:
  endpointSelector: {}
  egress:
  - toEntities: ["world"]
    # %s %s traffic
`, action, target)

	return []PolicyArtifact{
		{
			Type:      "nftables",
			Content:   nftablesContent,
			Metadata:  map[string]any{"fallback": true, "action": action, "target": target},
			IsPreview: true,
		},
		{
			Type:      "cilium",
			Content:   ciliumContent,
			Metadata:  map[string]any{"fallback": true, "action": action, "target": target},
			IsPreview: true,
		},
	}
}

// generateFallbackEBPF generates fallback eBPF artifacts
func (p *PolicyWriterAgent) generateFallbackEBPF(intent map[string]any) []PolicyArtifact {
	action, _ := intent["action"].(string)
	target, _ := intent["target"].(string)

	ebpfContent := fmt.Sprintf(`
#include <uapi/linux/bpf.h>
#include <linux/net.h>

SEC("kprobe/tcp_connect")
int fallback_ebpf_probe(struct pt_regs *ctx) {
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    struct sockaddr_in *addr = (struct sockaddr_in *)PT_REGS_PARM2(ctx);
    
    u32 dst_ip = addr->sin_addr.s_addr;
    u16 dst_port = addr->sin_port;
    
    // %s connections to %s
    if (dst_ip == %s) {
        bpf_trace_printk("Fallback eBPF: %s connection to %s\\n", 
                         action, target);
        // Add %s logic here
    }
    
    return 0;
}`, action, target, target, action, target, action)

	return []PolicyArtifact{
		{
			Type:      "ebpf",
			Content:   ebpfContent,
			Metadata:  map[string]any{"fallback": true, "action": action, "target": target},
			IsPreview: true,
		},
	}
}

// ConvertToModelControls converts PolicyControl to model.Control
func (p *PolicyWriterAgent) ConvertToModelControls(policyControls []PolicyControl) []model.Control {
	var modelControls []model.Control

	for _, pc := range policyControls {
		// Convert artifacts to gates
		var gates []string
		for _, artifact := range pc.Artifacts {
			gates = append(gates, fmt.Sprintf("%s:%s", artifact.Type, artifact.Content[:min(50, len(artifact.Content))]))
		}

		// Convert to model.Control
		modelControl := model.Control{
			ManualApproval: false, // Policy writer generates simulated controls
			Gates:          gates,
			Escalation:     []string{"manual_review", "rollback"},
		}

		modelControls = append(modelControls, modelControl)
	}

	return modelControls
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
