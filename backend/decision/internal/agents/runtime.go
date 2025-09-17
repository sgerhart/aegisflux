package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aegisflux/backend/common/agents"
)

// Runtime provides the agent runtime with tool access control and function calling
type Runtime struct {
	router       *agents.Router
	budget       *agents.BudgetManager
	logger       *slog.Logger
	toolRegistry map[string]ToolFunction
	allowlists   map[string][]string
}

// ToolFunction represents a tool function that can be called
type ToolFunction interface {
	Call(args map[string]any) (map[string]any, error)
}

// ToolCall represents a single tool call
type ToolCall struct {
	Function string         `json:"function"`
	Args     map[string]any `json:"args"`
}

// LLMStep represents a step in the LLM conversation
type LLMStep struct {
	Input       string                 `json:"input"`
	Tools       []string               `json:"tools"`
	Constraints map[string]any         `json:"constraints,omitempty"`
}

// LLMResponse represents the LLM's response with function calls
type LLMResponse struct {
	Calls []ToolCall `json:"calls"`
	Draft *string    `json:"draft,omitempty"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Function string         `json:"function"`
	Result   map[string]any `json:"result,omitempty"`
	Error    *string        `json:"error,omitempty"`
}

// ExecutionResult represents the final result of agent execution
type ExecutionResult struct {
	FinalDraft *string      `json:"final_draft,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
	Iterations int          `json:"iterations"`
	Cost       float64      `json:"estimated_cost"`
	Error      *string      `json:"error,omitempty"`
}

// NewRuntime creates a new agent runtime
func NewRuntime(logger *slog.Logger) (*Runtime, error) {
	// Initialize router
	router, err := agents.NewRouter(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize router: %w", err)
	}

	// Initialize budget
	budget, err := agents.NewBudgetManager(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize budget: %w", err)
	}

	runtime := &Runtime{
		router:       router,
		budget:       budget,
		logger:       logger,
		toolRegistry: make(map[string]ToolFunction),
		allowlists:   make(map[string][]string),
	}

	// Initialize tool allowlists
	runtime.initializeAllowlists()

	// Initialize tool registry
	runtime.initializeToolRegistry()

	return runtime, nil
}

// initializeAllowlists sets up role-based tool allowlists
func (r *Runtime) initializeAllowlists() {
	r.allowlists = map[string][]string{
		"planner": {
			"graph.query",
			"cve.lookup", 
			"risk.context",
			"policy.compile",
			"ebpf.template_suggest",
		},
		"policywriter": {
			"policy.compile",
			"ebpf.template_suggest", 
			"registry.sign_store",
		},
		"segmenter": {
			"graph.query",
			"risk.context",
		},
		"explainer": {
			// No tools - uses given context only
		},
	}

	r.logger.Debug("Initialized tool allowlists", "roles", len(r.allowlists))
}

// initializeToolRegistry sets up the tool registry with mock implementations
func (r *Runtime) initializeToolRegistry() {
	// Note: In a real implementation, these would be initialized with actual tool instances
	// For now, we'll create mock tool functions that can be called

	r.toolRegistry = map[string]ToolFunction{
		"graph.query":           &MockToolFunction{name: "graph.query"},
		"cve.lookup":            &MockToolFunction{name: "cve.lookup"},
		"risk.context":          &MockToolFunction{name: "risk.context"},
		"policy.compile":        &MockToolFunction{name: "policy.compile"},
		"ebpf.template_suggest": &MockToolFunction{name: "ebpf.template_suggest"},
		"registry.sign_store":   &MockToolFunction{name: "registry.sign_store"},
	}

	r.logger.Debug("Initialized tool registry", "tools", len(r.toolRegistry))
}

