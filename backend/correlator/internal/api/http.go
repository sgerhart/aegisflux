package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aegisflux/correlator/internal/metrics"
	"github.com/aegisflux/correlator/internal/model"
	"github.com/aegisflux/correlator/internal/rules"
	"github.com/aegisflux/correlator/internal/store"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPAPI provides HTTP endpoints for the correlator service
type HTTPAPI struct {
	store           *store.MemoryStore
	ruleLoader      *rules.Loader
	overrideManager *rules.OverrideManager
	metrics         *metrics.Metrics
	natsConn        *nats.Conn
}

// NewHTTPAPI creates a new HTTP API instance
func NewHTTPAPI(store *store.MemoryStore, ruleLoader *rules.Loader, overrideManager *rules.OverrideManager, metrics *metrics.Metrics, natsConn *nats.Conn) *HTTPAPI {
	return &HTTPAPI{
		store:           store,
		ruleLoader:      ruleLoader,
		overrideManager: overrideManager,
		metrics:         metrics,
		natsConn:        natsConn,
	}
}

// SetupRoutes configures HTTP routes
func (api *HTTPAPI) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/findings", api.handleFindings)
	mux.HandleFunc("/findings/reset", api.handleResetFindings)
	mux.HandleFunc("/rules", api.handleRules)
	mux.HandleFunc("/rules/overrides", api.handleRuleOverrides)
	mux.HandleFunc("/rules/overrides/", api.handleRuleOverrideByID)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", api.handleHealth)
	mux.HandleFunc("/readyz", api.handleReady)
}

// handleFindings handles GET /findings with optional query parameters
func (api *HTTPAPI) handleFindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var findings []*model.Finding

	// Parse query parameters
	hostID := r.URL.Query().Get("host_id")
	severity := r.URL.Query().Get("severity")
	limitStr := r.URL.Query().Get("limit")

	if hostID != "" {
		findings = api.store.GetFindingsByHost(hostID)
	} else if severity != "" {
		findings = api.store.GetFindingsBySeverity(severity)
	} else {
		findings = api.store.GetFindings()
	}

	// Apply limit if specified
	if limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			if limit < len(findings) {
				findings = findings[:limit]
			}
		}
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Return findings as JSON
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"findings": findings,
		"count":    len(findings),
		"timestamp": time.Now().UTC(),
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// handleResetFindings handles POST /findings/reset
func (api *HTTPAPI) handleResetFindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	api.store.ClearFindings()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"message":   "Findings cleared successfully",
		"timestamp": time.Now().UTC(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// handleHealth handles GET /healthz
func (api *HTTPAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := api.store.GetStats()

	// Update metrics
	if totalFindings, ok := stats["total_findings"].(int); ok {
		api.metrics.SetFindingsInStore(float64(totalFindings))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"stats":     stats,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// handleReady handles GET /readyz
func (api *HTTPAPI) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check NATS connection
	natsConnected := api.natsConn != nil && api.natsConn.IsConnected()
	api.metrics.SetNatsConnected(natsConnected)

	// Check if loader snapshot is loaded
	snapshot := api.ruleLoader.GetSnapshot()
	rulesLoaded := len(snapshot.Rules) > 0

	// Determine readiness
	ready := natsConnected && rulesLoaded
	status := "ready"
	statusCode := http.StatusOK

	if !ready {
		status = "not ready"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"status":         status,
		"timestamp":      time.Now().UTC(),
		"nats_connected": natsConnected,
		"rules_loaded":   rulesLoaded,
		"rules_count":    len(snapshot.Rules),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// handleRules handles GET /rules
func (api *HTTPAPI) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get rule files summary
	snapshot := api.ruleLoader.GetSnapshot()
	var files []rules.RuleFileSummary
	
	// For now, we'll create a simple summary
	// In a real implementation, you'd get this from the rule loader
	files = append(files, rules.RuleFileSummary{
		Filename:     "rules.d",
		RuleCount:    len(snapshot.Rules),
		EnabledCount: len(snapshot.Rules), // Simplified
		DisabledCount: 0,
	})

	// Get overrides summary
	overrides := api.overrideManager.ListOverrides()

	// Update metrics
	api.metrics.SetRulesLoaded(float64(len(snapshot.Rules)))
	api.metrics.SetRulesOverrides(float64(len(overrides)))

	response := rules.RuleSummaryResponse{
		Files:     files,
		Overrides: overrides,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// handleRuleOverrides handles POST /rules/overrides
func (api *HTTPAPI) handleRuleOverrides(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	var requestBody []byte
	var err error
	if requestBody, err = readRequestBody(r); err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Parse and validate request
	var request struct {
		RuleID      string   `json:"rule_id"`
		Enabled     *bool    `json:"enabled,omitempty"`
		Severity    *string  `json:"severity,omitempty"`
		Confidence  *float64 `json:"confidence,omitempty"`
		TTLSeconds  *int     `json:"ttl_seconds,omitempty"`
		Description string   `json:"description,omitempty"`
	}

	if err := json.Unmarshal(requestBody, &request); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	// Validate the override
	if err := api.overrideManager.ValidateOverride(
		request.RuleID,
		request.Enabled,
		request.Severity,
		request.Confidence,
		request.TTLSeconds,
	); err != nil {
		http.Error(w, fmt.Sprintf("Invalid override: %v", err), http.StatusBadRequest)
		return
	}

	// Add the override
	override, err := api.overrideManager.AddOverride(
		request.RuleID,
		request.Enabled,
		request.Severity,
		request.Confidence,
		request.TTLSeconds,
		request.Description,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to add override: %v", err), http.StatusInternalServerError)
		return
	}

	// Return the override ID
	response := map[string]interface{}{
		"id":        override.ID,
		"message":   "Override added successfully",
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// handleRuleOverrideByID handles DELETE /rules/overrides/{id}
func (api *HTTPAPI) handleRuleOverrideByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract override ID from URL path
	path := r.URL.Path
	prefix := "/rules/overrides/"
	if !strings.HasPrefix(path, prefix) {
		http.Error(w, "Invalid override ID", http.StatusBadRequest)
		return
	}

	overrideID := strings.TrimPrefix(path, prefix)
	if overrideID == "" {
		http.Error(w, "Override ID is required", http.StatusBadRequest)
		return
	}

	// Remove the override
	if err := api.overrideManager.RemoveOverride(overrideID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to remove override: %v", err), http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"message":   "Override removed successfully",
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// readRequestBody reads the request body
func readRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	
	// Limit body size to prevent abuse
	const maxBodySize = 1024 * 1024 // 1MB
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodySize)
	
	// Read the body
	body := make([]byte, 0, 512)
	buf := make([]byte, 512)
	
	for {
		n, err := r.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
	}
	
	return body, nil
}
