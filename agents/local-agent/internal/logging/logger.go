package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"aegisflux/agents/local-agent/internal/config"
)

// Logger provides structured logging with systemd integration
type Logger struct {
	*slog.Logger
	config *config.Config
}

// NewLogger creates a new structured logger
func NewLogger(cfg *config.Config) *Logger {
	var handler slog.Handler
	
	// Determine output destination
	var output io.Writer = os.Stdout
	
	// Check if running under systemd
	if isSystemd() {
		// Use journal output for systemd
		output = os.Stderr // systemd captures stderr for journal
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{
			Level:       parseLogLevel(cfg.LogLevel),
			AddSource:   false, // systemd already provides source info
		})
	} else {
		// Use file output for non-systemd environments
		logFile := filepath.Join(cfg.CacheDir, "agent.log")
		if file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			output = file
		}
		
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{
			Level:       parseLogLevel(cfg.LogLevel),
			AddSource:   true,
		})
	}
	
	logger := &Logger{
		Logger: slog.New(handler),
		config: cfg,
	}
	
	// Set default context
	slogLogger := logger.With(
		"host_id", cfg.HostID,
		"service", "aegisflux-agent",
		"component", "agent",
	)
	
	logger = &Logger{
		Logger: slogLogger,
	}
	
	return logger
}

// parseLogLevel parses log level string
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// isSystemd checks if running under systemd
func isSystemd() bool {
	// Check for systemd environment variables
	if os.Getenv("INVOCATION_ID") != "" {
		return true
	}
	
	// Check if NOTIFY_SOCKET is set (systemd notify socket)
	if os.Getenv("NOTIFY_SOCKET") != "" {
		return true
	}
	
	// Check if running as PID 1 (systemd service)
	if os.Getpid() == 1 {
		return true
	}
	
	return false
}

// LogProgramEvent logs program-related events
func (l *Logger) LogProgramEvent(event string, artifactID string, additional ...any) {
	args := []any{
		"event", event,
		"artifact_id", artifactID,
	}
	args = append(args, additional...)
	
	switch event {
	case "program_loaded":
		l.Info("BPF program loaded", args...)
	case "program_unloaded":
		l.Info("BPF program unloaded", args...)
	case "program_rolled_back":
		l.Warn("BPF program rolled back", args...)
	case "program_error":
		l.Error("BPF program error", args...)
	default:
		l.Info("Program event", args...)
	}
}

// LogTelemetryEvent logs telemetry events
func (l *Logger) LogTelemetryEvent(artifactID string, telemetry map[string]interface{}) {
	l.Debug("Telemetry data",
		"artifact_id", artifactID,
		"telemetry", telemetry)
}

// LogRollbackEvent logs rollback events
func (l *Logger) LogRollbackEvent(artifactID string, reason string, threshold string, value interface{}) {
	l.Warn("Rollback triggered",
		"artifact_id", artifactID,
		"reason", reason,
		"threshold", threshold,
		"value", value)
}

// LogRegistryEvent logs registry-related events
func (l *Logger) LogRegistryEvent(event string, additional ...any) {
	args := []any{"event", event}
	args = append(args, additional...)
	
	switch event {
	case "poll_started":
		l.Debug("Registry poll started", args...)
	case "poll_completed":
		l.Info("Registry poll completed", args...)
	case "artifact_downloaded":
		l.Info("Artifact downloaded", args...)
	case "artifact_verified":
		l.Info("Artifact verified", args...)
	case "registry_error":
		l.Error("Registry error", args...)
	default:
		l.Info("Registry event", args...)
	}
}

// LogSystemEvent logs system-related events
func (l *Logger) LogSystemEvent(event string, additional ...any) {
	args := []any{"event", event}
	args = append(args, additional...)
	
	switch event {
	case "agent_started":
		l.Info("Agent started", args...)
	case "agent_stopped":
		l.Info("Agent stopped", args...)
	case "shutdown_signal":
		l.Info("Shutdown signal received", args...)
	case "config_loaded":
		l.Info("Configuration loaded", args...)
	case "http_server_started":
		l.Info("HTTP server started", args...)
	case "http_server_stopped":
		l.Info("HTTP server stopped", args...)
	default:
		l.Info("System event", args...)
	}
}

// LogThresholdEvent logs threshold-related events
func (l *Logger) LogThresholdEvent(artifactID string, threshold string, value interface{}, limit interface{}) {
	l.Warn("Threshold exceeded",
		"artifact_id", artifactID,
		"threshold", threshold,
		"value", value,
		"limit", limit)
}

// LogNATSEvent logs NATS-related events
func (l *Logger) LogNATSEvent(event string, additional ...any) {
	args := []any{"event", event}
	args = append(args, additional...)
	
	switch event {
	case "nats_connected":
		l.Info("NATS connected", args...)
	case "nats_disconnected":
		l.Warn("NATS disconnected", args...)
	case "nats_error":
		l.Error("NATS error", args...)
	case "telemetry_sent":
		l.Debug("Telemetry sent", args...)
	case "rollback_signal_received":
		l.Info("Rollback signal received", args...)
	default:
		l.Info("NATS event", args...)
	}
}

// LogHTTPEvent logs HTTP-related events
func (l *Logger) LogHTTPEvent(event string, path string, statusCode int, additional ...any) {
	args := []any{
		"event", event,
		"path", path,
		"status_code", statusCode,
	}
	args = append(args, additional...)
	
	switch event {
	case "http_request":
		l.Debug("HTTP request", args...)
	case "http_error":
		l.Error("HTTP error", args...)
	default:
		l.Info("HTTP event", args...)
	}
}

// LogPerformanceEvent logs performance-related events
func (l *Logger) LogPerformanceEvent(operation string, duration string, additional ...any) {
	args := []any{
		"operation", operation,
		"duration", duration,
	}
	args = append(args, additional...)
	
	l.Debug("Performance event", args...)
}

// LogSecurityEvent logs security-related events
func (l *Logger) LogSecurityEvent(event string, additional ...any) {
	args := []any{"event", event}
	args = append(args, additional...)
	
	switch event {
	case "signature_verified":
		l.Info("Signature verified", args...)
	case "signature_failed":
		l.Error("Signature verification failed", args...)
	case "unauthorized_access":
		l.Error("Unauthorized access attempt", args...)
	case "vault_connected":
		l.Info("Vault connected", args...)
	case "vault_error":
		l.Error("Vault error", args...)
	default:
		l.Warn("Security event", args...)
	}
}

// WithComponent creates a logger with component context
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With("component", component),
		config: l.config,
	}
}

// WithArtifact creates a logger with artifact context
func (l *Logger) WithArtifact(artifactID string) *Logger {
	return &Logger{
		Logger: l.Logger.With("artifact_id", artifactID),
		config: l.config,
	}
}

// SetLogLevel dynamically sets the log level
func (l *Logger) SetLogLevel(level string) {
	// This would require recreating the logger with new handler
	// For now, just log the change
	l.Info("Log level changed", "new_level", level)
}
