package health

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Checker interface for health checks
type Checker interface {
	IsHealthy() bool
	IsReady() bool
}

// HealthServer handles health check endpoints
type HealthServer struct {
	checker Checker
	logger  *slog.Logger
}

// NewHealthServer creates a new health server
func NewHealthServer(checker Checker, logger *slog.Logger) *HealthServer {
	return &HealthServer{
		checker: checker,
		logger:  logger,
	}
}

// HealthResponse represents the response structure for health endpoints
type HealthResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// healthzHandler handles GET /healthz
// Returns 200 {"ok":true} if both gRPC and NATS client are up
func (h *HealthServer) healthzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	healthy := h.checker.IsHealthy()
	
	w.Header().Set("Content-Type", "application/json")
	
	if healthy {
		w.WriteHeader(http.StatusOK)
		response := HealthResponse{OK: true}
		json.NewEncoder(w).Encode(response)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		response := HealthResponse{OK: false, Message: "Service unhealthy"}
		json.NewEncoder(w).Encode(response)
	}
}

// readyzHandler handles GET /readyz
// Returns 200 once NATS is connected and schema is compiled
func (h *HealthServer) readyzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ready := h.checker.IsReady()
	
	w.Header().Set("Content-Type", "application/json")
	
	if ready {
		w.WriteHeader(http.StatusOK)
		response := HealthResponse{OK: true}
		json.NewEncoder(w).Encode(response)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		response := HealthResponse{OK: false, Message: "Service not ready"}
		json.NewEncoder(w).Encode(response)
	}
}


// RegisterRoutes registers all health check routes
func (h *HealthServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.healthzHandler)
	mux.HandleFunc("/readyz", h.readyzHandler)
	// Note: /metrics is handled by promhttp.Handler() in main.go
}
