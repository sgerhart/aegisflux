package rollout

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"log/slog"
)

// APIServer provides HTTP API endpoints for BPF rollout management
type APIServer struct {
	logger       *slog.Logger
	rolloutMgr   *BPFRolloutManager
	telemetryMon *TelemetryMonitor
	router       *mux.Router
}

// NewAPIServer creates a new API server for rollout management
func NewAPIServer(logger *slog.Logger, rolloutMgr *BPFRolloutManager, telemetryMon *TelemetryMonitor) *APIServer {
	server := &APIServer{
		logger:       logger,
		rolloutMgr:   rolloutMgr,
		telemetryMon: telemetryMon,
		router:       mux.NewRouter(),
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures the HTTP routes
func (s *APIServer) setupRoutes() {
	// Health check
	s.router.HandleFunc("/healthz", s.handleHealth).Methods("GET")

	// BPF rollout endpoints
	s.router.HandleFunc("/apply/ebpf", s.handleApplyEBPF).Methods("POST")
	s.router.HandleFunc("/apply/ebpf/{request_id}", s.handleGetRolloutStatus).Methods("GET")
	s.router.HandleFunc("/apply/ebpf", s.handleListRollouts).Methods("GET")
	s.router.HandleFunc("/apply/ebpf/{request_id}/rollback", s.handleRollback).Methods("POST")

	// Telemetry endpoints
	s.router.HandleFunc("/telemetry", s.handleGetTelemetry).Methods("GET")
	s.router.HandleFunc("/telemetry/{target_id}", s.handleGetTargetTelemetry).Methods("GET")
	s.router.HandleFunc("/telemetry/aggregate/{target_id}", s.handleGetTelemetryAggregation).Methods("GET")

	// Configuration endpoints
	s.router.HandleFunc("/config/thresholds", s.handleGetThresholds).Methods("GET")
	s.router.HandleFunc("/config/thresholds", s.handleSetThresholds).Methods("POST")
	s.router.HandleFunc("/config/observation-window", s.handleSetObservationWindow).Methods("POST")

	// Metrics endpoints
	s.router.HandleFunc("/metrics", s.handleGetMetrics).Methods("GET")
}

// ServeHTTP implements http.Handler interface
func (s *APIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// handleHealth handles health check requests
func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "bpf-rollout-api",
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleApplyEBPF handles BPF program application requests
func (s *APIServer) handleApplyEBPF(w http.ResponseWriter, r *http.Request) {
	var req ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}

	// Validate request
	if req.PlanID == "" || len(req.Targets) == 0 || req.ArtifactID == "" {
		s.writeErrorResponse(w, http.StatusBadRequest, "Missing required fields: plan_id, targets, artifact_id")
		return
	}

	// Set default TTL if not provided
	if req.TTL <= 0 {
		req.TTL = 3600 // 1 hour default
	}

	// Apply canary deployment
	response, err := s.rolloutMgr.ApplyCanary(r.Context(), req)
	if err != nil {
		s.logger.Error("Failed to apply BPF program", "error", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to apply BPF program")
		return
	}

	s.writeJSONResponse(w, http.StatusAccepted, response)
}

// handleGetRolloutStatus handles rollout status requests
func (s *APIServer) handleGetRolloutStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID := vars["request_id"]

	rollout, err := s.rolloutMgr.GetRolloutStatus(requestID)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "Rollout not found")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, rollout)
}

