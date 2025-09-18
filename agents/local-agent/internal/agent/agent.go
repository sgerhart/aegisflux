package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"aegisflux/agents/local-agent/internal/bpf"
	"aegisflux/agents/local-agent/internal/config"
	"aegisflux/agents/local-agent/internal/http"
	"aegisflux/agents/local-agent/internal/registry"
	"aegisflux/agents/local-agent/internal/rollback"
	"aegisflux/agents/local-agent/internal/systemd"
	"aegisflux/agents/local-agent/internal/telemetry"
	"aegisflux/agents/local-agent/internal/types"
)

// Agent represents the local agent that polls for and loads BPF programs
type Agent struct {
	logger           *slog.Logger
	config           *config.Config
	registryClient   *registry.Client
	bpfLoader        *bpf.Loader
	telemetrySender  *telemetry.Sender
	telemetryMonitor *telemetry.TelemetryMonitor
	rollbackManager  *rollback.RollbackManager
	httpServer       *http.Server
	systemdNotifier  *systemd.Notifier
	nc               *nats.Conn
	loadedPrograms   map[string]*types.LoadedProgram
	programsMutex    sync.RWMutex
	stopChan         chan struct{}
	ttlTimers        map[string]*time.Timer
	ttlMutex         sync.RWMutex
	startTime        time.Time
	version          string
}

// New creates a new agent instance
func New(logger *slog.Logger, cfg *config.Config) (*Agent, error) {
	// Connect to NATS
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create registry client
	registryClient := registry.NewClient(logger, cfg.RegistryURL, cfg.HostID)

	// Create BPF loader
	bpfLoader := bpf.NewLoader(logger, cfg.CacheDir)

	// Create telemetry sender
	telemetrySender := telemetry.NewSender(logger, nc, cfg.TelemetrySubject, cfg.HostID)

	// Create rollback manager with configured thresholds
	rollbackThresholds := rollback.RollbackThresholds{
		MaxErrors:          cfg.RollbackMaxErrors,
		MaxViolations:      cfg.RollbackMaxViolations,
		MaxCPUPercent:      cfg.RollbackMaxCPUPercent,
		MaxLatencyMs:       cfg.RollbackMaxLatencyMs,
		MaxMemoryKB:        cfg.RollbackMaxMemoryKB,
		VerifierFailure:    cfg.RollbackVerifierFailure,
		CheckIntervalSec:   cfg.RollbackCheckInterval,
		RollbackDelaySec:   cfg.RollbackDelaySec,
	}
	rollbackManager := rollback.NewRollbackManager(logger, nc, cfg.HostID, rollbackThresholds)

	// Create telemetry monitor with same thresholds
	telemetryThresholds := telemetry.TelemetryThresholds{
		MaxErrors:          cfg.RollbackMaxErrors,
		MaxViolations:      cfg.RollbackMaxViolations,
		MaxCPUPercent:      cfg.RollbackMaxCPUPercent,
		MaxLatencyMs:       cfg.RollbackMaxLatencyMs,
		MaxMemoryKB:        cfg.RollbackMaxMemoryKB,
		VerifierFailure:    cfg.RollbackVerifierFailure,
		CheckIntervalSec:   cfg.RollbackCheckInterval,
		RollbackDelaySec:   cfg.RollbackDelaySec,
	}
	telemetryMonitor := telemetry.NewTelemetryMonitor(logger, nc, cfg.HostID, rollbackManager, telemetryThresholds)

	// Create HTTP server
	httpServer := http.NewServer(logger, cfg.HostID, nil, cfg.HTTPPort) // Will set agent interface later

	// Create systemd notifier
	systemdNotifier := systemd.NewNotifier()

	agent := &Agent{
		logger:           logger,
		config:           cfg,
		registryClient:   registryClient,
		bpfLoader:        bpfLoader,
		telemetrySender:  telemetrySender,
		telemetryMonitor: telemetryMonitor,
		rollbackManager:  rollbackManager,
		httpServer:       httpServer,
		systemdNotifier:  systemdNotifier,
		nc:               nc,
		loadedPrograms:   make(map[string]*types.LoadedProgram),
		stopChan:         make(chan struct{}),
		ttlTimers:        make(map[string]*time.Timer),
		startTime:        time.Now(),
		version:          "1.0.0", // This would come from build info
	}

	// Set agent interface for HTTP server
	httpServer.SetAgent(agent)

	return agent, nil
}

