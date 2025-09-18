package ebpf

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// ExampleOrchestration demonstrates how to use the BPF orchestrator
func ExampleOrchestration() error {
	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Configuration
	templatesDir := "/path/to/bpf-templates"
	outputDir := "/tmp/bpf-orchestrator"
	registryURL := "http://localhost:8084"
	authToken := "your-auth-token"

	// Create orchestrator
	orchestrator := NewOrchestrator(logger, templatesDir, outputDir, registryURL, authToken)

	// Check health
	ctx := context.Background()
	if err := orchestrator.Health(ctx); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Get available templates
	templates, err := orchestrator.GetAvailableTemplates()
	if err != nil {
		return fmt.Errorf("failed to get templates: %w", err)
	}

	logger.Info("Available templates", "templates", templates)

	// Example: Orchestrate drop_egress_by_cgroup template
	request := OrchestrationRequest{
		TemplateName: "drop_egress_by_cgroup",
		Parameters: map[string]string{
			"dst_ip":    "8.8.8.8",
			"dst_port":  "53",
			"cgroup_id": "12345",
			"ttl":       "3600",
		},
		TargetArch:  "x86_64",
		KernelVer:   "5.15.0",
		Name:        "dns-blocker",
		Version:     "1.0.0",
		Description: "Blocks DNS queries to 8.8.8.8 for specific cgroup",
	}

	// Perform orchestration
	result, err := orchestrator.Orchestrate(ctx, request)
	if err != nil {
		return fmt.Errorf("orchestration failed: %w", err)
	}

	// Log results
	logger.Info("Orchestration completed",
		"artifact_id", result.ArtifactRef.ArtifactID,
		"signature", result.ArtifactRef.Signature,
		"build_time", result.RenderResult.BuildTime,
		"package_size", result.PackageResult.PackageSize,
		"total_time", result.OrchestrationTime)

	// Example: Verify the artifact
	if err := orchestrator.registry.VerifyArtifact(ctx, result.ArtifactRef); err != nil {
		logger.Warn("Artifact verification failed", "error", err)
	} else {
		logger.Info("Artifact verified successfully")
	}

	// Example: Cleanup old files
	if err := orchestrator.Cleanup(24 * time.Hour); err != nil {
		logger.Warn("Cleanup failed", "error", err)
	}

	return nil
}

// ExampleBatchOrchestration demonstrates batch processing
func ExampleBatchOrchestration() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Configuration
	templatesDir := "/path/to/bpf-templates"
	outputDir := "/tmp/bpf-orchestrator"
	registryURL := "http://localhost:8084"
	authToken := "your-auth-token"

	orchestrator := NewOrchestrator(logger, templatesDir, outputDir, registryURL, authToken)

	// Batch of orchestration requests
	requests := []OrchestrationRequest{
		{
			TemplateName: "drop_egress_by_cgroup",
			Parameters: map[string]string{
				"dst_ip":    "8.8.8.8",
				"dst_port":  "53",
				"cgroup_id": "12345",
				"ttl":       "3600",
			},
			TargetArch:  "x86_64",
			Name:        "dns-blocker",
			Version:     "1.0.0",
			Description: "Blocks DNS queries to 8.8.8.8",
		},
		{
			TemplateName: "deny_syscall_for_cgroup",
			Parameters: map[string]string{
				"cgroup_id":   "67890",
				"syscall":     "execve",
				"ttl":         "1800",
			},
			TargetArch:  "x86_64",
			Name:        "execve-blocker",
			Version:     "1.0.0",
			Description: "Blocks execve syscall for specific cgroup",
		},
	}

	ctx := context.Background()
	results := make([]*OrchestrationResult, 0, len(requests))

	// Process requests
	for i, request := range requests {
		logger.Info("Processing batch request", "index", i, "template", request.TemplateName)

		result, err := orchestrator.Orchestrate(ctx, request)
		if err != nil {
			logger.Error("Batch orchestration failed", "index", i, "error", err)
			continue
		}

		results = append(results, result)
		logger.Info("Batch request completed", "index", i, "artifact_id", result.ArtifactRef.ArtifactID)
	}

	logger.Info("Batch orchestration completed", "total_requests", len(requests), "successful", len(results))

	return nil
}

// ExampleTemplateValidation demonstrates template validation
func ExampleTemplateValidation() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	templatesDir := "/path/to/bpf-templates"
	outputDir := "/tmp/bpf-orchestrator"
	registryURL := "http://localhost:8084"
	authToken := "your-auth-token"

	orchestrator := NewOrchestrator(logger, templatesDir, outputDir, registryURL, authToken)

	// Validate all available templates
	templates, err := orchestrator.GetAvailableTemplates()
	if err != nil {
		return fmt.Errorf("failed to get templates: %w", err)
	}

	for _, template := range templates {
		logger.Info("Validating template", "template", template)

		if err := orchestrator.ValidateTemplate(template); err != nil {
			logger.Error("Template validation failed", "template", template, "error", err)
		} else {
			logger.Info("Template validation passed", "template", template)
		}
	}

	return nil
}

// ExampleErrorHandling demonstrates proper error handling
func ExampleErrorHandling() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Configuration with invalid values to trigger errors
	templatesDir := "/nonexistent/templates"
	outputDir := "/tmp/bpf-orchestrator"
	registryURL := "http://localhost:8084"
	authToken := "invalid-token"

	orchestrator := NewOrchestrator(logger, templatesDir, outputDir, registryURL, authToken)

	ctx := context.Background()

	// Example request with invalid template
	request := OrchestrationRequest{
		TemplateName: "nonexistent_template",
		Parameters:   map[string]string{},
		Name:         "test",
		Version:      "1.0.0",
	}

	// Perform orchestration with error handling
	result, err := orchestrator.Orchestrate(ctx, request)
	if err != nil {
		// Log error with context
		context := map[string]interface{}{
			"template": request.TemplateName,
			"request":  request,
		}
		LogError(logger, err, context)

		// Check error type and handle accordingly
		if GetErrorCode(err) == ErrCodeTemplateNotFound {
			logger.Info("Template not found, skipping orchestration")
			return nil
		}

		if IsRetryableError(err) {
			logger.Info("Error is retryable, could implement retry logic")
		}

		return fmt.Errorf("orchestration failed: %w", err)
	}

	logger.Info("Unexpected success", "result", result)
	return nil
}

// ExampleCleanup demonstrates cleanup operations
func ExampleCleanup() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	templatesDir := "/path/to/bpf-templates"
	outputDir := "/tmp/bpf-orchestrator"
	registryURL := "http://localhost:8084"
	authToken := "your-auth-token"

	orchestrator := NewOrchestrator(logger, templatesDir, outputDir, registryURL, authToken)

	// Cleanup files older than 1 hour
	if err := orchestrator.Cleanup(1 * time.Hour); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	logger.Info("Cleanup completed successfully")
	return nil
}
