package store

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"aegisflux/backend/bpf-registry/internal/model"
)

// FSStore provides a clean file system interface for artifact storage
type FSStore struct {
	dataDir string
	logger  *slog.Logger
}

// NewFSStore creates a new file system store
func NewFSStore(dataDir string, logger *slog.Logger) (*FSStore, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &FSStore{
		dataDir: dataDir,
		logger:  logger,
	}, nil
}

// Put stores an artifact with the given ID, tar.zst bytes, and metadata
func (fs *FSStore) Put(id string, tarZstBytes []byte, metadata map[string]interface{}) error {
	fs.logger.Info("Storing artifact", "id", id, "size", len(tarZstBytes))

	// Create artifact directory
	artifactDir := filepath.Join(fs.dataDir, id)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("failed to create artifact directory: %w", err)
	}

	// Store binary data as tar.zst file
	binaryPath := filepath.Join(artifactDir, "artifact.tar.zst")
	if err := fs.writeFile(binaryPath, tarZstBytes); err != nil {
		return fmt.Errorf("failed to write binary data: %w", err)
	}

	// Store metadata
	metadataPath := filepath.Join(artifactDir, "metadata.json")
	if err := fs.writeMetadata(metadataPath, metadata); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	fs.logger.Info("Artifact stored successfully", "id", id, "binary_size", len(tarZstBytes))
	return nil
}

// Get retrieves an artifact by ID, returning metadata and tar.zst bytes
func (fs *FSStore) Get(id string) (map[string]interface{}, []byte, error) {
	fs.logger.Info("Retrieving artifact", "id", id)

	// Read metadata
	metadata, err := fs.getMetadata(id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Read binary data
	binaryData, err := fs.getBinary(id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read binary data: %w", err)
	}

	fs.logger.Info("Artifact retrieved successfully", "id", id, "binary_size", len(binaryData))
	return metadata, binaryData, nil
}

// List returns a summary of all artifacts
func (fs *FSStore) List() ([]model.ArtifactSummary, error) {
	fs.logger.Info("Listing artifacts")

	var summaries []model.ArtifactSummary

	// Read all artifact directories
	entries, err := os.ReadDir(fs.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Read metadata for each artifact
		metadata, err := fs.getMetadata(entry.Name())
		if err != nil {
			fs.logger.Warn("Failed to read artifact metadata", "id", entry.Name(), "error", err)
			continue
		}

		// Convert metadata to ArtifactSummary
		summary, err := fs.metadataToSummary(entry.Name(), metadata)
		if err != nil {
			fs.logger.Warn("Failed to convert metadata to summary", "id", entry.Name(), "error", err)
			continue
		}

		summaries = append(summaries, *summary)
	}

	fs.logger.Info("Artifact listing completed", "count", len(summaries))
	return summaries, nil
}

// getMetadata reads metadata for an artifact
func (fs *FSStore) getMetadata(id string) (map[string]interface{}, error) {
	metadataPath := filepath.Join(fs.dataDir, id, "metadata.json")
	
	// Check if metadata file exists
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("artifact not found: %s", id)
	}

	// Read and parse metadata
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return metadata, nil
}

// getBinary reads binary data for an artifact
func (fs *FSStore) getBinary(id string) ([]byte, error) {
	binaryPath := filepath.Join(fs.dataDir, id, "artifact.tar.zst")
	
	// Check if binary file exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("artifact binary not found: %s", id)
	}

	// Read binary data
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read binary file: %w", err)
	}

	return data, nil
}

// metadataToSummary converts metadata map to ArtifactSummary
func (fs *FSStore) metadataToSummary(id string, metadata map[string]interface{}) (*model.ArtifactSummary, error) {
	summary := &model.ArtifactSummary{
		ID: id,
	}

	// Extract string fields
	if name, ok := metadata["name"].(string); ok {
		summary.Name = name
	}
	if version, ok := metadata["version"].(string); ok {
		summary.Version = version
	}
	if description, ok := metadata["description"].(string); ok {
		summary.Description = description
	}
	if artifactType, ok := metadata["type"].(string); ok {
		summary.Type = artifactType
	}
	if architecture, ok := metadata["architecture"].(string); ok {
		summary.Architecture = architecture
	}
	if kernelVersion, ok := metadata["kernel_version"].(string); ok {
		summary.KernelVersion = kernelVersion
	}
	if checksum, ok := metadata["checksum"].(string); ok {
		summary.Checksum = checksum
	}

	// Extract numeric fields
	if size, ok := metadata["size"].(float64); ok {
		summary.Size = int64(size)
	}

	// Extract time fields
	if createdAtStr, ok := metadata["created_at"].(string); ok {
		if createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
			summary.CreatedAt = createdAt
		}
	}
	if updatedAtStr, ok := metadata["updated_at"].(string); ok {
		if updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
			summary.UpdatedAt = updatedAt
		}
	}

	// Extract tags array
	if tagsInterface, ok := metadata["tags"]; ok {
		if tagsArray, ok := tagsInterface.([]interface{}); ok {
			for _, tag := range tagsArray {
				if tagStr, ok := tag.(string); ok {
					summary.Tags = append(summary.Tags, tagStr)
				}
			}
		}
	}

	return summary, nil
}

// writeFile writes data to a file
func (fs *FSStore) writeFile(path string, data []byte) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

// writeMetadata writes metadata to a JSON file
func (fs *FSStore) writeMetadata(path string, metadata map[string]interface{}) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return fs.writeFile(path, data)
}