// Run starts the agent main loop
func (a *Agent) Run(ctx context.Context) error {
	a.logger.Info("Starting agent main loop")

	// Start telemetry sender
	if err := a.telemetrySender.Start(ctx); err != nil {
		return fmt.Errorf("failed to start telemetry sender: %w", err)
	}
	defer a.telemetrySender.Stop()

	// Start rollback manager
	if err := a.rollbackManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start rollback manager: %w", err)
	}
	defer a.rollbackManager.Stop()

	// Start telemetry monitor
	if err := a.telemetryMonitor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start telemetry monitor: %w", err)
	}
	defer a.telemetryMonitor.Stop()

	// Set up rollback callbacks
	a.rollbackManager.AddCallback(a.handleRollbackEvent)
	a.telemetryMonitor.AddCallback(a.handleTelemetryThreshold)

	// Start HTTP server
	go func() {
		if err := a.httpServer.Start(ctx); err != nil {
			a.logger.Error("HTTP server error", "error", err)
		}
	}()

	// Notify systemd that we're ready
	if a.systemdNotifier.IsAvailable() {
		if err := a.systemdNotifier.NotifyReady(); err != nil {
			a.logger.Warn("Failed to notify systemd ready", "error", err)
		}
		
		// Start watchdog
		a.systemdNotifier.StartWatchdog(30 * time.Second)
	}

	// Start polling loop
	pollTicker := time.NewTicker(a.config.PollInterval)
	defer pollTicker.Stop()

	// Start TTL cleanup loop
	ttlTicker := time.NewTicker(1 * time.Minute)
	defer ttlTicker.Stop()

	// Initial poll
	go a.pollRegistry(ctx)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Agent context cancelled, shutting down")
			return a.shutdown()

		case <-a.stopChan:
			a.logger.Info("Agent stop signal received, shutting down")
			return a.shutdown()

		case <-pollTicker.C:
			go a.pollRegistry(ctx)

		case <-ttlTicker.C:
			a.checkTTLExpiration()
		}
	}
}

// Stop stops the agent
func (a *Agent) Stop() {
	close(a.stopChan)
}

// pollRegistry polls the registry for new artifacts
func (a *Agent) pollRegistry(ctx context.Context) {
	a.logger.Debug("Polling registry for artifacts")

	// Check registry health
	if err := a.registryClient.HealthCheck(ctx); err != nil {
		a.logger.Error("Registry health check failed", "error", err)
		return
	}

	// Get artifacts for this host
	artifacts, err := a.registryClient.GetArtifactsForHost(ctx)
	if err != nil {
		a.logger.Error("Failed to get artifacts from registry", "error", err)
		return
	}

	a.logger.Info("Retrieved artifacts from registry", "count", len(artifacts))

	// Process each artifact
	for _, artifact := range artifacts {
		go a.processArtifact(ctx, artifact)
	}
}