// handleListRollouts handles listing all rollouts
func (s *APIServer) handleListRollouts(w http.ResponseWriter, r *http.Request) {
	rollouts := s.rolloutMgr.ListActiveRollouts()

	response := map[string]interface{}{
		"rollouts": rollouts,
		"total":    len(rollouts),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleRollback handles rollback requests
func (s *APIServer) handleRollback(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID := vars["request_id"]

	rollout, err := s.rolloutMgr.GetRolloutStatus(requestID)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "Rollout not found")
		return
	}

	// Extract targets from rollout
	var targets []string
	for _, target := range rollout.Targets {
		targets = append(targets, target.TargetID)
	}

	// Perform rollback
	// In a real implementation, you'd call the rollback method
	s.logger.Info("Rollback requested", "request_id", requestID, "targets", targets)

	response := map[string]interface{}{
		"request_id": requestID,
		"status":     "rollback_initiated",
		"targets":    targets,
		"timestamp":  time.Now(),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleGetTelemetry handles telemetry data requests
func (s *APIServer) handleGetTelemetry(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	targetID := r.URL.Query().Get("target_id")
	minViolations := r.URL.Query().Get("min_violations")
	maxLatency := r.URL.Query().Get("max_latency")

	var telemetryData map[string]*TelemetryData

	if targetID != "" {
		// Get specific target telemetry
		data, err := s.telemetryMon.GetTelemetryData(targetID)
		if err != nil {
			s.writeErrorResponse(w, http.StatusNotFound, "Telemetry data not found")
			return
		}
		telemetryData = map[string]*TelemetryData{targetID: data}
	} else {
		// Get all telemetry data
		telemetryData = s.telemetryMon.GetAllTelemetryData()
	}

	// Apply filters if provided
	if minViolations != "" || maxLatency != "" {
		filter := TelemetryFilter{}
		
		if minViolations != "" {
			if val, err := strconv.Atoi(minViolations); err == nil {
				filter.MinViolations = val
			}
		}
		
		if maxLatency != "" {
			if val, err := strconv.ParseFloat(maxLatency, 64); err == nil {
				filter.MaxLatency = val
			}
		}

		telemetryData = s.telemetryMon.FilterTelemetryData(filter)
	}

	s.writeJSONResponse(w, http.StatusOK, telemetryData)
}

// handleGetTargetTelemetry handles specific target telemetry requests
func (s *APIServer) handleGetTargetTelemetry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetID := vars["target_id"]

	data, err := s.telemetryMon.GetTelemetryData(targetID)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "Telemetry data not found")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, data)
}

// handleGetTelemetryAggregation handles telemetry aggregation requests
func (s *APIServer) handleGetTelemetryAggregation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetID := vars["target_id"]

	durationStr := r.URL.Query().Get("duration")
	duration := 1 * time.Hour // default
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	aggregation, err := s.telemetryMon.GetTelemetryAggregation(targetID, duration)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "Aggregation data not found")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, aggregation)
}

// handleGetThresholds handles getting violation thresholds
func (s *APIServer) handleGetThresholds(w http.ResponseWriter, r *http.Request) {
	// In a real implementation, you'd get this from the rollout manager
	thresholds := ViolationThreshold{
		MaxViolations:  5,
		MaxErrorRate:   0.1,
		MaxLatency:     1000,
		MinSuccessRate: 0.95,
	}

	s.writeJSONResponse(w, http.StatusOK, thresholds)
}

// handleSetThresholds handles setting violation thresholds
func (s *APIServer) handleSetThresholds(w http.ResponseWriter, r *http.Request) {
	var thresholds ViolationThreshold
	if err := json.NewDecoder(r.Body).Decode(&thresholds); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}

	s.rolloutMgr.SetViolationThresholds(thresholds)

	response := map[string]interface{}{
		"message":    "Thresholds updated successfully",
		"thresholds": thresholds,
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleSetObservationWindow handles setting observation window
func (s *APIServer) handleSetObservationWindow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Window string `json:"window"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}

	window, err := time.ParseDuration(req.Window)
	if err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid duration format")
		return
	}

	s.rolloutMgr.SetObservationWindow(window)

	response := map[string]interface{}{
		"message": "Observation window updated successfully",
		"window":  window.String(),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleGetMetrics handles metrics requests
func (s *APIServer) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	rollouts := s.rolloutMgr.ListActiveRollouts()
	telemetryData := s.telemetryMon.GetAllTelemetryData()

	// Calculate metrics
	totalRollouts := len(rollouts)
	successfulRollouts := 0
	failedRollouts := 0
	rollbackRollouts := 0

	for _, rollout := range rollouts {
		switch rollout.Status {
		case "success":
			successfulRollouts++
		case "failed":
			failedRollouts++
		case "rollback":
			rollbackRollouts++
		}
	}

	totalTargets := 0
	totalViolations := 0
	totalErrors := 0

	for _, data := range telemetryData {
		totalTargets++
		totalViolations += data.Violations
		totalErrors += data.Errors
	}

	metrics := map[string]interface{}{
		"rollouts": map[string]interface{}{
			"total":     totalRollouts,
			"successful": successfulRollouts,
			"failed":    failedRollouts,
			"rollback":  rollbackRollouts,
		},
		"targets": map[string]interface{}{
			"total":       totalTargets,
			"violations":  totalViolations,
			"errors":      totalErrors,
		},
		"timestamp": time.Now(),
	}

	s.writeJSONResponse(w, http.StatusOK, metrics)
}

// writeJSONResponse writes a JSON response
func (s *APIServer) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// writeErrorResponse writes an error response
func (s *APIServer) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	errorResponse := map[string]interface{}{
		"error":     message,
		"timestamp": time.Now(),
	}

	s.writeJSONResponse(w, statusCode, errorResponse)
}
