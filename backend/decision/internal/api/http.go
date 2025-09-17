package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"

	"aegisflux/backend/decision/internal/agents"
	"aegisflux/backend/decision/internal/guardrails"
	"aegisflux/backend/decision/internal/model"
	"aegisflux/backend/decision/internal/store"
)

// HTTPAPI handles HTTP requests for the decision service
type HTTPAPI struct {
	store         store.PlanStore
	logger        *slog.Logger
	natsConn      *nats.Conn
	agentRuntime  *agents.Runtime
	planner       *agents.PlannerAgent
	policyWriter  *agents.PolicyWriterAgent
	segmenter     *agents.SegmenterAgent
	explainer     *agents.ExplainerAgent
	guardrails    *guardrails.Guardrails
}

// NewHTTPAPI creates a new HTTP API handler
func NewHTTPAPI(store store.PlanStore, logger *slog.Logger, natsConn *nats.Conn, agentRuntime *agents.Runtime, guardrailsInstance *guardrails.Guardrails) *HTTPAPI {
	planner := agents.NewPlannerAgent(agentRuntime, logger)
	policyWriter := agents.NewPolicyWriterAgent(agentRuntime, logger)
	segmenter := agents.NewSegmenterAgent(agentRuntime, logger)
	explainer := agents.NewExplainerAgent(agentRuntime, logger)
	
	return &HTTPAPI{
		store:        store,
		logger:       logger,
		natsConn:     natsConn,
		agentRuntime: agentRuntime,
		planner:      planner,
		policyWriter: policyWriter,
		segmenter:    segmenter,
		explainer:    explainer,
		guardrails:   guardrailsInstance,
	}
}

// SetupRoutes sets up all HTTP routes
func (api *HTTPAPI) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Plan endpoints
	mux.HandleFunc("POST /plans", api.handleCreatePlan)
	mux.HandleFunc("POST /plans/draft", api.handleCreatePlanDraft)
	mux.HandleFunc("POST /plans/policy", api.handleGeneratePolicy)
	mux.HandleFunc("POST /targets/segment", api.handleSegmentTargets)
	mux.HandleFunc("POST /plans/explain", api.handleExplainPlan)
	mux.HandleFunc("GET /plans", api.handleListPlans)
	mux.HandleFunc("GET /plans/", api.handleGetPlan)

	// Health endpoints
	mux.HandleFunc("GET /healthz", api.handleHealth)
	mux.HandleFunc("GET /readyz", api.handleReady)

	// Agent runtime endpoints
	mux.HandleFunc("GET /agents/roles", api.handleGetRoles)
	mux.HandleFunc("GET /agents/budget", api.handleGetBudget)

	// Guardrails endpoints
	mux.HandleFunc("POST /guardrails/strategy", api.handleDecideStrategy)
	mux.HandleFunc("GET /guardrails/status", api.handleGuardrailsStatus)

	return mux
}