// processArtifact processes a single artifact
func (a *Agent) processArtifact(ctx context.Context, artifact types.ArtifactInfo) {
	a.logger.Info("Processing artifact",
		"artifact_id", artifact.ArtifactID,
		"name", artifact.Name,
		"version", artifact.Version)

	// Check if already loaded
	a.programsMutex.RLock()
	if _, exists := a.loadedPrograms[artifact.ArtifactID]; exists {
		a.programsMutex.RUnlock()
		a.logger.Debug("Artifact already loaded, skipping", "artifact_id", artifact.ArtifactID)
		return
	}
	a.programsMutex.RUnlock()

	// Check program limit
	if a.getLoadedProgramCount() >= a.config.MaxPrograms {
		a.logger.Warn("Program limit exceeded, skipping artifact",
			"artifact_id", artifact.ArtifactID,
			"max_programs", a.config.MaxPrograms)
		return
	}

	// Download artifact
	artifactData, err := a.registryClient.DownloadArtifact(ctx, artifact.ArtifactID)
	if err != nil {
		a.logger.Error("Failed to download artifact",
			"artifact_id", artifact.ArtifactID,
			"error", err)
		return
	}

	// Verify signature
	if err := a.registryClient.VerifySignature(ctx, artifactData, artifact.Signature, a.config.PublicKey); err != nil {
		a.logger.Error("Failed to verify artifact signature",
			"artifact_id", artifact.ArtifactID,
			"error", err)
		return
	}

	// Save to cache
	if err := a.bpfLoader.SaveToCache(artifact.ArtifactID, artifactData); err != nil {
		a.logger.Warn("Failed to save artifact to cache",
			"artifact_id", artifact.ArtifactID,
			"error", err)
	}

	// Determine TTL
	ttl := a.config.DefaultTTL
	if artifact.TTLSec != nil {
		ttl = time.Duration(*artifact.TTLSec) * time.Second
	}

	// Load BPF program
	program, err := a.bpfLoader.LoadProgram(artifact.ArtifactID, artifactData, artifact.Parameters, ttl)
	if err != nil {
		a.logger.Error("Failed to load BPF program",
			"artifact_id", artifact.ArtifactID,
			"error", err)
		
		// Send error telemetry
		a.telemetrySender.SendProgramError(artifact.ArtifactID, err.Error())
		return
	}

	// Store loaded program
	a.programsMutex.Lock()
	a.loadedPrograms[artifact.ArtifactID] = program
	a.programsMutex.Unlock()

	// Register with rollback manager
	a.rollbackManager.RegisterProgram(program)

	// Set TTL timer
	a.setTTLTimer(artifact.ArtifactID, ttl)

	// Send loaded telemetry
	if err := a.telemetrySender.SendProgramLoaded(program); err != nil {
		a.logger.Warn("Failed to send program loaded telemetry",
			"artifact_id", artifact.ArtifactID,
			"error", err)
	}

	a.logger.Info("Artifact processed successfully",
		"artifact_id", artifact.ArtifactID,
		"name", program.Name,
		"version", program.Version,
		"ttl", ttl)
}

// setTTLTimer sets a timer to unload a program after TTL expires
func (a *Agent) setTTLTimer(artifactID string, ttl time.Duration) {
	a.ttlMutex.Lock()
	defer a.ttlMutex.Unlock()

	// Cancel existing timer if any
	if existingTimer, exists := a.ttlTimers[artifactID]; exists {
		existingTimer.Stop()
	}

	// Set new timer
	timer := time.AfterFunc(ttl, func() {
		a.logger.Info("TTL expired, unloading program", "artifact_id", artifactID)
		if err := a.unloadProgram(artifactID); err != nil {
			a.logger.Error("Failed to unload program after TTL",
				"artifact_id", artifactID,
				"error", err)
		}
	})

	a.ttlTimers[artifactID] = timer
}

// unloadProgram unloads a BPF program
func (a *Agent) unloadProgram(artifactID string) error {
	a.logger.Info("Unloading BPF program", "artifact_id", artifactID)

	// Remove from loaded programs
	a.programsMutex.Lock()
	_, exists := a.loadedPrograms[artifactID]
	if exists {
		delete(a.loadedPrograms, artifactID)
	}
	a.programsMutex.Unlock()

	if !exists {
		return fmt.Errorf("program not found: %s", artifactID)
	}

	// Unregister from rollback manager
	a.rollbackManager.UnregisterProgram(artifactID)

	// Unload from BPF loader
	if err := a.bpfLoader.UnloadProgram(artifactID); err != nil {
		return fmt.Errorf("failed to unload BPF program: %w", err)
	}

	// Cancel TTL timer
	a.ttlMutex.Lock()
	if timer, exists := a.ttlTimers[artifactID]; exists {
		timer.Stop()
		delete(a.ttlTimers, artifactID)
	}
	a.ttlMutex.Unlock()

	// Send unloaded telemetry
	if err := a.telemetrySender.SendProgramUnloaded(artifactID); err != nil {
		a.logger.Warn("Failed to send program unloaded telemetry",
			"artifact_id", artifactID,
			"error", err)
	}

	a.logger.Info("BPF program unloaded successfully", "artifact_id", artifactID)
	return nil
}