// ExecuteAgent runs an agent with the given role and input
func (r *Runtime) ExecuteAgent(ctx context.Context, role string, input string, constraints map[string]any) (*ExecutionResult, error) {
	r.logger.Info("Executing agent", "role", role, "input_length", len(input))

	// Check if role is valid
	allowedTools, exists := r.allowlists[role]
	if !exists {
		return nil, fmt.Errorf("unknown agent role: %s", role)
	}

	// Check budget before starting (estimate with default values)
	if err := r.budget.CheckBudget("default", "default", 100, 100); err != nil {
		return nil, fmt.Errorf("budget exceeded: %w", err)
	}

	// Get LLM client for this role
	client, err := r.router.ClientFor(role)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for role %s: %w", role, err)
	}

	// Prepare initial step
	step := LLMStep{
		Input:       input,
		Tools:       allowedTools,
		Constraints: constraints,
	}

	// Execute conversation loop (max 2 iterations)
	var finalDraft *string
	var toolResults []ToolResult
	totalCost := 0.0

	for iteration := 0; iteration < 2; iteration++ {
		r.logger.Debug("Agent iteration", "role", role, "iteration", iteration+1)

		// Send step to LLM
		response, cost, err := r.sendToLLM(ctx, client, step, role)
		if err != nil {
			return nil, fmt.Errorf("LLM communication failed: %w", err)
		}
		totalCost += cost

		// Check budget after LLM call (estimate with actual token usage)
		if err := r.budget.CheckBudget(client.GetProvider(), "default", int(cost*1000), int(cost*1000)); err != nil {
			return nil, fmt.Errorf("budget exceeded after LLM call: %w", err)
		}

		// Store final draft if provided
		if response.Draft != nil {
			finalDraft = response.Draft
		}

		// Execute tool calls if any
		if len(response.Calls) > 0 {
			iterationResults, err := r.executeToolCalls(ctx, role, response.Calls)
			if err != nil {
				r.logger.Error("Tool execution failed", "error", err)
				// Continue with partial results
			}
			toolResults = append(toolResults, iterationResults...)

			// Prepare next step with tool results
			step = LLMStep{
				Input: r.formatToolResultsForLLM(toolResults),
				Tools: allowedTools,
				Constraints: constraints,
			}
		} else {
			// No more tool calls, we're done
			break
		}
	}

	result := &ExecutionResult{
		FinalDraft:  finalDraft,
		ToolResults: toolResults,
		Iterations:  len(toolResults) + 1, // +1 for initial LLM call
		Cost:        totalCost,
	}

	r.logger.Info("Agent execution completed", 
		"role", role, 
		"iterations", result.Iterations,
		"cost", totalCost,
		"tools_used", len(toolResults))

	return result, nil
}