// handleCreatePlan handles POST /plans requests with complete pipeline
func (api *HTTPAPI) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req model.CreatePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.FindingID == nil && req.Finding == nil {
		http.Error(w, "Either finding_id or finding must be provided", http.StatusBadRequest)
		return
	}

	// Generate unique plan ID
	planID := fmt.Sprintf("plan-%d", time.Now().Unix())

	// Extract finding data
	var finding map[string]interface{}
	if req.FindingID != nil {
		// TODO: Fetch finding by ID from correlator service
		// For now, create a mock finding
		finding = map[string]interface{}{
			"id":        *req.FindingID,
			"severity":  "high",
			"host_id":   "web-01",
			"rule_id":   "bash-exec-after-connect",
			"evidence":  []string{"Bash execution detected after network connection"},
			"confidence": 0.8,
			"context": map[string]interface{}{
				"labels": []string{"web", "frontend"},
			},
		}
	} else {
		finding = *req.Finding
	}

	// Step 1: Build plan draft using planner
	api.logger.Info("Building plan draft", "plan_id", planID, "finding_id", finding["id"])
	draft, err := api.planner.BuildPlanDraft(r.Context(), finding)
	if err != nil {
		api.logger.Error("Failed to build plan draft", "plan_id", planID, "error", err)
		http.Error(w, "Failed to build plan draft", http.StatusInternalServerError)
		return
	}

	// Step 2: Extract targets from draft or fallback to finding host_id
	targets := draft.Targets
	if len(targets) == 0 {
		if hostID, ok := finding["host_id"].(string); ok {
			targets = []string{hostID}
		} else {
			targets = []string{"unknown-host"}
		}
	}

	// Step 3: Generate controls from control intents using policy writer
	api.logger.Info("Generating controls", "plan_id", planID, "control_intents", len(draft.ControlIntents))
	controls, err := api.policyWriter.GenerateControls(r.Context(), draft.ControlIntents)
	if err != nil {
		api.logger.Error("Failed to generate controls", "plan_id", planID, "error", err)
		// Continue with empty controls rather than failing
		controls = []agents.PolicyControl{}
	}

	// Step 4: Expand targets using segmenter if needed
	canaryLimit := 5 // Default canary limit
	if len(targets) > 0 {
		api.logger.Info("Expanding targets", "plan_id", planID, "primary_target", targets[0], "canary_limit", canaryLimit)
		segmentation, err := api.segmenter.InferRelatedTargets(r.Context(), targets[0], canaryLimit)
		if err != nil {
			api.logger.Warn("Failed to expand targets, using original", "plan_id", planID, "error", err)
		} else {
			// Add related targets to the target list
			for _, relatedTarget := range segmentation.RelatedTargets {
				targets = append(targets, relatedTarget.TargetID)
			}
			// Limit to canary size
			if len(targets) > canaryLimit {
				targets = targets[:canaryLimit]
			}
		}
	}

	// Step 5: Decide strategy using guardrails
	hostLabels := []string{}
	if context, ok := finding["context"].(map[string]interface{}); ok {
		if labels, ok := context["labels"].([]string); ok {
			hostLabels = labels
		}
	}
	if labels, ok := finding["labels"].([]string); ok {
		hostLabels = append(hostLabels, labels...)
	}

	// Use override strategy mode if provided
	desiredMode := draft.DesiredMode
	if req.StrategyMode != nil {
		desiredMode = *req.StrategyMode
	}

	api.logger.Info("Deciding strategy", "plan_id", planID, "desired_mode", desiredMode, "num_targets", len(targets), "host_labels", hostLabels)
	strategyDecision, err := api.guardrails.DecideStrategy(string(desiredMode), len(targets), hostLabels)
	if err != nil {
		api.logger.Error("Failed to decide strategy", "plan_id", planID, "error", err)
		// Fallback to conservative strategy
		strategyDecision = &guardrails.StrategyDecision{
			Strategy:   model.StrategyModeConservative,
			CanarySize: 0,
			TTLSeconds: 3600,
			Reasons:    []string{"Fallback to conservative strategy due to guardrails error"},
			AppliedRules: []string{"fallback"},
		}
	}

	// Step 6: Generate explanation using explainer
	planForExplanation := map[string]interface{}{
		"id":        planID,
		"status":    "draft",
		"strategy": map[string]interface{}{
			"mode": strategyDecision.Strategy,
			"success_criteria": map[string]interface{}{
				"min_success_rate": 0.95,
				"timeout_seconds":  300,
			},
		},
		"targets": targets,
		"controls": []map[string]interface{}{
			{
				"gates": []string{"monitor_network_connections", "block_suspicious_traffic"},
			},
		},
		"finding": map[string]interface{}{
			"id":       finding["id"],
			"type":     "security_incident",
			"severity": finding["severity"],
		},
		"ttl_seconds": strategyDecision.TTLSeconds,
	}

	api.logger.Info("Generating explanation", "plan_id", planID)
	explanation, err := api.explainer.ExplainPlan(r.Context(), planForExplanation)
	if err != nil {
		api.logger.Error("Failed to generate explanation", "plan_id", planID, "error", err)
		explanation = "Plan explanation could not be generated"
	}

	// Step 7: Assemble final plan
	plan := model.Plan{
		ID:        planID,
		Status:    model.PlanStatusProposed,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Finding: model.Finding{
			ID:         getStringFromFinding(finding, "id"),
			Severity:   getStringFromFinding(finding, "severity"),
			Confidence: getFloatFromFinding(finding, "confidence"),
			HostID:     getStringFromFinding(finding, "host_id"),
			RuleID:     getStringFromFinding(finding, "rule_id"),
			Evidence:   convertToStringSlice(finding["evidence"]),
			Timestamp:  time.Now(),
		},
		Strategy: model.Strategy{
			Mode: strategyDecision.Strategy,
			SuccessCriteria: model.SuccessCriteria{
				MinSuccessRate: 0.95,
				MaxFailureRate: 0.05,
				TimeoutSeconds: 300,
			},
		},
		Targets: targets,
		Controls: convertToModelControls(controls),
		TTLSeconds: strategyDecision.TTLSeconds,
		Notes: explanation,
	}

	// Step 8: Store plan
	api.logger.Info("Storing plan", "plan_id", planID, "status", plan.Status)
	if err := api.store.Store(r.Context(), &plan); err != nil {
		api.logger.Error("Failed to store plan", "plan_id", planID, "error", err)
		http.Error(w, "Failed to store plan", http.StatusInternalServerError)
		return
	}

	// Step 9: Publish plan to NATS
	api.logger.Info("Publishing plan", "plan_id", planID)
	planData, _ := json.Marshal(plan)
	if err := api.natsConn.Publish("plans.created", planData); err != nil {
		api.logger.Error("Failed to publish plan", "plan_id", planID, "error", err)
		// Don't fail the request if NATS publish fails
	}

	// Structured logging for pipeline completion
	api.logger.Info("Plan pipeline completed", 
		"plan_id", planID,
		"targets", len(targets),
		"strategy_mode", string(strategyDecision.Strategy),
		"controls_count", len(controls),
		"ttl_seconds", strategyDecision.TTLSeconds,
		"applied_rules", strategyDecision.AppliedRules,
	)

	// Return response
	response := model.CreatePlanResponse{
		Plan:    plan,
		Message: "Plan created successfully with agentic pipeline",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.logger.Error("Failed to encode response", "plan_id", planID, "error", err)
	}
}

