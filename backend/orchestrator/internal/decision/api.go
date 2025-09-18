package decision

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"log/slog"
	"aegisflux/backend/orchestrator/internal/rollout"
)

// DecisionAPIServer provides HTTP API endpoints for decision engine integration
type DecisionAPIServer struct {
	logger         *slog.Logger
	processor      *DecisionProcessor
	rolloutIntegration *DecisionRolloutIntegration
	router         *mux.Router
}

// NewDecisionAPIServer creates a new decision API server
func NewDecisionAPIServer(logger *slog.Logger, processor *DecisionProcessor, rolloutIntegration *DecisionRolloutIntegration) *DecisionAPIServer {
	server := &DecisionAPIServer{
		logger:         logger,
		processor:      processor,
		rolloutIntegration: rolloutIntegration,
		router:         mux.NewRouter(),
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures the HTTP routes
func (s *DecisionAPIServer) setupRoutes() {
	// Health check
	s.router.HandleFunc("/healthz", s.handleHealth).Methods("GET")

	// Plan management endpoints
	s.router.HandleFunc("/plans", s.handleCreatePlan).Methods("POST")
	s.router.HandleFunc("/plans/{plan_id}", s.handleGetPlan).Methods("GET")
	s.router.HandleFunc("/plans/{plan_id}", s.handleUpdatePlan).Methods("PUT")
	s.router.HandleFunc("/plans/{plan_id}/status", s.handleGetPlanStatus).Methods("GET")
	s.router.HandleFunc("/plans/{plan_id}/process", s.handleProcessPlan).Methods("POST")

	// Control management endpoints
	s.router.HandleFunc("/plans/{plan_id}/controls", s.handleCreateControl).Methods("POST")
	s.router.HandleFunc("/plans/{plan_id}/controls/{control_id}", s.handleGetControl).Methods("GET")
	s.router.HandleFunc("/plans/{plan_id}/controls/{control_id}", s.handleUpdateControl).Methods("PUT")
	s.router.HandleFunc("/plans/{plan_id}/controls/{control_id}", s.handleDeleteControl).Methods("DELETE")

	// Deployment endpoints
	s.router.HandleFunc("/plans/{plan_id}/deploy", s.handleDeployPlan).Methods("POST")
	s.router.HandleFunc("/plans/{plan_id}/deployments", s.handleListDeployments).Methods("GET")
	s.router.HandleFunc("/deployments/{deployment_id}", s.handleGetDeploymentStatus).Methods("GET")
	s.router.HandleFunc("/deployments/{deployment_id}/rollback", s.handleRollbackDeployment).Methods("POST")

	// Rollout integration endpoints
	s.router.HandleFunc("/rollouts", s.handleListActiveRollouts).Methods("GET")
	s.router.HandleFunc("/rollouts/{request_id}", s.handleGetRolloutStatus).Methods("GET")
	s.router.HandleFunc("/rollouts/{request_id}/rollback", s.handleRollbackRollout).Methods("POST")

	// Telemetry endpoints
	s.router.HandleFunc("/telemetry/plans/{plan_id}", s.handleGetPlanTelemetry).Methods("GET")
	s.router.HandleFunc("/telemetry/controls/{control_id}", s.handleGetControlTelemetry).Methods("GET")
}

// ServeHTTP implements http.Handler interface
func (s *DecisionAPIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// handleHealth handles health check requests
func (s *DecisionAPIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "decision-integration-api",
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleCreatePlan handles plan creation requests
func (s *DecisionAPIServer) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var plan Plan
	if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}

	// Validate plan
	if plan.ID == "" || plan.Name == "" {
		s.writeErrorResponse(w, http.StatusBadRequest, "Missing required fields: id, name")
		return
	}

	// In a real implementation, you'd save the plan to the decision engine
	s.logger.Info("Plan created", "plan_id", plan.ID, "name", plan.Name)

	response := map[string]interface{}{
		"plan_id":    plan.ID,
		"status":     "created",
		"created_at": time.Now(),
	}

	s.writeJSONResponse(w, http.StatusCreated, response)
}

// handleGetPlan handles plan retrieval requests
func (s *DecisionAPIServer) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["plan_id"]

	// In a real implementation, you'd retrieve from the decision engine
	plan := &Plan{
		ID:     planID,
		Name:   "Example Plan",
		Status: "active",
		Controls: []Control{
			{
				ID:     "control-1",
				Type:   "ebpf_drop_egress_by_cgroup",
				Target: "host-1",
				Parameters: map[string]interface{}{
					"dst_ip":    "8.8.8.8",
					"dst_port":  "53",
					"cgroup_id": "12345",
					"ttl":       "3600",
				},
				Status: "pending",
			},
		},
	}

	s.writeJSONResponse(w, http.StatusOK, plan)
}

// handleUpdatePlan handles plan update requests
func (s *DecisionAPIServer) handleUpdatePlan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["plan_id"]

	var plan Plan
	if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}

	plan.ID = planID // Ensure ID matches URL parameter

	// In a real implementation, you'd update the plan in the decision engine
	s.logger.Info("Plan updated", "plan_id", planID)

	response := map[string]interface{}{
		"plan_id":    planID,
		"status":     "updated",
		"updated_at": time.Now(),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleGetPlanStatus handles plan status requests
func (s *DecisionAPIServer) handleGetPlanStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["plan_id"]

	status, err := s.processor.GetPlanStatus(r.Context(), planID)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "Plan not found")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, status)
}

