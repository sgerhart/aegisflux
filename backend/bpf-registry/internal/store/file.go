package store

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"aegisflux/backend/bpf-registry/internal/model"
	"aegisflux/backend/bpf-registry/internal/sign"
)

// FileStore manages eBPF artifacts in the filesystem
type FileStore struct {
	fsStore *FSStore
	logger  *slog.Logger
	signer  *sign.VaultSigner
}

// NewFileStore creates a new file-based artifact store
func NewFileStore(dataDir string, logger *slog.Logger) (*FileStore, error) {
	// Create FS store
	fsStore, err := NewFSStore(dataDir, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create FS store: %w", err)
	}

	// Initialize Vault signer
	signer, err := sign.NewVaultSigner(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault signer: %w", err)
	}

	return &FileStore{
		fsStore: fsStore,
		logger:  logger,
		signer:  signer,
	}, nil
}

// StoreArtifact stores an artifact and returns its ID
func (s *FileStore) StoreArtifact(req *model.CreateArtifactRequest) (*model.Artifact, error) {
	// Generate unique ID (simple timestamp-based for now)
	id := fmt.Sprintf("artifact_%d", time.Now().UnixNano())
	
	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 data: %w", err)
	}

	// Calculate checksum
	hash := sha256.Sum256(data)
	checksum := fmt.Sprintf("%x", hash)

	// Create artifact metadata
	artifact := &model.Artifact{
		ID:            id,
		Name:          req.Name,
		Version:       req.Version,
		Description:   req.Description,
		Type:          req.Type,
		Architecture:  req.Architecture,
		KernelVersion: req.KernelVersion,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Size:          int64(len(data)),
		Checksum:      checksum,
		Metadata:      req.Metadata,
		Tags:          req.Tags,
		Hosts:         []string{}, // Empty initially
	}

	// Sign with Vault
	artifact.Signature = s.signWithVault(data)

	// Convert artifact to metadata map for FSStore
	metadata := s.artifactToMetadata(artifact)

	// Store using FSStore
	err = s.fsStore.Put(id, data, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to store artifact: %w", err)
	}

	s.logger.Info("Artifact stored successfully",
		"id", id,
		"name", req.Name,
		"version", req.Version,
		"type", req.Type,
		"size", len(data),
		"checksum", checksum)

	return artifact, nil
}

// GetArtifact retrieves artifact metadata by ID
func (s *FileStore) GetArtifact(id string) (*model.Artifact, error) {
	// Get metadata and binary data using FSStore
	metadataMap, _, err := s.fsStore.Get(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact: %w", err)
	}

	// Convert metadata map to Artifact struct
	artifact, err := s.metadataToArtifact(id, metadataMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert metadata: %w", err)
	}

	return artifact, nil
}

// GetArtifactBinary retrieves the binary data for an artifact
func (s *FileStore) GetArtifactBinary(id string) ([]byte, error) {
	// Get binary data using FSStore
	_, binaryData, err := s.fsStore.Get(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact binary: %w", err)
	}

	return binaryData, nil
}

// GetArtifactsForHost retrieves artifacts associated with a specific host
func (s *FileStore) GetArtifactsForHost(hostID string) ([]*model.Artifact, error) {
	var artifacts []*model.Artifact

	// Get all artifacts using FSStore
	summaries, err := s.fsStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	for _, summary := range summaries {
		// Get full artifact metadata
		artifact, err := s.GetArtifact(summary.ID)
		if err != nil {
			s.logger.Warn("Failed to read artifact metadata", "id", summary.ID, "error", err)
			continue
		}

		// Check if artifact is associated with this host
		for _, host := range artifact.Hosts {
			if host == hostID {
				artifacts = append(artifacts, artifact)
				break
			}
		}
	}

	return artifacts, nil
}

