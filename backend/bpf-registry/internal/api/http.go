package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"aegisflux/backend/bpf-registry/internal/model"
)

// ArtifactStore interface for artifact operations
type ArtifactStore interface {
	StoreArtifact(req *model.CreateArtifactRequest) (*model.Artifact, error)
	GetArtifact(id string) (*model.Artifact, error)
	GetArtifactBinary(id string) ([]byte, error)
	GetArtifactsForHost(hostID string) ([]*model.Artifact, error)
	ListArtifacts() ([]*model.Artifact, error)
}

// HTTPAPI handles HTTP requests for the BPF registry
type HTTPAPI struct {
	store  ArtifactStore
	logger *slog.Logger
}

// NewHTTPAPI creates a new HTTP API handler
func NewHTTPAPI(store ArtifactStore, logger *slog.Logger) *HTTPAPI {
	return &HTTPAPI{
		store:  store,
		logger: logger,
	}
}

// SetupRoutes sets up all HTTP routes
func (api *HTTPAPI) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/healthz", api.handleHealthz)

	// Artifact endpoints - more specific routes to avoid conflicts
	mux.HandleFunc("/artifacts", api.handleArtifacts) // POST for creation, GET for listing
	
	// Specific routes for different artifact operations
	mux.HandleFunc("/artifacts/for-host/", api.handleGetArtifactsForHost)
	mux.HandleFunc("/artifacts/binary/", api.handleGetArtifactBinary)
	mux.HandleFunc("/artifacts/", api.handleGetArtifact) // GET for individual artifact metadata

	return mux
}

// handleHealthz handles health check requests
func (api *HTTPAPI) handleHealthz(w http.ResponseWriter, r *http.Request) {
	response := model.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
	}

	api.writeJSONResponse(w, http.StatusOK, response)
}

// handleArtifacts handles /artifacts endpoint (POST for creation, GET for listing)
func (api *HTTPAPI) handleArtifacts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		api.handleCreateArtifact(w, r)
	case http.MethodGet:
		api.handleListArtifacts(w, r)
	default:
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleArtifactWithID handles /artifacts/{id} for metadata retrieval
func (api *HTTPAPI) handleArtifactWithID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	api.handleGetArtifact(w, r)
}

// handleCreateArtifact handles artifact creation requests
func (api *HTTPAPI) handleCreateArtifact(w http.ResponseWriter, r *http.Request) {
	api.logger.Info("Creating artifact", "method", r.Method, "url", r.URL.Path)

	var req model.CreateArtifactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.logger.Error("Failed to decode request body", "error", err)
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}

	// Validate required fields
	if req.Name == "" || req.Version == "" || req.Type == "" || req.Data == "" {
		api.writeErrorResponse(w, http.StatusBadRequest, "Missing required fields: name, version, type, data")
		return
	}

	// Store artifact
	artifact, err := api.store.StoreArtifact(&req)
	if err != nil {
		api.logger.Error("Failed to store artifact", "error", err)
		api.writeErrorResponse(w, http.StatusInternalServerError, "Failed to store artifact")
		return
	}

	api.logger.Info("Artifact created successfully", "id", artifact.ID, "name", artifact.Name)
	api.writeJSONResponse(w, http.StatusCreated, artifact)
}

// handleGetArtifact handles artifact metadata retrieval requests
func (api *HTTPAPI) handleGetArtifact(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/artifacts/")
	if id == "" {
		api.writeErrorResponse(w, http.StatusBadRequest, "Missing artifact ID")
		return
	}

	api.logger.Info("Retrieving artifact metadata", "id", id)

	artifact, err := api.store.GetArtifact(id)
	if err != nil {
		api.logger.Error("Failed to retrieve artifact", "id", id, "error", err)
		api.writeErrorResponse(w, http.StatusNotFound, "Artifact not found")
		return
	}

	api.writeJSONResponse(w, http.StatusOK, artifact)
}

// handleGetArtifactBinary handles artifact binary retrieval requests
func (api *HTTPAPI) handleGetArtifactBinary(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/artifacts/"), "/binary")
	if id == "" {
		api.writeErrorResponse(w, http.StatusBadRequest, "Missing artifact ID")
		return
	}

	api.logger.Info("Retrieving artifact binary", "id", id)

	binaryData, err := api.store.GetArtifactBinary(id)
	if err != nil {
		api.logger.Error("Failed to retrieve artifact binary", "id", id, "error", err)
		api.writeErrorResponse(w, http.StatusNotFound, "Artifact binary not found")
		return
	}

	// Set appropriate headers for binary download
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.tar.zst\"", id))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(binaryData)))

	_, err = w.Write(binaryData)
	if err != nil {
		api.logger.Error("Failed to write binary data", "id", id, "error", err)
	}
}

// handleGetArtifactsForHost handles requests for artifacts associated with a host
func (api *HTTPAPI) handleGetArtifactsForHost(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	hostID := strings.TrimPrefix(path, "/artifacts/for-host/")
	if hostID == "" {
		api.writeErrorResponse(w, http.StatusBadRequest, "Missing host ID")
		return
	}

	api.logger.Info("Retrieving artifacts for host", "host_id", hostID)

	artifacts, err := api.store.GetArtifactsForHost(hostID)
	if err != nil {
		api.logger.Error("Failed to retrieve artifacts for host", "host_id", hostID, "error", err)
		api.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve artifacts")
		return
	}

	response := model.ArtifactListResponse{
		Artifacts: make([]model.Artifact, len(artifacts)),
		Total:     len(artifacts),
	}

	for i, artifact := range artifacts {
		response.Artifacts[i] = *artifact
	}

	api.writeJSONResponse(w, http.StatusOK, response)
}

// writeJSONResponse writes a JSON response
func (api *HTTPAPI) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		api.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// handleListArtifacts handles GET /artifacts for listing all artifacts
func (api *HTTPAPI) handleListArtifacts(w http.ResponseWriter, r *http.Request) {
	api.logger.Info("Listing artifacts")

	artifacts, err := api.store.ListArtifacts()
	if err != nil {
		api.logger.Error("Failed to list artifacts", "error", err)
		api.writeErrorResponse(w, http.StatusInternalServerError, "Failed to list artifacts")
		return
	}

	response := model.ArtifactListResponse{
		Artifacts: make([]model.Artifact, len(artifacts)),
		Total:     len(artifacts),
	}

	for i, artifact := range artifacts {
		response.Artifacts[i] = *artifact
	}

	api.logger.Info("Artifacts listed successfully", "count", len(artifacts))
	api.writeJSONResponse(w, http.StatusOK, response)
}

// writeErrorResponse writes an error response
func (api *HTTPAPI) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	errorResponse := map[string]interface{}{
		"error":     message,
		"timestamp": time.Now(),
	}

	api.writeJSONResponse(w, statusCode, errorResponse)
}
