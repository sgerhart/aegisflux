package types

import (
	"encoding/json"
	"time"
)

// ArtifactInfo represents artifact information from the registry
type ArtifactInfo struct {
	ArtifactID  string          `json:"artifact_id"`
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description,omitempty"`
	CreatedAt   string          `json:"created_at"`
	Size        int64           `json:"size"`
	Checksum    string          `json:"checksum"`
	Signature   string          `json:"signature"`
	Parameters  json.RawMessage `json:"parameters"`
	TTLSec      *int64          `json:"ttl_sec,omitempty"`
}

// RegistryResponse represents the response from the registry API
type RegistryResponse struct {
	Artifacts  []ArtifactInfo `json:"artifacts"`
	Total      int64          `json:"total"`
	NextPoll   *string        `json:"next_poll,omitempty"`
}

// ProgramStatus represents the status of a loaded BPF program
type ProgramStatus string

const (
	ProgramStatusLoading  ProgramStatus = "loading"
	ProgramStatusLoaded   ProgramStatus = "loaded"
	ProgramStatusRunning  ProgramStatus = "running"
	ProgramStatusFailed   ProgramStatus = "failed"
	ProgramStatusUnloading ProgramStatus = "unloading"
	ProgramStatusUnloaded ProgramStatus = "unloaded"
)

// LoadedProgram represents a loaded BPF program
type LoadedProgram struct {
	ArtifactID  string          `json:"artifact_id"`
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Status      ProgramStatus   `json:"status"`
	LoadedAt    time.Time       `json:"loaded_at"`
	TTL         time.Duration   `json:"ttl"`
	Parameters  json.RawMessage `json:"parameters"`
	Telemetry   ProgramTelemetry `json:"telemetry"`
	Error       string          `json:"error,omitempty"`
}

// ProgramTelemetry represents telemetry data for a program
type ProgramTelemetry struct {
	ArtifactID       string  `json:"artifact_id"`
	HostID           string  `json:"host_id"`
	Timestamp        string  `json:"timestamp"`
	Status           string  `json:"status"`
	VerifierMsg      *string `json:"verifier_msg,omitempty"`
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryKB         int64   `json:"memory_kb"`
	PacketsProcessed int64   `json:"packets_processed"`
	Violations       int64   `json:"violations"`
	Errors           int64   `json:"errors"`
	LatencyMs        float64 `json:"latency_ms"`
}

// TelemetryEvent represents a telemetry event sent to NATS
type TelemetryEvent struct {
	Type      string            `json:"type"`
	Timestamp string            `json:"timestamp"`
	Data      ProgramTelemetry  `json:"data"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// BPFProgram represents a BPF program that can be loaded
type BPFProgram struct {
	ArtifactID  string          `json:"artifact_id"`
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	ObjectData  []byte          `json:"-"` // BPF object data (not serialized)
	Parameters  json.RawMessage `json:"parameters"`
	TTL         time.Duration   `json:"ttl"`
	LoadedAt    time.Time       `json:"loaded_at"`
	Status      ProgramStatus   `json:"status"`
	Error       string          `json:"error,omitempty"`
}

// VerificationResult represents the result of artifact verification
type VerificationResult struct {
	Valid      bool   `json:"valid"`
	Error      string `json:"error,omitempty"`
	VerifiedAt time.Time `json:"verified_at"`
}

// DownloadResult represents the result of artifact download
type DownloadResult struct {
	ArtifactID string    `json:"artifact_id"`
	Data       []byte    `json:"-"`
	Size       int64     `json:"size"`
	DownloadedAt time.Time `json:"downloaded_at"`
	Error      string    `json:"error,omitempty"`
}
