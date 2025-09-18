package model

import (
	"time"
)

// Artifact represents an eBPF artifact stored in the registry
type Artifact struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Type        string            `json:"type"` // e.g., "program", "map", "tracepoint"
	Architecture string           `json:"architecture"` // e.g., "x86_64", "arm64"
	KernelVersion string          `json:"kernel_version"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Size        int64             `json:"size"`
	Checksum    string            `json:"checksum"`
	Signature   string            `json:"signature,omitempty"` // Vault signature
	Metadata    map[string]string `json:"metadata,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Hosts       []string          `json:"hosts,omitempty"` // Host IDs this artifact is deployed to
}

// CreateArtifactRequest represents a request to create a new artifact
type CreateArtifactRequest struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Type         string            `json:"type"`
	Architecture string            `json:"architecture"`
	KernelVersion string           `json:"kernel_version"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Data         string            `json:"data"` // Base64 encoded artifact data
}

// ArtifactListResponse represents a response containing a list of artifacts
type ArtifactListResponse struct {
	Artifacts []Artifact `json:"artifacts"`
	Total     int        `json:"total"`
}

// ArtifactSummary represents a lightweight summary of an artifact for listing
type ArtifactSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Architecture string   `json:"architecture"`
	KernelVersion string  `json:"kernel_version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Size        int64     `json:"size"`
	Checksum    string    `json:"checksum"`
	Tags        []string  `json:"tags,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}
