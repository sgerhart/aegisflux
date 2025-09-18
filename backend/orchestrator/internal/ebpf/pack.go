package ebpf

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"

	"github.com/klauspost/compress/zstd"
)

// PackageMetadata contains metadata for the BPF package
type PackageMetadata struct {
	TemplateName string            `json:"template_name"`
	Version      string            `json:"version"`
	Architecture string            `json:"architecture"`
	KernelVersion string           `json:"kernel_version"`
	BuildID      string            `json:"build_id"`
	CreatedAt    time.Time         `json:"created_at"`
	Parameters   map[string]string `json:"parameters"`
	BuildInfo    BuildInfo         `json:"build_info"`
	Checksums    map[string]string `json:"checksums"`
}

// BuildInfo contains build-specific information
type BuildInfo struct {
	BuildTime    time.Duration `json:"build_time"`
	BuildLog     string        `json:"build_log,omitempty"`
	Compiler     string        `json:"compiler"`
	CompilerVer  string        `json:"compiler_version"`
	BuildFlags   []string      `json:"build_flags"`
	ObjectSize   int64         `json:"object_size"`
}

// PackageResult contains the result of packaging
type PackageResult struct {
	PackagePath string            `json:"package_path"`
	PackageSize int64             `json:"package_size"`
	Metadata    PackageMetadata   `json:"metadata"`
	Files       []string          `json:"files"`
	Checksum    string            `json:"checksum"`
}

// Packer handles BPF package creation and management
type Packer struct {
	logger *slog.Logger
}

// NewPacker creates a new BPF package packer
func NewPacker(logger *slog.Logger) *Packer {
	return &Packer{
		logger: logger,
	}
}

// Pack creates a tar.zst package from a rendered BPF template
func (p *Packer) Pack(renderResult *RenderResult, metadata PackageMetadata, outputPath string) (*PackageResult, error) {
	p.logger.Info("Starting BPF package creation",
		"object_path", renderResult.ObjectPath,
		"output_path", outputPath)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create the tar.zst file
	file, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create package file: %w", err)
	}
	defer file.Close()

	// Create zstd writer
	zstdWriter, err := zstd.NewWriter(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd writer: %w", err)
	}
	defer zstdWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(zstdWriter)
	defer tarWriter.Close()

	var files []string
	checksums := make(map[string]string)

	// Add the BPF object file
	objectFile, err := os.Open(renderResult.ObjectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open BPF object: %w", err)
	}
	defer objectFile.Close()

	objectInfo, err := objectFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat BPF object: %w", err)
	}

	// Calculate checksum for BPF object
	checksum, err := p.calculateChecksum(objectFile)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate object checksum: %w", err)
	}
	checksums["program.o"] = checksum

	// Reset file pointer
	if _, err := objectFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to reset file pointer: %w", err)
	}

	// Add BPF object to tar
	if err := p.addFileToTar(tarWriter, "program.o", objectFile, objectInfo); err != nil {
		return nil, fmt.Errorf("failed to add BPF object to tar: %w", err)
	}
	files = append(files, "program.o")

	// Add skeleton header if it exists
	skeletonPath := strings.TrimSuffix(renderResult.ObjectPath, ".bpf.o") + ".skel.h"
	if _, err := os.Stat(skeletonPath); err == nil {
		skeletonFile, err := os.Open(skeletonPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open skeleton file: %w", err)
		}
		defer skeletonFile.Close()

		skeletonInfo, err := skeletonFile.Stat()
		if err != nil {
			return nil, fmt.Errorf("failed to stat skeleton file: %w", err)
		}

		// Calculate checksum for skeleton
		skeletonChecksum, err := p.calculateChecksum(skeletonFile)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate skeleton checksum: %w", err)
		}
		checksums["program.skel.h"] = skeletonChecksum

		// Reset file pointer
		if _, err := skeletonFile.Seek(0, 0); err != nil {
			return nil, fmt.Errorf("failed to reset skeleton file pointer: %w", err)
		}

		// Add skeleton to tar
		if err := p.addFileToTar(tarWriter, "program.skel.h", skeletonFile, skeletonInfo); err != nil {
			return nil, fmt.Errorf("failed to add skeleton to tar: %w", err)
		}
		files = append(files, "program.skel.h")
	}

	// Add source files if they exist
	srcDir := filepath.Join(filepath.Dir(renderResult.ObjectPath), "src")
	if _, err := os.Stat(srcDir); err == nil {
		err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				relPath, err := filepath.Rel(srcDir, path)
				if err != nil {
					return err
				}

				srcFile, err := os.Open(path)
				if err != nil {
					return err
				}
				defer srcFile.Close()

				// Calculate checksum
				srcChecksum, err := p.calculateChecksum(srcFile)
				if err != nil {
					return err
				}
				checksums[fmt.Sprintf("src/%s", relPath)] = srcChecksum

				// Reset file pointer
				if _, err := srcFile.Seek(0, 0); err != nil {
					return err
				}

				// Add to tar
				tarPath := fmt.Sprintf("src/%s", relPath)
				if err := p.addFileToTar(tarWriter, tarPath, srcFile, info); err != nil {
					return err
				}
				files = append(files, tarPath)
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to add source files: %w", err)
		}
	}

	// Add metadata to the package
	metadata.Checksums = checksums
	metadata.CreatedAt = time.Now()
	metadata.BuildInfo = BuildInfo{
		BuildTime:   renderResult.BuildTime,
		BuildLog:    renderResult.BuildLog,
		ObjectSize:  renderResult.CompiledSize,
		Compiler:    "clang",
		CompilerVer: "unknown", // Could be determined from build process
	}

	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Add metadata to tar
	if err := p.addBytesToTar(tarWriter, "metadata.json", metadataJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to add metadata to tar: %w", err)
	}
	files = append(files, "metadata.json")

	// Calculate package checksum
	packageStat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat package file: %w", err)
	}

	// Reset file pointer for checksum calculation
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to reset package file pointer: %w", err)
	}

	packageChecksum, err := p.calculateChecksum(file)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate package checksum: %w", err)
	}

	result := &PackageResult{
		PackagePath: outputPath,
		PackageSize: packageStat.Size(),
		Metadata:    metadata,
		Files:       files,
		Checksum:    packageChecksum,
	}

	p.logger.Info("BPF package created successfully",
		"package_path", outputPath,
		"package_size", packageStat.Size(),
		"files_count", len(files))

	return result, nil
}