// checkTTLExpiration checks for programs that should be unloaded due to TTL
func (a *Agent) checkTTLExpiration() {
	a.programsMutex.RLock()
	programs := make([]*types.LoadedProgram, 0, len(a.loadedPrograms))
	for _, program := range a.loadedPrograms {
		programs = append(programs, program)
	}
	a.programsMutex.RUnlock()

	now := time.Now()
	for _, program := range programs {
		if now.Sub(program.LoadedAt) > program.TTL {
			a.logger.Info("Program TTL expired, scheduling unload",
				"artifact_id", program.ArtifactID,
				"loaded_at", program.LoadedAt,
				"ttl", program.TTL)
			
			go func(artifactID string) {
				if err := a.unloadProgram(artifactID); err != nil {
					a.logger.Error("Failed to unload expired program",
						"artifact_id", artifactID,
						"error", err)
				}
			}(program.ArtifactID)
		}
	}
}

// getLoadedProgramCount returns the number of loaded programs
func (a *Agent) getLoadedProgramCount() int {
	a.programsMutex.RLock()
	defer a.programsMutex.RUnlock()
	return len(a.loadedPrograms)
}

// GetLoadedPrograms returns all loaded programs
func (a *Agent) GetLoadedPrograms() map[string]*types.LoadedProgram {
	a.programsMutex.RLock()
	defer a.programsMutex.RUnlock()
	
	programs := make(map[string]*types.LoadedProgram)
	for id, program := range a.loadedPrograms {
		programs[id] = program
	}
	return programs
}

// GetProgram returns a specific loaded program
func (a *Agent) GetProgram(artifactID string) (*types.LoadedProgram, bool) {
	a.programsMutex.RLock()
	defer a.programsMutex.RUnlock()
	
	program, exists := a.loadedPrograms[artifactID]
	return program, exists
}

// shutdown gracefully shuts down the agent
func (a *Agent) shutdown() error {
	a.logger.Info("Shutting down agent")

	// Notify systemd that we're stopping
	if a.systemdNotifier.IsAvailable() {
		if err := a.systemdNotifier.NotifyStopping(); err != nil {
			a.logger.Warn("Failed to notify systemd stopping", "error", err)
		}
	}

	// Stop HTTP server
	if err := a.httpServer.Stop(); err != nil {
		a.logger.Error("Failed to stop HTTP server", "error", err)
	}

	// Unload all programs
	a.programsMutex.Lock()
	programs := make([]string, 0, len(a.loadedPrograms))
	for artifactID := range a.loadedPrograms {
		programs = append(programs, artifactID)
	}
	a.programsMutex.Unlock()

	for _, artifactID := range programs {
		if err := a.unloadProgram(artifactID); err != nil {
			a.logger.Error("Failed to unload program during shutdown",
				"artifact_id", artifactID,
				"error", err)
		}
	}

	// Close systemd connection
	if err := a.systemdNotifier.Close(); err != nil {
		a.logger.Warn("Failed to close systemd connection", "error", err)
	}

	// Close NATS connection
	a.nc.Close()

	a.logger.Info("Agent shutdown complete")
	return nil
}