// sendToLLM sends a step to the LLM and parses the response
func (r *Runtime) sendToLLM(ctx context.Context, client agents.LLMClient, step LLMStep, role string) (*LLMResponse, float64, error) {
	// Format the input for the LLM
	prompt := r.formatStepForLLM(step, role)

	// Make the LLM call
	response, err := client.Generate(prompt)
	if err != nil {
		return nil, 0, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse the response
	var llmResponse LLMResponse
	if err := json.Unmarshal([]byte(response), &llmResponse); err != nil {
		return nil, 0, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Estimate cost (simplified - in real implementation, track actual tokens)
	estimatedCost := float64(len(prompt)+len(response)) / 1000.0 * 0.002 // $0.002 per 1K tokens

	return &llmResponse, estimatedCost, nil
}

// formatStepForLLM formats a step for LLM input
func (r *Runtime) formatStepForLLM(step LLMStep, role string) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Role: %s\n", role))
	builder.WriteString(fmt.Sprintf("Input: %s\n", step.Input))

	if len(step.Tools) > 0 {
		builder.WriteString("Available tools: ")
		builder.WriteString(strings.Join(step.Tools, ", "))
		builder.WriteString("\n")
	}

	if len(step.Constraints) > 0 {
		constraintsJSON, _ := json.Marshal(step.Constraints)
		builder.WriteString(fmt.Sprintf("Constraints: %s\n", string(constraintsJSON)))
	}

	builder.WriteString("\nRespond with JSON in this format:\n")
	builder.WriteString(`{
  "calls": [
    {"function": "tool_name", "args": {"param1": "value1"}}
  ],
  "draft": "optional draft response"
}`)

	return builder.String()
}

// executeToolCalls executes a list of tool calls
func (r *Runtime) executeToolCalls(ctx context.Context, role string, calls []ToolCall) ([]ToolResult, error) {
	var results []ToolResult

	for _, call := range calls {
		r.logger.Debug("Executing tool call", "role", role, "function", call.Function)

		// Check if tool is allowed for this role
		allowedTools := r.allowlists[role]
		if !r.isToolAllowed(call.Function, allowedTools) {
			errorMsg := fmt.Sprintf("tool '%s' not allowed for role '%s'", call.Function, role)
			results = append(results, ToolResult{
				Function: call.Function,
				Error:    &errorMsg,
			})
			continue
		}

		// Get tool function
		tool, exists := r.toolRegistry[call.Function]
		if !exists {
			errorMsg := fmt.Sprintf("tool '%s' not found", call.Function)
			results = append(results, ToolResult{
				Function: call.Function,
				Error:    &errorMsg,
			})
			continue
		}

		// Execute tool call
		result, err := tool.Call(call.Args)
		if err != nil {
			errorMsg := err.Error()
			results = append(results, ToolResult{
				Function: call.Function,
				Error:    &errorMsg,
			})
			continue
		}

		results = append(results, ToolResult{
			Function: call.Function,
			Result:   result,
		})

		r.logger.Debug("Tool call completed", "function", call.Function, "success", true)
	}

	return results, nil
}

// isToolAllowed checks if a tool is allowed for a role
func (r *Runtime) isToolAllowed(toolName string, allowedTools []string) bool {
	for _, allowed := range allowedTools {
		if allowed == toolName {
			return true
		}
	}
	return false
}

// formatToolResultsForLLM formats tool results for LLM input
func (r *Runtime) formatToolResultsForLLM(results []ToolResult) string {
	var builder strings.Builder

	builder.WriteString("Tool execution results:\n")
	for _, result := range results {
		builder.WriteString(fmt.Sprintf("Tool: %s\n", result.Function))
		if result.Error != nil {
			builder.WriteString(fmt.Sprintf("Error: %s\n", *result.Error))
		} else {
			resultJSON, _ := json.Marshal(result.Result)
			builder.WriteString(fmt.Sprintf("Result: %s\n", string(resultJSON)))
		}
		builder.WriteString("\n")
	}

	return builder.String()
}

// GetAvailableRoles returns the list of available agent roles
func (r *Runtime) GetAvailableRoles() []string {
	var roles []string
	for role := range r.allowlists {
		roles = append(roles, role)
	}
	return roles
}

// GetRoleTools returns the tools allowed for a specific role
func (r *Runtime) GetRoleTools(role string) []string {
	if tools, exists := r.allowlists[role]; exists {
		return tools
	}
	return []string{}
}

// GetBudgetStats returns current budget statistics
func (r *Runtime) GetBudgetStats() map[string]any {
	return r.budget.GetStats()
}

// MockToolFunction provides a mock implementation of ToolFunction
type MockToolFunction struct {
	name string
}

// Call implements the ToolFunction interface with mock behavior
func (m *MockToolFunction) Call(args map[string]any) (map[string]any, error) {
	// Simulate some processing time
	time.Sleep(10 * time.Millisecond)

	// Return mock result based on tool name
	switch m.name {
	case "graph.query":
		return map[string]any{
			"results": []map[string]any{
				{"host_id": "web-01", "connections": 5},
				{"host_id": "db-01", "connections": 12},
			},
			"query_time_ms": 25,
		}, nil

	case "cve.lookup":
		return map[string]any{
			"vulnerabilities": []map[string]any{
				{"cve_id": "CVE-2024-1234", "severity": "HIGH", "score": 7.5},
			},
			"packages_checked": len(args),
		}, nil

	case "risk.context":
		return map[string]any{
			"risk_score": 6.5,
			"five_xx_rate": 0.03,
			"recent_findings": 8,
		}, nil

	case "policy.compile":
		return map[string]any{
			"artifacts": []map[string]any{
				{"type": "nftables", "rules": "table inet security { chain input { ... } }"},
				{"type": "cilium", "policy": "apiVersion: cilium.io/v2\nkind: CiliumNetworkPolicy"},
			},
		}, nil

	case "ebpf.template_suggest":
		return map[string]any{
			"template": map[string]any{
				"name": "network_monitor",
				"code": "#include <uapi/linux/bpf.h>\nSEC(\"kprobe/tcp_connect\")...",
			},
			"params": map[string]any{
				"buffer_size": 1024,
				"sample_rate": 1,
			},
		}, nil

	case "registry.sign_store":
		return map[string]any{
			"artifact_id": "artifact-12345-abcdef",
			"signature": "sha256:1234567890abcdef...",
			"stored_at": time.Now().Format(time.RFC3339),
		}, nil

	default:
		return map[string]any{
			"message": fmt.Sprintf("Mock result for tool: %s", m.name),
			"args_received": args,
		}, nil
	}
}