// handleProcessPlan handles plan processing requests
func (s *DecisionAPIServer) handleProcessPlan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["plan_id"]

	// Get plan (in a real implementation, from decision engine)
	plan, err := s.processor.getPlan(r.Context(), planID)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "Plan not found")
		return
	}

	// Process plan
	response, err := s.processor.ProcessPlan(r.Context(), plan)
	if err != nil {
		s.logger.Error("Failed to process plan", "plan_id", planID, "error", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to process plan")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleCreateControl handles control creation requests
func (s *DecisionAPIServer) handleCreateControl(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["plan_id"]

	var control Control
	if err := json.NewDecoder(r.Body).Decode(&control); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}

	// Validate control
	if control.ID == "" || control.Type == "" || control.Target == "" {
		s.writeErrorResponse(w, http.StatusBadRequest, "Missing required fields: id, type, target")
		return
	}

	// In a real implementation, you'd add the control to the plan
	s.logger.Info("Control created", "plan_id", planID, "control_id", control.ID, "type", control.Type)

	response := map[string]interface{}{
		"plan_id":     planID,
		"control_id":  control.ID,
		"status":      "created",
		"created_at":  time.Now(),
	}

	s.writeJSONResponse(w, http.StatusCreated, response)
}

// handleGetControl handles control retrieval requests
func (s *DecisionAPIServer) handleGetControl(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["plan_id"]
	controlID := vars["control_id"]
	
	// Use planID to avoid unused variable warning
	_ = planID

	// In a real implementation, you'd retrieve from the decision engine
	control := Control{
		ID:     controlID,
		Type:   "ebpf_drop_egress_by_cgroup",
		Target: "host-1",
		Parameters: map[string]interface{}{
			"dst_ip":    "8.8.8.8",
			"dst_port":  "53",
			"cgroup_id": "12345",
			"ttl":       "3600",
		},
		Status: "pending",
	}

	s.writeJSONResponse(w, http.StatusOK, control)
}

// handleUpdateControl handles control update requests
func (s *DecisionAPIServer) handleUpdateControl(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["plan_id"]
	controlID := vars["control_id"]

	var control Control
	if err := json.NewDecoder(r.Body).Decode(&control); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}

	control.ID = controlID // Ensure ID matches URL parameter

	// Update control
	err := s.processor.UpdatePlanControl(r.Context(), planID, &control)
	if err != nil {
		s.logger.Error("Failed to update control", "plan_id", planID, "control_id", controlID, "error", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to update control")
		return
	}

	response := map[string]interface{}{
		"plan_id":     planID,
		"control_id":  controlID,
		"status":      "updated",
		"updated_at":  time.Now(),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleDeleteControl handles control deletion requests
func (s *DecisionAPIServer) handleDeleteControl(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["plan_id"]
	controlID := vars["control_id"]

	// In a real implementation, you'd delete the control from the plan
	s.logger.Info("Control deleted", "plan_id", planID, "control_id", controlID)

	response := map[string]interface{}{
		"plan_id":     planID,
		"control_id":  controlID,
		"status":      "deleted",
		"deleted_at":  time.Now(),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleDeployPlan handles plan deployment requests
func (s *DecisionAPIServer) handleDeployPlan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["plan_id"]

	var req DeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}

	req.PlanID = planID // Ensure plan ID matches URL parameter

	// Validate deployment request
	if req.DeploymentType == "" {
		req.DeploymentType = "canary" // Default to canary
	}

	// Deploy plan
	response, err := s.processor.DeployPlan(r.Context(), &req)
	if err != nil {
		s.logger.Error("Failed to deploy plan", "plan_id", planID, "error", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to deploy plan")
		return
	}

	s.writeJSONResponse(w, http.StatusAccepted, response)
}

// handleListDeployments handles deployment listing requests
func (s *DecisionAPIServer) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["plan_id"]

	// Get active deployments
	deployments := s.rolloutIntegration.GetActiveDeployments()

	// Filter by plan ID
	var planDeployments []*DeploymentResponse
	for _, deployment := range deployments {
		if deployment.PlanID == planID {
			planDeployments = append(planDeployments, deployment)
		}
	}

	response := map[string]interface{}{
		"plan_id":     planID,
		"deployments": planDeployments,
		"total":       len(planDeployments),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleGetDeploymentStatus handles deployment status requests
func (s *DecisionAPIServer) handleGetDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deploymentID := vars["deployment_id"]

	status, err := s.rolloutIntegration.GetDeploymentStatus(deploymentID)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "Deployment not found")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, status)
}

// handleRollbackDeployment handles deployment rollback requests
func (s *DecisionAPIServer) handleRollbackDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deploymentID := vars["deployment_id"]

	// Get deployment status
	deployment, err := s.rolloutIntegration.GetDeploymentStatus(deploymentID)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "Deployment not found")
		return
	}

	// Create rollback request
	rollbackReq := rollout.RollbackRequest{
		RequestID: fmt.Sprintf("rollback-%s-%d", deploymentID, time.Now().Unix()),
		Strategy:  rollout.RollbackStrategyImmediate,
		Targets:   deployment.Targets,
		Reason:    "Manual rollback requested via API",
	}

	// In a real implementation, you'd call the rollback manager
	s.logger.Info("Rollback requested", "deployment_id", deploymentID)

	response := map[string]interface{}{
		"deployment_id": deploymentID,
		"status":        "rollback_initiated",
		"reason":        rollbackReq.Reason,
		"timestamp":     time.Now(),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleListActiveRollouts handles active rollouts listing requests
func (s *DecisionAPIServer) handleListActiveRollouts(w http.ResponseWriter, r *http.Request) {
	rollouts := s.processor.rolloutMgr.ListActiveRollouts()

	response := map[string]interface{}{
		"rollouts": rollouts,
		"total":    len(rollouts),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleGetRolloutStatus handles rollout status requests
func (s *DecisionAPIServer) handleGetRolloutStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID := vars["request_id"]

	status, err := s.processor.rolloutMgr.GetRolloutStatus(requestID)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "Rollout not found")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, status)
}

