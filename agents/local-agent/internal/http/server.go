package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"aegisflux/agents/local-agent/internal/rollback"
	"aegisflux/agents/local-agent/internal/types"
)

// Server provides HTTP endpoints for observability
type Server struct {
	logger     *slog.Logger
	hostID     string
	agent      AgentInterface
	server     *http.Server
	startTime  time.Time
}

// AgentInterface defines the interface for the agent
type AgentInterface interface {
	GetLoadedPrograms() map[string]*types.LoadedProgram
	GetRollbackHistory() []rollback.RollbackEvent
	GetTelemetryData() map[string]*types.ProgramTelemetry
	GetUptime() time.Duration
	GetVersion() string
}

// NewServer creates a new HTTP server
func NewServer(logger *slog.Logger, hostID string, agent AgentInterface, port int) *Server {
	mux := http.NewServeMux()
	
	server := &Server{
		logger:    logger,
		hostID:    hostID,
		agent:     agent,
		startTime: time.Now(),
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
	}
	
	// Register routes
	server.registerRoutes(mux)
	
	return server
}

// SetAgent sets the agent interface
func (s *Server) SetAgent(agent AgentInterface) {
	s.agent = agent
}

// registerRoutes registers HTTP routes
func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/programs", s.handlePrograms)
	mux.HandleFunc("/rollbacks", s.handleRollbacks)
	mux.HandleFunc("/telemetry", s.handleTelemetry)
	mux.HandleFunc("/version", s.handleVersion)
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting HTTP server", "addr", s.server.Addr)
	
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", "error", err)
		}
	}()
	
	// Wait for context cancellation
	<-ctx.Done()
	
	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	s.logger.Info("Shutting down HTTP server")
	return s.server.Shutdown(shutdownCtx)
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// handleHealth handles /healthz endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	health := HealthResponse{
		Status:    "healthy",
		HostID:    s.hostID,
		Timestamp: time.Now().Format(time.RFC3339),
		Uptime:    s.agent.GetUptime().String(),
		Version:   s.agent.GetVersion(),
	}
	
	// Check if agent is responsive
	programs := s.agent.GetLoadedPrograms()
	health.LoadedPrograms = len(programs)
	
	// Set status based on loaded programs
	if len(programs) > 0 {
		health.Status = "running"
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(health)
}

// handleStatus handles /status endpoint
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	programs := s.agent.GetLoadedPrograms()
	appliedArtifacts := make([]AppliedArtifact, 0, len(programs))
	
	now := time.Now()
	for _, program := range programs {
		ttlRemaining := program.TTL - now.Sub(program.LoadedAt)
		if ttlRemaining < 0 {
			ttlRemaining = 0
		}
		
		appliedArtifacts = append(appliedArtifacts, AppliedArtifact{
			ArtifactID:    program.ArtifactID,
			Name:          program.Name,
			Version:       program.Version,
			Status:        string(program.Status),
			LoadedAt:      program.LoadedAt.Format(time.RFC3339),
			TTLRemaining:  ttlRemaining.String(),
			TTLSeconds:    int64(ttlRemaining.Seconds()),
			Error:         program.Error,
			Telemetry:     program.Telemetry,
		})
	}
	
	status := StatusResponse{
		HostID:          s.hostID,
		Timestamp:       time.Now().Format(time.RFC3339),
		Uptime:          s.agent.GetUptime().String(),
		Version:         s.agent.GetVersion(),
		AppliedArtifacts: appliedArtifacts,
		TotalPrograms:   len(appliedArtifacts),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// handleMetrics handles /metrics endpoint
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	programs := s.agent.GetLoadedPrograms()
	telemetryData := s.agent.GetTelemetryData()
	
	metrics := MetricsResponse{
		HostID:    s.hostID,
		Timestamp: time.Now().Format(time.RFC3339),
		Counters: Counters{
			TotalPrograms:     len(programs),
			RunningPrograms:   0,
			FailedPrograms:    0,
			RolledBackPrograms: 0,
		},
		Gauges: Gauges{
			UptimeSeconds: s.agent.GetUptime().Seconds(),
		},
	}
	
	// Count programs by status
	for _, program := range programs {
		switch program.Status {
		case types.ProgramStatusRunning:
			metrics.Counters.RunningPrograms++
		case types.ProgramStatusFailed:
			metrics.Counters.FailedPrograms++
		}
	}
	
	// Add telemetry data
	if len(telemetryData) > 0 {
		var totalCPU, totalMemory, totalViolations, totalErrors float64
		for _, telemetry := range telemetryData {
			totalCPU += telemetry.CPUPercent
			totalMemory += float64(telemetry.MemoryKB)
			totalViolations += float64(telemetry.Violations)
			totalErrors += float64(telemetry.Errors)
		}
		
		metrics.Gauges.AverageCPUPercent = totalCPU / float64(len(telemetryData))
		metrics.Gauges.TotalMemoryKB = totalMemory
		metrics.Gauges.TotalViolations = totalViolations
		metrics.Gauges.TotalErrors = totalErrors
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}

// handlePrograms handles /programs endpoint
func (s *Server) handlePrograms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	programs := s.agent.GetLoadedPrograms()
	programList := make([]ProgramInfo, 0, len(programs))
	
	now := time.Now()
	for _, program := range programs {
		ttlRemaining := program.TTL - now.Sub(program.LoadedAt)
		if ttlRemaining < 0 {
			ttlRemaining = 0
		}
		
		programList = append(programList, ProgramInfo{
			ArtifactID:    program.ArtifactID,
			Name:          program.Name,
			Version:       program.Version,
			Status:        string(program.Status),
			LoadedAt:      program.LoadedAt.Format(time.RFC3339),
			TTL:           program.TTL.String(),
			TTLRemaining:  ttlRemaining.String(),
			TTLSeconds:    int64(ttlRemaining.Seconds()),
			Parameters:    program.Parameters,
			Error:         program.Error,
			Telemetry:     program.Telemetry,
		})
	}
	
	response := ProgramsResponse{
		HostID:    s.hostID,
		Timestamp: time.Now().Format(time.RFC3339),
		Programs:  programList,
		Total:     len(programList),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleRollbacks handles /rollbacks endpoint
func (s *Server) handleRollbacks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	rollbackHistory := s.agent.GetRollbackHistory()
	
	// Convert rollback events to interface{} slice
	rollbacks := make([]interface{}, len(rollbackHistory))
	for i, event := range rollbackHistory {
		rollbacks[i] = event
	}
	
	response := RollbacksResponse{
		HostID:    s.hostID,
		Timestamp: time.Now().Format(time.RFC3339),
		Rollbacks: rollbacks,
		Total:     len(rollbackHistory),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleTelemetry handles /telemetry endpoint
func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	telemetryData := s.agent.GetTelemetryData()
	
	response := TelemetryResponse{
		HostID:    s.hostID,
		Timestamp: time.Now().Format(time.RFC3339),
		Telemetry: telemetryData,
		Total:     len(telemetryData),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleVersion handles /version endpoint
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	version := VersionResponse{
		Version:   s.agent.GetVersion(),
		HostID:    s.hostID,
		Timestamp: time.Now().Format(time.RFC3339),
		Uptime:    s.agent.GetUptime().String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(version)
}
