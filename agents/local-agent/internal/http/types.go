package http

import (
	"aegisflux/agents/local-agent/internal/types"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status         string `json:"status"`
	HostID         string `json:"host_id"`
	Timestamp      string `json:"timestamp"`
	Uptime         string `json:"uptime"`
	Version        string `json:"version"`
	LoadedPrograms int    `json:"loaded_programs"`
}

// StatusResponse represents the status response
type StatusResponse struct {
	HostID          string           `json:"host_id"`
	Timestamp       string           `json:"timestamp"`
	Uptime          string           `json:"uptime"`
	Version         string           `json:"version"`
	AppliedArtifacts []AppliedArtifact `json:"applied_artifacts"`
	TotalPrograms   int              `json:"total_programs"`
}

// AppliedArtifact represents an applied artifact in status response
type AppliedArtifact struct {
	ArtifactID   string          `json:"artifact_id"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	Status       string          `json:"status"`
	LoadedAt     string          `json:"loaded_at"`
	TTLRemaining string          `json:"ttl_remaining"`
	TTLSeconds   int64           `json:"ttl_seconds"`
	Error        string          `json:"error,omitempty"`
	Telemetry    types.ProgramTelemetry `json:"telemetry"`
}

// MetricsResponse represents the metrics response
type MetricsResponse struct {
	HostID    string   `json:"host_id"`
	Timestamp string   `json:"timestamp"`
	Counters  Counters `json:"counters"`
	Gauges    Gauges   `json:"gauges"`
}

// Counters represents counter metrics
type Counters struct {
	TotalPrograms     int `json:"total_programs"`
	RunningPrograms   int `json:"running_programs"`
	FailedPrograms    int `json:"failed_programs"`
	RolledBackPrograms int `json:"rolled_back_programs"`
}

// Gauges represents gauge metrics
type Gauges struct {
	UptimeSeconds      float64 `json:"uptime_seconds"`
	AverageCPUPercent  float64 `json:"average_cpu_percent"`
	TotalMemoryKB      float64 `json:"total_memory_kb"`
	TotalViolations    float64 `json:"total_violations"`
	TotalErrors        float64 `json:"total_errors"`
}

// ProgramsResponse represents the programs response
type ProgramsResponse struct {
	HostID    string        `json:"host_id"`
	Timestamp string        `json:"timestamp"`
	Programs  []ProgramInfo `json:"programs"`
	Total     int           `json:"total"`
}

// ProgramInfo represents detailed program information
type ProgramInfo struct {
	ArtifactID   string                 `json:"artifact_id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Status       string                 `json:"status"`
	LoadedAt     string                 `json:"loaded_at"`
	TTL          string                 `json:"ttl"`
	TTLRemaining string                 `json:"ttl_remaining"`
	TTLSeconds   int64                  `json:"ttl_seconds"`
	Parameters   interface{}            `json:"parameters"`
	Error        string                 `json:"error,omitempty"`
	Telemetry    types.ProgramTelemetry `json:"telemetry"`
}

// RollbacksResponse represents the rollbacks response
type RollbacksResponse struct {
	HostID    string        `json:"host_id"`
	Timestamp string        `json:"timestamp"`
	Rollbacks []interface{} `json:"rollbacks"`
	Total     int           `json:"total"`
}

// TelemetryResponse represents the telemetry response
type TelemetryResponse struct {
	HostID    string                        `json:"host_id"`
	Timestamp string                        `json:"timestamp"`
	Telemetry map[string]*types.ProgramTelemetry `json:"telemetry"`
	Total     int                           `json:"total"`
}

// VersionResponse represents the version response
type VersionResponse struct {
	Version   string `json:"version"`
	HostID    string `json:"host_id"`
	Timestamp string `json:"timestamp"`
	Uptime    string `json:"uptime"`
}