// handleRollbackRollout handles rollout rollback requests
func (s *DecisionAPIServer) handleRollbackRollout(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID := vars["request_id"]

	// Get rollout status
	rollout, err := s.processor.rolloutMgr.GetRolloutStatus(requestID)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "Rollout not found")
		return
	}

	// Extract targets from rollout
	var targets []string
	for _, target := range rollout.Targets {
		targets = append(targets, target.TargetID)
	}

	// In a real implementation, you'd call the rollback manager
	s.logger.Info("Rollout rollback requested", "request_id", requestID, "targets", targets)

	response := map[string]interface{}{
		"request_id": requestID,
		"status":     "rollback_initiated",
		"targets":    targets,
		"timestamp":  time.Now(),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleGetPlanTelemetry handles plan telemetry requests
func (s *DecisionAPIServer) handleGetPlanTelemetry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["plan_id"]

	// Get telemetry for all controls in the plan
	// In a real implementation, you'd aggregate telemetry data
	telemetry := map[string]interface{}{
		"plan_id": planID,
		"telemetry": map[string]interface{}{
			"total_violations": 0,
			"total_errors":     0,
			"average_latency":  0.0,
		},
		"timestamp": time.Now(),
	}

	s.writeJSONResponse(w, http.StatusOK, telemetry)
}

// handleGetControlTelemetry handles control telemetry requests
func (s *DecisionAPIServer) handleGetControlTelemetry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	controlID := vars["control_id"]

	// Get telemetry for specific control
	// In a real implementation, you'd get real telemetry data
	telemetry := map[string]interface{}{
		"control_id": controlID,
		"telemetry": map[string]interface{}{
			"violations": 0,
			"errors":     0,
			"latency":    0.0,
			"packets":    0,
			"blocks":     0,
		},
		"timestamp": time.Now(),
	}

	s.writeJSONResponse(w, http.StatusOK, telemetry)
}

// writeJSONResponse writes a JSON response
func (s *DecisionAPIServer) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// writeErrorResponse writes an error response
func (s *DecisionAPIServer) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	errorResponse := map[string]interface{}{
		"error":     message,
		"timestamp": time.Now(),
	}

	s.writeJSONResponse(w, statusCode, errorResponse)
}
