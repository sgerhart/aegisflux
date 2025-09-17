package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/nats-io/nats.go"

	"aegisflux/backend/config-api/internal/store"
)

// Handler handles HTTP requests for the config API
type Handler struct {
	store   *store.PostgresStore
	nats    *nats.Conn
	logger  *slog.Logger
}

// NewHandler creates a new API handler
func NewHandler(store *store.PostgresStore, nats *nats.Conn, logger *slog.Logger) *Handler {
	return &Handler{
		store:  store,
		nats:   nats,
		logger: logger,
	}
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle CORS preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Route requests
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	switch {
	case path == "healthz":
		h.handleHealthz(w, r)
	case path == "readyz":
		h.handleReadyz(w, r)
	case path == "config" && r.Method == "GET":
		h.handleGetAllConfigs(w, r)
	case len(parts) == 2 && parts[0] == "config" && r.Method == "GET":
		h.handleGetConfig(w, r, parts[1])
	case len(parts) == 2 && parts[0] == "config" && r.Method == "PUT":
		h.handleSetConfig(w, r, parts[1])
	default:
		http.NotFound(w, r)
	}
}

// handleHealthz handles health check requests
func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"service":   "config-api",
		"status":    "healthy",
		"timestamp": fmt.Sprintf("%d", r.Context().Value("timestamp")),
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleReadyz handles readiness check requests
func (h *Handler) handleReadyz(w http.ResponseWriter, r *http.Request) {
	// Check database connectivity
	if err := h.store.Health(); err != nil {
		h.logger.Error("Readiness check failed - database not accessible", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "config-api",
			"status":  "not ready",
			"error":   "database not accessible",
		})
		return
	}

	// Check NATS connectivity
	if !h.nats.IsConnected() {
		h.logger.Error("Readiness check failed - NATS not connected")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "config-api",
			"status":  "not ready",
			"error":   "NATS not connected",
		})
		return
	}

	response := map[string]interface{}{
		"service":   "config-api",
		"status":    "ready",
		"timestamp": fmt.Sprintf("%d", r.Context().Value("timestamp")),
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetAllConfigs handles GET /config requests
func (h *Handler) handleGetAllConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.store.GetAllConfigs()
	if err != nil {
		h.logger.Error("Failed to get all configs", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to retrieve configurations"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"configs": configs,
		"count":   len(configs),
	})
}

// handleGetConfig handles GET /config/{key} requests
func (h *Handler) handleGetConfig(w http.ResponseWriter, r *http.Request, key string) {
	config, err := h.store.GetConfig(key)
	if err != nil {
		h.logger.Error("Failed to get config", "key", key, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to retrieve configuration"})
		return
	}

	if config == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Configuration not found"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(config)
}

// SetConfigRequest represents the request body for setting a configuration
type SetConfigRequest struct {
	Value     json.RawMessage `json:"value"`
	Scope     string          `json:"scope,omitempty"`
	UpdatedBy string          `json:"updated_by,omitempty"`
}

// handleSetConfig handles PUT /config/{key} requests
func (h *Handler) handleSetConfig(w http.ResponseWriter, r *http.Request, key string) {
	var req SetConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON in request body"})
		return
	}

	// Set defaults
	if req.Scope == "" {
		req.Scope = "global"
	}
	if req.UpdatedBy == "" {
		req.UpdatedBy = "api"
	}

	// Validate value is valid JSON
	if !json.Valid(req.Value) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON value"})
		return
	}

	// Save to database
	config, err := h.store.SetConfig(key, req.Value, req.Scope, req.UpdatedBy)
	if err != nil {
		h.logger.Error("Failed to set config", "key", key, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save configuration"})
		return
	}

	// Publish to NATS
	configChange := map[string]interface{}{
		"key":        key,
		"value":      req.Value,
		"scope":      req.Scope,
		"updated_by": req.UpdatedBy,
		"timestamp":  config.UpdatedAt.Unix(),
	}

	configChangeJSON, err := json.Marshal(configChange)
	if err != nil {
		h.logger.Error("Failed to marshal config change", "error", err)
	} else {
		if err := h.nats.Publish("config.changed", configChangeJSON); err != nil {
			h.logger.Error("Failed to publish config change", "error", err)
		} else {
			h.logger.Info("Published config change", "key", key, "subject", "config.changed")
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(config)
}