// handleListPlans handles GET /plans requests
func (api *HTTPAPI) handleListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := api.store.List(r.Context())
	if err != nil {
		api.logger.Error("Failed to list plans", "error", err)
		http.Error(w, "Failed to list plans", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(plans); err != nil {
		api.logger.Error("Failed to encode response", "error", err)
	}
}

// handleGetPlan handles GET /plans/{id} requests
func (api *HTTPAPI) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	// Extract plan ID from URL path
	planID := r.URL.Path[len("/plans/"):]
	if planID == "" {
		http.Error(w, "Plan ID required", http.StatusBadRequest)
		return
	}

	plan, err := api.store.Get(r.Context(), planID)
	if err != nil {
		api.logger.Error("Failed to get plan", "error", err, "plan_id", planID)
		http.Error(w, "Plan not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(plan); err != nil {
		api.logger.Error("Failed to encode response", "error", err)
	}
}

// handleHealth handles GET /healthz requests
func (api *HTTPAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "decision",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleReady handles GET /readyz requests
func (api *HTTPAPI) handleReady(w http.ResponseWriter, r *http.Request) {
	// Check if NATS is connected
	natsConnected := api.natsConn != nil && api.natsConn.IsConnected()

	response := map[string]interface{}{
		"status":         "ready",
		"timestamp":      time.Now(),
		"service":        "decision",
		"nats_connected": natsConnected,
	}

	status := http.StatusOK
	if !natsConnected {
		status = http.StatusServiceUnavailable
		response["status"] = "not ready"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// createStubPlan creates a stub plan for testing
func (api *HTTPAPI) createStubPlan(req *model.CreatePlanRequest) *model.Plan {
	now := time.Now()
	
	// Generate a unique plan ID
	planID := fmt.Sprintf("plan-%d", now.UnixNano())
	
	// Determine strategy mode
	strategyMode := model.StrategyModeBalanced
	if req.StrategyMode != nil {
		strategyMode = *req.StrategyMode
	}

	// Create stub plan
	plan := &model.Plan{
		ID:          planID,
		Name:        "Stub Decision Plan",
		Description: "This is a stub plan created for testing. Real agentic pipeline will be implemented in later prompts.",
		Status:      model.PlanStatusPending,
		Strategy: model.Strategy{
			Mode: strategyMode,
			SuccessCriteria: model.SuccessCriteria{
				MinSuccessRate:  0.8,
				MaxFailureRate:  0.2,
				TimeoutSeconds:  300,
				RequiredMetrics: []string{"success_rate", "response_time"},
			},
			Rollback: model.Rollback{
				Enabled:        true,
				Triggers:       []string{"failure_rate_exceeded", "timeout"},
				TimeoutSeconds: 60,
				Actions:        []string{"rollback_changes", "notify_team"},
			},
			Control: model.Control{
				ManualApproval: false,
				Gates:          []string{"validation", "testing"},
				Escalation:     []string{"notify_manager", "emergency_procedure"},
			},
			Description: fmt.Sprintf("Stub strategy for %s mode", strategyMode),
		},
		Steps: []string{
			"Analyze finding",
			"Determine response strategy",
			"Execute mitigation steps",
			"Monitor results",
			"Report completion",
		},
		Metadata:    req.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   &time.Time{},
	}

	// Set finding ID if provided
	if req.FindingID != nil {
		plan.FindingID = *req.FindingID
	}

	// Set expiration time (1 hour from now)
	expiresAt := now.Add(1 * time.Hour)
	plan.ExpiresAt = &expiresAt

	api.logger.Info("Created stub plan", "plan_id", plan.ID, "strategy_mode", strategyMode)
	return plan
}

// handleGetRoles handles GET /agents/roles requests
func (api *HTTPAPI) handleGetRoles(w http.ResponseWriter, r *http.Request) {
	roles := api.agentRuntime.GetAvailableRoles()
	
	response := map[string]interface{}{
		"roles": roles,
		"count": len(roles),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.logger.Error("Failed to encode response", "error", err)
	}
}

// handleGetBudget handles GET /agents/budget requests
func (api *HTTPAPI) handleGetBudget(w http.ResponseWriter, r *http.Request) {
	stats := api.agentRuntime.GetBudgetStats()
	
	response := map[string]interface{}{
		"budget_stats": stats,
		"timestamp":    time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.logger.Error("Failed to encode response", "error", err)
	}
}

// handleCreatePlanDraft handles POST /plans/draft requests
func (api *HTTPAPI) handleCreatePlanDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		FindingID string                 `json:"finding_id,omitempty"`
		Finding   map[string]interface{} `json:"finding,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if request.FindingID == "" && len(request.Finding) == 0 {
		http.Error(w, "Either finding_id or finding must be provided", http.StatusBadRequest)
		return
	}

	var finding map[string]interface{}
	if request.FindingID != "" {
		// TODO: Fetch finding by ID from correlator service
		// For now, create a mock finding
		finding = map[string]interface{}{
			"id":        request.FindingID,
			"severity":  "high",
			"host_id":   "web-01",
			"rule_id":   "bash-exec-after-connect",
			"evidence":  []string{"Bash execution detected after network connection"},
			"confidence": 0.8,
		}
	} else {
		finding = request.Finding
	}

	// Create plan draft using planner agent
	draft, err := api.planner.BuildPlanDraft(r.Context(), finding)
	if err != nil {
		api.logger.Error("Failed to create plan draft", "error", err)
		http.Error(w, "Failed to create plan draft", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"draft":     draft,
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.logger.Error("Failed to encode response", "error", err)
	}
}

// handleGeneratePolicy handles POST /plans/policy requests
func (api *HTTPAPI) handleGeneratePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		ControlIntents []map[string]interface{} `json:"control_intents"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if len(request.ControlIntents) == 0 {
		http.Error(w, "control_intents must be provided and non-empty", http.StatusBadRequest)
		return
	}

	// Generate policy controls using policy writer agent
	controls, err := api.policyWriter.GenerateControls(r.Context(), request.ControlIntents)
	if err != nil {
		api.logger.Error("Failed to generate policy controls", "error", err)
		http.Error(w, "Failed to generate policy controls", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"controls":  controls,
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.logger.Error("Failed to encode response", "error", err)
	}
}

// handleSegmentTargets handles POST /plans/segment requests
func (api *HTTPAPI) handleSegmentTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		PrimaryTarget string `json:"primary_target"`
		CanarySize    int    `json:"canary_size,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if request.PrimaryTarget == "" {
		http.Error(w, "primary_target must be provided", http.StatusBadRequest)
		return
	}

	// Segment targets using segmenter agent
	result, err := api.segmenter.InferRelatedTargets(r.Context(), request.PrimaryTarget, request.CanarySize)
	if err != nil {
		api.logger.Error("Failed to segment targets", "error", err)
		http.Error(w, "Failed to segment targets", http.StatusInternalServerError)
		return
	}

	// Get segmentation summary
	summary := api.segmenter.GetSegmentationSummary(result)

	response := map[string]interface{}{
		"segmentation": result,
		"summary":      summary,
		"timestamp":    time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.logger.Error("Failed to encode response", "error", err)
	}
}

// handleExplainPlan handles POST /plans/explain requests
func (api *HTTPAPI) handleExplainPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Plan interface{} `json:"plan"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if request.Plan == nil {
		http.Error(w, "plan must be provided", http.StatusBadRequest)
		return
	}

	// Generate explanation using explainer agent
	explanation, err := api.explainer.ExplainPlan(r.Context(), request.Plan)
	if err != nil {
		api.logger.Error("Failed to explain plan", "error", err)
		http.Error(w, "Failed to explain plan", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"explanation": explanation,
		"timestamp":   time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.logger.Error("Failed to encode response", "error", err)
	}
}

// handleDecideStrategy handles POST /guardrails/strategy requests
func (api *HTTPAPI) handleDecideStrategy(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		DesiredStrategy string   `json:"desired_strategy"`
		NumTargets      int      `json:"num_targets"`
		HostLabels      []string `json:"host_labels"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if request.DesiredStrategy == "" {
		http.Error(w, "desired_strategy must be provided", http.StatusBadRequest)
		return
	}
	if request.NumTargets < 0 {
		http.Error(w, "num_targets must be non-negative", http.StatusBadRequest)
		return
	}

	// Decide strategy using guardrails
	decision, err := api.guardrails.DecideStrategy(request.DesiredStrategy, request.NumTargets, request.HostLabels)
	if err != nil {
		api.logger.Error("Failed to decide strategy", "error", err)
		http.Error(w, "Failed to decide strategy", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"decision":  decision,
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.logger.Error("Failed to encode response", "error", err)
	}
}

// handleGuardrailsStatus handles GET /guardrails/status requests
func (api *HTTPAPI) handleGuardrailsStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := api.guardrails.GetGuardrailsStatus()

	response := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.logger.Error("Failed to encode response", "error", err)
	}
}

// Helper functions for the pipeline

// getStringFromFinding safely extracts a string value from finding map
func getStringFromFinding(finding map[string]interface{}, key string) string {
	if val, ok := finding[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getFloatFromFinding safely extracts a float64 value from finding map
func getFloatFromFinding(finding map[string]interface{}, key string) float64 {
	if val, ok := finding[key]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return 0.0
}

// convertToStringSlice safely converts interface{} to []string
func convertToStringSlice(val interface{}) []string {
	if val == nil {
		return []string{}
	}
	
	if slice, ok := val.([]interface{}); ok {
		result := make([]string, len(slice))
		for i, v := range slice {
			if str, ok := v.(string); ok {
				result[i] = str
			}
		}
		return result
	}
	
	if slice, ok := val.([]string); ok {
		return slice
	}
	
	return []string{}
}

// convertToModelControls converts PolicyControl slice to model.Control slice
func convertToModelControls(policyControls []agents.PolicyControl) []model.Control {
	controls := make([]model.Control, len(policyControls))
	for i, pc := range policyControls {
		controls[i] = model.Control{
			ID:          fmt.Sprintf("control-%d", i+1),
			Type:        pc.Artifacts[0].Type, // Use first artifact type
			Description: pc.Description,
			Gates:       extractGatesFromArtifacts(pc.Artifacts),
		}
	}
	return controls
}

// extractGatesFromArtifacts extracts gate names from policy artifacts
func extractGatesFromArtifacts(artifacts []agents.PolicyArtifact) []string {
	var gates []string
	for _, artifact := range artifacts {
		switch artifact.Type {
		case "nftables":
			gates = append(gates, "network_filtering")
		case "cilium":
			gates = append(gates, "network_policy")
		case "ebpf":
			gates = append(gates, "runtime_monitoring")
		default:
			gates = append(gates, "generic_control")
		}
	}
	
	// Remove duplicates
	gateMap := make(map[string]bool)
	var uniqueGates []string
	for _, gate := range gates {
		if !gateMap[gate] {
			gateMap[gate] = true
			uniqueGates = append(uniqueGates, gate)
		}
	}
	
	return uniqueGates
}