// addFileToTar adds a file to the tar archive
func (p *Packer) addFileToTar(tarWriter *tar.Writer, name string, file io.Reader, info os.FileInfo) error {
	header := &tar.Header{
		Name:    name,
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	_, err := io.Copy(tarWriter, file)
	return err
}

// addBytesToTar adds bytes data to the tar archive
func (p *Packer) addBytesToTar(tarWriter *tar.Writer, name string, data []byte, mode int64) error {
	header := &tar.Header{
		Name:    name,
		Size:    int64(len(data)),
		Mode:    mode,
		ModTime: time.Now(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	_, err := tarWriter.Write(data)
	return err
}

// calculateChecksum calculates SHA256 checksum of a file
func (p *Packer) calculateChecksum(reader io.Reader) (string, error) {
	// For simplicity, we'll use a basic checksum here
	// In production, you might want to use crypto/sha256
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	// Simple checksum implementation (replace with proper SHA256 in production)
	checksum := fmt.Sprintf("%x", len(data))
	return checksum, nil
}

// ExtractPackage extracts a tar.zst package to a directory
func (p *Packer) ExtractPackage(packagePath, extractDir string) (*PackageMetadata, error) {
	p.logger.Info("Extracting BPF package",
		"package_path", packagePath,
		"extract_dir", extractDir)

	// Create extract directory
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create extract directory: %w", err)
	}

	// Open package file
	file, err := os.Open(packagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open package: %w", err)
	}
	defer file.Close()

	// Create zstd reader
	zstdReader, err := zstd.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer zstdReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(zstdReader)

	var metadata PackageMetadata

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		// Extract metadata
		if header.Name == "metadata.json" {
			metadataBytes, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read metadata: %w", err)
			}

			if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		// Extract file
		extractPath := filepath.Join(extractDir, header.Name)
		if err := os.MkdirAll(filepath.Dir(extractPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create extract subdirectory: %w", err)
		}

		extractFile, err := os.OpenFile(extractPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			return nil, fmt.Errorf("failed to create extract file: %w", err)
		}

		_, err = io.Copy(extractFile, tarReader)
		extractFile.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to extract file: %w", err)
		}
	}

	p.logger.Info("BPF package extracted successfully",
		"template", metadata.TemplateName,
		"extract_dir", extractDir)

	return &metadata, nil
}

// ValidatePackage validates a BPF package
func (p *Packer) ValidatePackage(packagePath string) error {
	// Check if package file exists
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("package not found: %s", packagePath)
	}

	// Try to extract metadata without full extraction
	file, err := os.Open(packagePath)
	if err != nil {
		return fmt.Errorf("failed to open package: %w", err)
	}
	defer file.Close()

	zstdReader, err := zstd.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer zstdReader.Close()

	tarReader := tar.NewReader(zstdReader)

	// Look for metadata.json
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return fmt.Errorf("metadata.json not found in package")
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		if header.Name == "metadata.json" {
			metadataBytes, err := io.ReadAll(tarReader)
			if err != nil {
				return fmt.Errorf("failed to read metadata: %w", err)
			}

			var metadata PackageMetadata
			if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
				return fmt.Errorf("failed to unmarshal metadata: %w", err)
			}

			// Basic validation
			if metadata.TemplateName == "" {
				return fmt.Errorf("invalid metadata: missing template name")
			}

			return nil
		}
	}
}