// SendTelemetry sends telemetry for all loaded programs
func (a *Agent) SendTelemetry() {
	a.programsMutex.RLock()
	programs := make([]*types.LoadedProgram, 0, len(a.loadedPrograms))
	for _, program := range a.loadedPrograms {
		programs = append(programs, program)
	}
	a.programsMutex.RUnlock()

	for _, program := range programs {
		if err := a.telemetrySender.SendTelemetry(program); err != nil {
			a.logger.Warn("Failed to send telemetry",
				"artifact_id", program.ArtifactID,
				"error", err)
		}
	}
}

// handleRollbackEvent handles rollback events from the rollback manager
func (a *Agent) handleRollbackEvent(event rollback.RollbackEvent, program *types.LoadedProgram) {
	a.logger.Info("Handling rollback event",
		"artifact_id", event.ArtifactID,
		"reason", event.Reason,
		"threshold", event.Threshold)

	// Unload the program
	if err := a.unloadProgram(event.ArtifactID); err != nil {
		a.logger.Error("Failed to unload program during rollback",
			"artifact_id", event.ArtifactID,
			"error", err)
		
		// Send error telemetry
		a.telemetrySender.SendProgramError(event.ArtifactID, err.Error())
		return
	}

	// Send rollback telemetry
	if err := a.telemetrySender.SendProgramRolledBack(
		event.ArtifactID,
		string(event.Reason),
		event.Threshold,
		event.Value,
	); err != nil {
		a.logger.Warn("Failed to send rollback telemetry",
			"artifact_id", event.ArtifactID,
			"error", err)
	}

	a.logger.Info("Program rollback completed",
		"artifact_id", event.ArtifactID,
		"reason", event.Reason)
}

// handleTelemetryThreshold handles telemetry threshold violations
func (a *Agent) handleTelemetryThreshold(artifactID string, telemetry *types.ProgramTelemetry, threshold string, value interface{}) {
	a.logger.Warn("Telemetry threshold exceeded",
		"artifact_id", artifactID,
		"threshold", threshold,
		"value", value,
		"errors", telemetry.Errors,
		"violations", telemetry.Violations,
		"cpu_percent", telemetry.CPUPercent,
		"latency_ms", telemetry.LatencyMs,
		"memory_kb", telemetry.MemoryKB)

	// Update telemetry data
	a.telemetryMonitor.UpdateTelemetry(artifactID, telemetry)
}

// RequestRollback manually requests a rollback for a specific program
func (a *Agent) RequestRollback(artifactID string, reason string) error {
	rollbackReason := rollback.RollbackReasonManual
	switch reason {
	case "telemetry_threshold":
		rollbackReason = rollback.RollbackReasonTelemetryThreshold
	case "verifier_failure":
		rollbackReason = rollback.RollbackReasonVerifierFailure
	case "high_cpu":
		rollbackReason = rollback.RollbackReasonHighCPU
	case "orchestrator_signal":
		rollbackReason = rollback.RollbackReasonOrchestratorSignal
	case "ttl_expired":
		rollbackReason = rollback.RollbackReasonTTLExpired
	}

	return a.rollbackManager.RequestRollback(artifactID, rollbackReason, map[string]interface{}{
		"manual_request": true,
		"reason":         reason,
	})
}

// UpdateRollbackThresholds updates rollback thresholds
func (a *Agent) UpdateRollbackThresholds(thresholds rollback.RollbackThresholds) {
	a.rollbackManager.UpdateThresholds(thresholds)
}

// GetRollbackHistory returns rollback history
func (a *Agent) GetRollbackHistory() []rollback.RollbackEvent {
	return a.rollbackManager.GetRollbackHistory()
}

// AgentInterface implementation

// GetUptime returns the agent uptime
func (a *Agent) GetUptime() time.Duration {
	return time.Since(a.startTime)
}

// GetVersion returns the agent version
func (a *Agent) GetVersion() string {
	return a.version
}

// GetTelemetryData returns current telemetry data
func (a *Agent) GetTelemetryData() map[string]*types.ProgramTelemetry {
	return a.telemetryMonitor.GetMonitoringData()
}
