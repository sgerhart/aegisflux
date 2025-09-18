package ebpf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"log/slog"
)

// OrchestrationRequest represents a complete orchestration request
type OrchestrationRequest struct {
	TemplateName string            `json:"template_name"`
	Parameters   map[string]string `json:"parameters"`
	TargetArch   string            `json:"target_arch,omitempty"`
	KernelVer    string            `json:"kernel_version,omitempty"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
}

// OrchestrationResult represents the complete result of orchestration
type OrchestrationResult struct {
	ArtifactRef    *ArtifactReference `json:"artifact_reference"`
	RenderResult   *RenderResult      `json:"render_result"`
	PackageResult  *PackageResult     `json:"package_result"`
	OrchestrationTime time.Duration   `json:"orchestration_time"`
}

// Orchestrator coordinates the entire BPF template rendering, packing, and upload process
type Orchestrator struct {
	logger       *slog.Logger
	renderer     *Renderer
	packer       *Packer
	registry     *RegistryClient
	templatesDir string
	outputDir    string
}

// NewOrchestrator creates a new BPF orchestrator
func NewOrchestrator(logger *slog.Logger, templatesDir, outputDir, registryURL, authToken string) *Orchestrator {
	renderer := NewRenderer(logger, templatesDir, outputDir, registryURL)
	packer := NewPacker(logger)
	registry := NewRegistryClient(logger, registryURL, authToken)

	return &Orchestrator{
		logger:       logger,
		renderer:     renderer,
		packer:       packer,
		registry:     registry,
		templatesDir: templatesDir,
		outputDir:    outputDir,
	}
}

// Orchestrate performs the complete render, pack, sign, and upload process
func (o *Orchestrator) Orchestrate(ctx context.Context, request OrchestrationRequest) (*OrchestrationResult, error) {
	o.logger.Info("Starting BPF orchestration",
		"template", request.TemplateName,
		"name", request.Name,
		"version", request.Version)

	startTime := time.Now()

	// Step 1: Render the template
	renderParams := RenderParams{
		TemplateName: request.TemplateName,
		Parameters:   request.Parameters,
		TargetArch:   request.TargetArch,
		KernelVer:    request.KernelVer,
		BuildID:      fmt.Sprintf("%s-%s", request.Name, request.Version),
	}

	renderResult, err := o.renderer.Render(ctx, renderParams)
	if err != nil {
		return nil, fmt.Errorf("rendering failed: %w", err)
	}

	o.logger.Info("Template rendering completed",
		"object_path", renderResult.ObjectPath,
		"build_time", renderResult.BuildTime)

	// Step 2: Pack the rendered template
	packageMetadata := PackageMetadata{
		TemplateName:  request.TemplateName,
		Version:       request.Version,
		Architecture:  request.TargetArch,
		KernelVersion: request.KernelVer,
		BuildID:       renderParams.BuildID,
		Parameters:    request.Parameters,
	}

	// Create package path
	packageName := fmt.Sprintf("%s-%s-%s.tar.zst", request.Name, request.Version, request.TargetArch)
	packagePath := filepath.Join(o.outputDir, "packages", packageName)

	packageResult, err := o.packer.Pack(renderResult, packageMetadata, packagePath)
	if err != nil {
		return nil, fmt.Errorf("packing failed: %w", err)
	}

	o.logger.Info("Package creation completed",
		"package_path", packageResult.PackagePath,
		"package_size", packageResult.PackageSize)

	// Step 3: Sign and upload to registry
	artifactRef, err := o.registry.SignAndUpload(ctx, packageResult, request.Name, request.Version, request.Description)
	if err != nil {
		return nil, fmt.Errorf("signing and upload failed: %w", err)
	}

	orchestrationTime := time.Since(startTime)

	result := &OrchestrationResult{
		ArtifactRef:       artifactRef,
		RenderResult:      renderResult,
		PackageResult:     packageResult,
		OrchestrationTime: orchestrationTime,
	}

	o.logger.Info("BPF orchestration completed successfully",
		"artifact_id", artifactRef.ArtifactID,
		"total_time", orchestrationTime)

	return result, nil
}

// ValidateTemplate validates a template before orchestration
func (o *Orchestrator) ValidateTemplate(templateName string) error {
	return o.renderer.ValidateTemplate(templateName)
}

// GetAvailableTemplates returns available templates
func (o *Orchestrator) GetAvailableTemplates() ([]string, error) {
	return o.renderer.GetAvailableTemplates()
}

// Health checks the health of all components
func (o *Orchestrator) Health(ctx context.Context) error {
	// Check registry health
	if err := o.registry.Health(ctx); err != nil {
		return fmt.Errorf("registry health check failed: %w", err)
	}

	// Check templates directory
	if _, err := os.Stat(o.templatesDir); os.IsNotExist(err) {
		return fmt.Errorf("templates directory not found: %s", o.templatesDir)
	}

	// Check output directory
	if _, err := os.Stat(o.outputDir); os.IsNotExist(err) {
		return fmt.Errorf("output directory not found: %s", o.outputDir)
	}

	return nil
}

// Cleanup removes temporary files and directories
func (o *Orchestrator) Cleanup(olderThan time.Duration) error {
	o.logger.Info("Starting cleanup of old files", "older_than", olderThan)

	cutoffTime := time.Now().Add(-olderThan)

	// Clean up render directories
	renderDir := filepath.Join(o.outputDir, "renders")
	if err := o.cleanupDirectory(renderDir, cutoffTime); err != nil {
		o.logger.Warn("Failed to cleanup render directory", "error", err)
	}

	// Clean up package files
	packageDir := filepath.Join(o.outputDir, "packages")
	if err := o.cleanupDirectory(packageDir, cutoffTime); err != nil {
		o.logger.Warn("Failed to cleanup package directory", "error", err)
	}

	o.logger.Info("Cleanup completed")
	return nil
}

// cleanupDirectory removes files older than the cutoff time
func (o *Orchestrator) cleanupDirectory(dir string, cutoffTime time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, nothing to clean
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			filePath := filepath.Join(dir, entry.Name())
			if err := os.Remove(filePath); err != nil {
				o.logger.Warn("Failed to remove old file", "file", filePath, "error", err)
			} else {
				o.logger.Debug("Removed old file", "file", filePath)
			}
		}
	}

	return nil
}
