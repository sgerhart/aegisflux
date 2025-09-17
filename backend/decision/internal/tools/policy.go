package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"time"
)

// PolicyConfig contains configuration for the policy tool
type PolicyConfig struct {
	CompilerEndpoint string
	APIKey           string
	Timeout          time.Duration
	MockMode         bool // For testing without real policy compiler
}

// PolicyTool provides interface to policy compilation services
type PolicyTool struct {
	config PolicyConfig
	logger *slog.Logger
}

// NewPolicyTool creates a new policy tool instance
func NewPolicyTool(config PolicyConfig, logger *slog.Logger) *PolicyTool {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &PolicyTool{
		config: config,
		logger: logger,
	}
}

// PolicyIntent represents the intent for policy compilation
type PolicyIntent struct {
	Action      string                 `json:"action"`
	Target      string                 `json:"target"`
	Conditions  map[string]any         `json:"conditions"`
	Parameters  map[string]any         `json:"parameters"`
	Priority    int                    `json:"priority"`
	Expiration  string                 `json:"expiration,omitempty"`
	Metadata    map[string]any         `json:"metadata,omitempty"`
}

// Compile compiles a policy intent into executable artifacts
func (p *PolicyTool) Compile(intent map[string]any) (artifacts []map[string]any, error error) {
	p.logger.Debug("Compiling policy intent", "intent", intent)

	if p.config.MockMode {
		return p.mockCompile(intent), nil
	}

	// TODO: Implement real policy compilation API integration
	// This would typically involve:
	// 1. Sending intent to policy compiler service
	// 2. Generating nftables rules for network policies
	// 3. Generating Cilium policies for service mesh
	// 4. Creating Kubernetes NetworkPolicies
	// 5. Generating firewall rules for various platforms
	
	return p.mockCompile(intent), nil
}

// mockCompile provides mock policy compilation results
func (p *PolicyTool) mockCompile(intent map[string]any) []map[string]any {
	rand.Seed(time.Now().UnixNano())
	
	var artifacts []map[string]any
	
	// Extract action from intent
	action, _ := intent["action"].(string)
	if action == "" {
		action = "block"
	}
	
	// Generate nftables artifact
	nftArtifact := p.generateNftablesPolicy(intent, action)
	artifacts = append(artifacts, nftArtifact)
	
	// Generate Cilium artifact
	ciliumArtifact := p.generateCiliumPolicy(intent, action)
	artifacts = append(artifacts, ciliumArtifact)
	
	// Generate Kubernetes NetworkPolicy artifact
	k8sArtifact := p.generateK8sNetworkPolicy(intent, action)
	artifacts = append(artifacts, k8sArtifact)
	
	p.logger.Info("Policy compilation completed", 
		"action", action, 
		"artifacts_generated", len(artifacts))
	
	return artifacts
}

// generateNftablesPolicy creates a mock nftables policy
func (p *PolicyTool) generateNftablesPolicy(intent map[string]any, action string) map[string]any {
	target, _ := intent["target"].(string)
	if target == "" {
		target = "10.0.0.0/8"
	}
	
	nftRules := fmt.Sprintf(`
# Generated nftables policy - %s
table inet security {
    chain input {
        type filter hook input priority 0;
        policy accept;
        
        # Block traffic from %s
        ip saddr %s %s
        counter drop
    }
    
    chain forward {
        type filter hook forward priority 0;
        policy accept;
        
        # Block forwarded traffic from %s
        ip saddr %s %s
        counter drop
    }
}`, 
		action,
		target,
		target,
		map[string]string{"block": "drop", "allow": "accept", "log": "log"}[action],
		target,
		target,
		map[string]string{"block": "drop", "allow": "accept", "log": "log"}[action])
	
	return map[string]any{
		"type": "nftables",
		"name": fmt.Sprintf("security-policy-%d", rand.Intn(10000)),
		"rules": nftRules,
		"metadata": map[string]any{
			"generated_at": time.Now().Format("2006-01-02T15:04:05Z"),
			"action": action,
			"target": target,
			"compiler_version": "1.0.0",
		},
	}
}

