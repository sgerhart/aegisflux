package bpf

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/klauspost/compress/zstd"

	"aegisflux/agents/local-agent/internal/types"
)

// Loader handles loading and managing BPF programs
type Loader struct {
	logger    *slog.Logger
	cacheDir  string
	programs  map[string]*types.LoadedProgram
}

// NewLoader creates a new BPF loader
func NewLoader(logger *slog.Logger, cacheDir string) *Loader {
	return &Loader{
		logger:   logger,
		cacheDir: cacheDir,
		programs: make(map[string]*types.LoadedProgram),
	}
}

// LoadProgram loads a BPF program from artifact data
func (l *Loader) LoadProgram(artifactID string, artifactData []byte, parameters json.RawMessage, ttl time.Duration) (*types.LoadedProgram, error) {
	l.logger.Info("Loading BPF program",
		"artifact_id", artifactID,
		"data_size", len(artifactData),
		"ttl", ttl)

	// Extract artifact data
	extractedData, err := l.extractArtifact(artifactData)
	if err != nil {
		return nil, fmt.Errorf("failed to extract artifact: %w", err)
	}

	// Load BPF object
	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(extractedData.ObjectData))
	if err != nil {
		return nil, fmt.Errorf("failed to load BPF collection spec: %w", err)
	}

	// Create collection
	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create BPF collection: %w", err)
	}
	defer coll.Close()

	// Create loaded program
	program := &types.LoadedProgram{
		ArtifactID: artifactID,
		Name:       extractedData.Name,
		Version:    extractedData.Version,
		Status:     types.ProgramStatusLoaded,
		LoadedAt:   time.Now(),
		TTL:        ttl,
		Parameters: parameters,
		Telemetry: types.ProgramTelemetry{
			ArtifactID:       artifactID,
			HostID:           os.Getenv("AGENT_HOST_ID"),
			Timestamp:        time.Now().Format(time.RFC3339),
			Status:           string(types.ProgramStatusLoaded),
			VerifierMsg:      nil,
			CPUPercent:       0.0,
			MemoryKB:         0,
			PacketsProcessed: 0,
			Violations:       0,
			Errors:           0,
			LatencyMs:        0.0,
		},
	}

	// Apply parameters to maps
	if err := l.applyParameters(coll, parameters); err != nil {
		program.Status = types.ProgramStatusFailed
		program.Error = fmt.Sprintf("failed to apply parameters: %v", err)
		l.logger.Error("Failed to apply parameters",
			"artifact_id", artifactID,
			"error", err)
		return program, err
	}

	// Attach programs
	if err := l.attachPrograms(coll); err != nil {
		program.Status = types.ProgramStatusFailed
		program.Error = fmt.Sprintf("failed to attach programs: %v", err)
		l.logger.Error("Failed to attach programs",
			"artifact_id", artifactID,
			"error", err)
		return program, err
	}

	program.Status = types.ProgramStatusRunning
	l.programs[artifactID] = program

	l.logger.Info("BPF program loaded successfully",
		"artifact_id", artifactID,
		"name", program.Name,
		"version", program.Version)

	return program, nil
}

// UnloadProgram unloads a BPF program
func (l *Loader) UnloadProgram(artifactID string) error {
	l.logger.Info("Unloading BPF program", "artifact_id", artifactID)

	program, exists := l.programs[artifactID]
	if !exists {
		return fmt.Errorf("program not found: %s", artifactID)
	}

	program.Status = types.ProgramStatusUnloading

	// In a real implementation, you would:
	// 1. Detach all links
	// 2. Close the collection
	// 3. Clean up resources

	program.Status = types.ProgramStatusUnloaded
	delete(l.programs, artifactID)

	l.logger.Info("BPF program unloaded successfully", "artifact_id", artifactID)
	return nil
}

// GetLoadedPrograms returns all loaded programs
func (l *Loader) GetLoadedPrograms() map[string]*types.LoadedProgram {
	return l.programs
}

// GetProgram returns a specific loaded program
func (l *Loader) GetProgram(artifactID string) (*types.LoadedProgram, bool) {
	program, exists := l.programs[artifactID]
	return program, exists
}