// ListArtifacts retrieves all artifacts (for future use)
func (s *FileStore) ListArtifacts() ([]*model.Artifact, error) {
	var artifacts []*model.Artifact

	// Get all artifacts using FSStore
	summaries, err := s.fsStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	for _, summary := range summaries {
		// Get full artifact metadata
		artifact, err := s.GetArtifact(summary.ID)
		if err != nil {
			s.logger.Warn("Failed to read artifact metadata", "id", summary.ID, "error", err)
			continue
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}


// signWithVault signs artifact data with Vault
func (s *FileStore) signWithVault(data []byte) string {
	signature, err := s.signer.Sign(data)
	if err != nil {
		s.logger.Error("Failed to sign artifact with Vault", "error", err)
		// Return a fallback signature to prevent complete failure
		return fmt.Sprintf("signature_error_%d", time.Now().Unix())
	}
	
	s.logger.Info("Artifact signed with Vault", "signature_length", len(signature))
	return signature
}

// artifactToMetadata converts an Artifact struct to a metadata map
func (s *FileStore) artifactToMetadata(artifact *model.Artifact) map[string]interface{} {
	metadata := map[string]interface{}{
		"id":             artifact.ID,
		"name":           artifact.Name,
		"version":        artifact.Version,
		"description":    artifact.Description,
		"type":           artifact.Type,
		"architecture":   artifact.Architecture,
		"kernel_version": artifact.KernelVersion,
		"created_at":     artifact.CreatedAt.Format(time.RFC3339Nano),
		"updated_at":     artifact.UpdatedAt.Format(time.RFC3339Nano),
		"size":           float64(artifact.Size),
		"checksum":       artifact.Checksum,
		"signature":      artifact.Signature,
		"metadata":       artifact.Metadata,
		"tags":           artifact.Tags,
		"hosts":          artifact.Hosts,
	}
	return metadata
}

// metadataToArtifact converts a metadata map to an Artifact struct
func (s *FileStore) metadataToArtifact(id string, metadata map[string]interface{}) (*model.Artifact, error) {
	artifact := &model.Artifact{
		ID: id,
	}

	// Extract string fields
	if name, ok := metadata["name"].(string); ok {
		artifact.Name = name
	}
	if version, ok := metadata["version"].(string); ok {
		artifact.Version = version
	}
	if description, ok := metadata["description"].(string); ok {
		artifact.Description = description
	}
	if artifactType, ok := metadata["type"].(string); ok {
		artifact.Type = artifactType
	}
	if architecture, ok := metadata["architecture"].(string); ok {
		artifact.Architecture = architecture
	}
	if kernelVersion, ok := metadata["kernel_version"].(string); ok {
		artifact.KernelVersion = kernelVersion
	}
	if checksum, ok := metadata["checksum"].(string); ok {
		artifact.Checksum = checksum
	}
	if signature, ok := metadata["signature"].(string); ok {
		artifact.Signature = signature
	}

	// Extract numeric fields
	if size, ok := metadata["size"].(float64); ok {
		artifact.Size = int64(size)
	}

	// Extract time fields
	if createdAtStr, ok := metadata["created_at"].(string); ok {
		if createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
			artifact.CreatedAt = createdAt
		}
	}
	if updatedAtStr, ok := metadata["updated_at"].(string); ok {
		if updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
			artifact.UpdatedAt = updatedAt
		}
	}

	// Extract metadata map
	if metadataMap, ok := metadata["metadata"].(map[string]interface{}); ok {
		artifact.Metadata = make(map[string]string)
		for k, v := range metadataMap {
			if vStr, ok := v.(string); ok {
				artifact.Metadata[k] = vStr
			}
		}
	}

	// Extract tags array
	if tagsInterface, ok := metadata["tags"]; ok {
		if tagsArray, ok := tagsInterface.([]interface{}); ok {
			for _, tag := range tagsArray {
				if tagStr, ok := tag.(string); ok {
					artifact.Tags = append(artifact.Tags, tagStr)
				}
			}
		}
	}

	// Extract hosts array
	if hostsInterface, ok := metadata["hosts"]; ok {
		if hostsArray, ok := hostsInterface.([]interface{}); ok {
			for _, host := range hostsArray {
				if hostStr, ok := host.(string); ok {
					artifact.Hosts = append(artifact.Hosts, hostStr)
				}
			}
		}
	}

	return artifact, nil
}