// generateCiliumPolicy creates a mock Cilium policy
func (p *PolicyTool) generateCiliumPolicy(intent map[string]any, action string) map[string]any {
	target, _ := intent["target"].(string)
	if target == "" {
		target = "malicious-pod"
	}
	
	ciliumPolicy := map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind": "CiliumNetworkPolicy",
		"metadata": map[string]any{
			"name": fmt.Sprintf("security-policy-%d", rand.Intn(10000)),
			"namespace": "default",
		},
		"spec": map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"app": "protected-service",
				},
			},
			"ingress": []map[string]any{
				{
					"fromEndpoints": []map[string]any{
						{
							"matchLabels": map[string]any{
								"name": "trusted-service",
							},
						},
					},
				},
			},
			"egress": []map[string]any{
				{
					"toEndpoints": []map[string]any{
						{
							"matchLabels": map[string]any{
								"name": "trusted-destination",
							},
						},
					},
				},
			},
		},
	}
	
	if action == "block" {
		// Add deny rules for target
		ciliumPolicy["spec"].(map[string]any)["deny"] = []map[string]any{
			{
				"fromEndpoints": []map[string]any{
					{
						"matchLabels": map[string]any{
							"name": target,
						},
					},
				},
			},
		}
	}
	
	policyJSON, _ := json.MarshalIndent(ciliumPolicy, "", "  ")
	
	return map[string]any{
		"type": "cilium",
		"name": fmt.Sprintf("cilium-policy-%d", rand.Intn(10000)),
		"policy": string(policyJSON),
		"metadata": map[string]any{
			"generated_at": time.Now().Format("2006-01-02T15:04:05Z"),
			"action": action,
			"target": target,
			"compiler_version": "1.0.0",
		},
	}
}

// generateK8sNetworkPolicy creates a mock Kubernetes NetworkPolicy
func (p *PolicyTool) generateK8sNetworkPolicy(intent map[string]any, action string) map[string]any {
	target, _ := intent["target"].(string)
	if target == "" {
		target = "malicious-namespace"
	}
	
	k8sPolicy := map[string]any{
		"apiVersion": "networking.k8s.io/v1",
		"kind": "NetworkPolicy",
		"metadata": map[string]any{
			"name": fmt.Sprintf("security-netpol-%d", rand.Intn(10000)),
			"namespace": "default",
		},
		"spec": map[string]any{
			"podSelector": map[string]any{
				"matchLabels": map[string]any{
					"app": "protected-app",
				},
			},
			"policyTypes": []string{"Ingress", "Egress"},
			"ingress": []map[string]any{
				{
					"from": []map[string]any{
						{
							"podSelector": map[string]any{
								"matchLabels": map[string]any{
									"app": "trusted-app",
								},
							},
						},
					},
				},
			},
			"egress": []map[string]any{
				{
					"to": []map[string]any{
						{
							"podSelector": map[string]any{
								"matchLabels": map[string]any{
									"app": "trusted-destination",
								},
							},
						},
					},
				},
			},
		},
	}
	
	if action == "block" {
		// Add deny rules for target
		k8sPolicy["spec"].(map[string]any)["ingressDeny"] = []map[string]any{
			{
				"from": []map[string]any{
					{
						"namespaceSelector": map[string]any{
							"matchLabels": map[string]any{
								"name": target,
							},
						},
					},
				},
			},
		}
	}
	
	policyJSON, _ := json.MarshalIndent(k8sPolicy, "", "  ")
	
	return map[string]any{
		"type": "kubernetes",
		"name": fmt.Sprintf("k8s-netpol-%d", rand.Intn(10000)),
		"policy": string(policyJSON),
		"metadata": map[string]any{
			"generated_at": time.Now().Format("2006-01-02T15:04:05Z"),
			"action": action,
			"target": target,
			"compiler_version": "1.0.0",
		},
	}
}

// ValidatePolicy validates a policy before compilation
func (p *PolicyTool) ValidatePolicy(intent map[string]any) (bool, []string, error) {
	var errors []string
	
	// Check required fields
	if _, ok := intent["action"]; !ok {
		errors = append(errors, "action field is required")
	}
	
	if _, ok := intent["target"]; !ok {
		errors = append(errors, "target field is required")
	}
	
	// Validate action values
	if action, ok := intent["action"].(string); ok {
		validActions := []string{"block", "allow", "log", "redirect"}
		valid := false
		for _, validAction := range validActions {
			if action == validAction {
				valid = true
				break
			}
		}
		if !valid {
			errors = append(errors, fmt.Sprintf("invalid action '%s', must be one of: %v", action, validActions))
		}
	}
	
	// Validate priority if provided
	if priority, ok := intent["priority"].(float64); ok {
		if priority < 0 || priority > 1000 {
			errors = append(errors, "priority must be between 0 and 1000")
		}
	}
	
	return len(errors) == 0, errors, nil
}

// GetSupportedActions returns the list of supported policy actions
func (p *PolicyTool) GetSupportedActions() []string {
	return []string{"block", "allow", "log", "redirect", "rate_limit", "inspect"}
}

// GetSupportedTargets returns the list of supported policy targets
func (p *PolicyTool) GetSupportedTargets() []string {
	return []string{"ip_address", "ip_range", "hostname", "namespace", "pod", "service", "port", "protocol"}
}