// extractArtifact extracts BPF object data from artifact tar.zst
func (l *Loader) extractArtifact(artifactData []byte) (*ExtractedArtifact, error) {
	// Create zstd reader
	zstdReader, err := zstd.NewReader(bytes.NewReader(artifactData))
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer zstdReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(zstdReader)

	result := &ExtractedArtifact{}

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		switch header.Name {
		case "metadata.json":
			metadata, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read metadata: %w", err)
			}

			var meta map[string]interface{}
			if err := json.Unmarshal(metadata, &meta); err != nil {
				return nil, fmt.Errorf("failed to parse metadata: %w", err)
			}

			result.Name = meta["name"].(string)
			result.Version = meta["version"].(string)

		case "program.o":
			objectData, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read BPF object: %w", err)
			}
			result.ObjectData = objectData

		default:
			l.logger.Debug("Skipping file in artifact", "filename", header.Name)
		}
	}

	if result.ObjectData == nil {
		return nil, fmt.Errorf("no BPF object found in artifact")
	}

	return result, nil
}

// applyParameters applies parameters to BPF maps
func (l *Loader) applyParameters(coll *ebpf.Collection, parameters json.RawMessage) error {
	if len(parameters) == 0 {
		return nil
	}

	var params map[string]interface{}
	if err := json.Unmarshal(parameters, &params); err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	// Apply parameters to maps
	for mapName, mapValue := range params {
		if bpfMap, exists := coll.Maps[mapName]; exists {
			if err := l.setMapValue(bpfMap, mapValue); err != nil {
				l.logger.Warn("Failed to set map value",
					"map_name", mapName,
					"error", err)
			}
		}
	}

	return nil
}

// setMapValue sets a value in a BPF map
func (l *Loader) setMapValue(m *ebpf.Map, value interface{}) error {
	// This is a simplified implementation
	// In practice, you'd need to handle different map types and value types
	l.logger.Debug("Setting map value", "map_name", m.String())
	return nil
}

// attachPrograms attaches BPF programs to kernel hooks
func (l *Loader) attachPrograms(coll *ebpf.Collection) error {
	for progName, prog := range coll.Programs {
		l.logger.Debug("Attaching program", "program_name", progName)

		// Determine attachment type based on program type
		switch prog.Type() {
		case ebpf.XDP:
			// Attach XDP program
			if link, err := link.AttachXDP(link.XDPOptions{
				Program: prog,
			}); err != nil {
				return fmt.Errorf("failed to attach XDP program %s: %w", progName, err)
			} else {
				l.logger.Info("XDP program attached", "program_name", progName)
				_ = link // Store link for cleanup
			}

		case ebpf.Tracing:
			// Attach tracing program - for now, just log that it would be attached
			l.logger.Info("Tracing program would be attached", "program_name", progName)
			// TODO: Implement proper tracing program attachment
			// This would require specific attachment logic based on the program type

		default:
			l.logger.Warn("Unknown program type, skipping attachment",
				"program_name", progName,
				"type", prog.Type())
		}
	}

	return nil
}

// ExtractedArtifact represents extracted artifact data
type ExtractedArtifact struct {
	Name       string
	Version    string
	ObjectData []byte
}

// SaveToCache saves artifact data to cache directory
func (l *Loader) SaveToCache(artifactID string, data []byte) error {
	cacheFile := filepath.Join(l.cacheDir, artifactID+".tar.zst")
	
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	l.logger.Debug("Artifact saved to cache", "artifact_id", artifactID, "file", cacheFile)
	return nil
}

// LoadFromCache loads artifact data from cache directory
func (l *Loader) LoadFromCache(artifactID string) ([]byte, error) {
	cacheFile := filepath.Join(l.cacheDir, artifactID+".tar.zst")
	
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	l.logger.Debug("Artifact loaded from cache", "artifact_id", artifactID, "file", cacheFile)
	return data, nil
}

// CleanupCache removes expired cache files
func (l *Loader) CleanupCache(maxAge time.Duration) error {
	entries, err := os.ReadDir(l.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	cleaned := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(l.cacheDir, entry.Name())
			if err := os.Remove(filePath); err != nil {
				l.logger.Warn("Failed to remove cache file", "file", filePath, "error", err)
			} else {
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		l.logger.Info("Cleaned up cache files", "count", cleaned)
	}

	return nil
}
