package ebpf

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"log/slog"
)

// RenderParams contains parameters for rendering a BPF template
type RenderParams struct {
	TemplateName string            `json:"template_name"`
	Parameters   map[string]string `json:"parameters"`
	TargetArch   string            `json:"target_arch,omitempty"`
	KernelVer    string            `json:"kernel_version,omitempty"`
	BuildID      string            `json:"build_id,omitempty"`
}

// RenderResult contains the result of template rendering
type RenderResult struct {
	ObjectPath   string            `json:"object_path"`
	BuildLog     string            `json:"build_log"`
	BuildTime    time.Duration     `json:"build_time"`
	Metadata     map[string]string `json:"metadata"`
	CompiledSize int64             `json:"compiled_size"`
}

// Renderer handles BPF template rendering and compilation
type Renderer struct {
	logger      *slog.Logger
	templatesDir string
	outputDir   string
	registryURL string
}

// NewRenderer creates a new BPF template renderer
func NewRenderer(logger *slog.Logger, templatesDir, outputDir, registryURL string) *Renderer {
	return &Renderer{
		logger:        logger,
		templatesDir:  templatesDir,
		outputDir:     outputDir,
		registryURL:   registryURL,
	}
}

// Render compiles a BPF template with the given parameters
func (r *Renderer) Render(ctx context.Context, params RenderParams) (*RenderResult, error) {
	r.logger.Info("Starting BPF template render", 
		"template", params.TemplateName,
		"params", params.Parameters)

	startTime := time.Now()

	// Validate template exists
	templatePath := filepath.Join(r.templatesDir, params.TemplateName)
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("template not found: %s", params.TemplateName)
	}

	// Create output directory for this render
	renderID := fmt.Sprintf("%s_%d", params.TemplateName, time.Now().Unix())
	renderDir := filepath.Join(r.outputDir, renderID)
	if err := os.MkdirAll(renderDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create render directory: %w", err)
	}

	// Copy template to render directory
	if err := r.copyTemplate(templatePath, renderDir, params.Parameters); err != nil {
		return nil, fmt.Errorf("failed to copy template: %w", err)
	}

	// Compile the BPF object
	objectPath, buildLog, err := r.compileBPF(ctx, renderDir, params)
	if err != nil {
		return nil, fmt.Errorf("failed to compile BPF object: %w", err)
	}

	// Get compiled object size
	stat, err := os.Stat(objectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat compiled object: %w", err)
	}

	buildTime := time.Since(startTime)

	result := &RenderResult{
		ObjectPath:   objectPath,
		BuildLog:     buildLog,
		BuildTime:    buildTime,
		CompiledSize: stat.Size(),
		Metadata: map[string]string{
			"template_name": params.TemplateName,
			"render_id":     renderID,
			"target_arch":   params.TargetArch,
			"kernel_ver":    params.KernelVer,
			"build_id":      params.BuildID,
			"build_time":    buildTime.String(),
		},
	}

	// Add parameter metadata
	for key, value := range params.Parameters {
		result.Metadata[fmt.Sprintf("param_%s", key)] = value
	}

	r.logger.Info("BPF template render completed",
		"template", params.TemplateName,
		"build_time", buildTime,
		"object_size", stat.Size())

	return result, nil
}

// copyTemplate copies the template directory and applies parameter substitutions
func (r *Renderer) copyTemplate(srcDir, dstDir string, params map[string]string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Read source file
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Apply parameter substitutions
		processedContent := r.substituteParams(string(content), params)

		// Write to destination
		if err := os.WriteFile(dstPath, []byte(processedContent), info.Mode()); err != nil {
			return err
		}

		return nil
	})
}

// substituteParams replaces template parameters in file content
func (r *Renderer) substituteParams(content string, params map[string]string) string {
	result := content
	for key, value := range params {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// compileBPF compiles the BPF object using make
func (r *Renderer) compileBPF(ctx context.Context, renderDir string, params RenderParams) (string, string, error) {
	// Change to render directory
	originalDir, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(renderDir); err != nil {
		return "", "", fmt.Errorf("failed to change to render directory: %w", err)
	}

	// Set environment variables for compilation
	env := os.Environ()
	if params.TargetArch != "" {
		env = append(env, fmt.Sprintf("ARCH=%s", params.TargetArch))
	}
	if params.KernelVer != "" {
		env = append(env, fmt.Sprintf("KERNEL_VERSION=%s", params.KernelVer))
	}

	// Run make to compile the BPF object
	cmd := exec.CommandContext(ctx, "make", "all")
	cmd.Env = env
	cmd.Dir = renderDir

	output, err := cmd.CombinedOutput()
	buildLog := string(output)

	if err != nil {
		r.logger.Error("BPF compilation failed", 
			"error", err,
			"build_log", buildLog)
		return "", buildLog, fmt.Errorf("make failed: %w", err)
	}

	// Find the compiled object file
	objectPath := filepath.Join(renderDir, "*.bpf.o")
	matches, err := filepath.Glob(objectPath)
	if err != nil {
		return "", buildLog, fmt.Errorf("failed to find compiled object: %w", err)
	}

	if len(matches) == 0 {
		return "", buildLog, fmt.Errorf("no compiled object found")
	}

	return matches[0], buildLog, nil
}

// GetAvailableTemplates returns a list of available BPF templates
func (r *Renderer) GetAvailableTemplates() ([]string, error) {
	var templates []string

	err := filepath.Walk(r.templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if this is a template directory (contains Makefile)
		if info.IsDir() && path != r.templatesDir {
			makefilePath := filepath.Join(path, "Makefile")
			if _, err := os.Stat(makefilePath); err == nil {
				relPath, err := filepath.Rel(r.templatesDir, path)
				if err == nil {
					templates = append(templates, relPath)
				}
			}
		}

		return nil
	})

	return templates, err
}

// ValidateTemplate checks if a template is valid and can be rendered
func (r *Renderer) ValidateTemplate(templateName string) error {
	templatePath := filepath.Join(r.templatesDir, templateName)
	
	// Check if template directory exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return fmt.Errorf("template not found: %s", templateName)
	}

	// Check for required files
	requiredFiles := []string{"Makefile", "src"}
	for _, file := range requiredFiles {
		filePath := filepath.Join(templatePath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("required file not found: %s/%s", templateName, file)
		}
	}

	// Try to validate Makefile syntax
	if _, err := exec.Command("make", "-n", "-C", templatePath).Output(); err != nil {
		return fmt.Errorf("invalid Makefile in template %s: %w", templateName, err)
	}

	return nil
}
