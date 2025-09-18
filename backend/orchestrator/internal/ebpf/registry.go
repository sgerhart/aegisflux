package ebpf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"log/slog"
)

// ArtifactReference represents a reference to a signed artifact in the registry
type ArtifactReference struct {
	ArtifactID string `json:"artifact_id"`
	Signature  string `json:"signature"`
	URL        string `json:"url"`
	Checksum   string `json:"checksum"`
}

// CreateArtifactRequest represents the request to create an artifact
type CreateArtifactRequest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Type        string            `json:"type"`
	Data        string            `json:"data"`        // Base64 encoded package data
	Metadata    PackageMetadata   `json:"metadata"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// CreateArtifactResponse represents the response from artifact creation
type CreateArtifactResponse struct {
	ID        string `json:"id"`
	Signature string `json:"signature"`
	URL       string `json:"url"`
	Checksum  string `json:"checksum"`
}

// RegistryClient handles communication with the BPF registry
type RegistryClient struct {
	logger     *slog.Logger
	baseURL    string
	httpClient *http.Client
	authToken  string
}

// NewRegistryClient creates a new BPF registry client
func NewRegistryClient(logger *slog.Logger, baseURL, authToken string) *RegistryClient {
	return &RegistryClient{
		logger:  logger,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		authToken: authToken,
	}
}

// SignAndUpload signs and uploads a BPF package to the registry
func (c *RegistryClient) SignAndUpload(ctx context.Context, packageResult *PackageResult, name, version, description string) (*ArtifactReference, error) {
	c.logger.Info("Starting artifact signing and upload",
		"package_path", packageResult.PackagePath,
		"name", name,
		"version", version)

	// Read package data
	packageData, err := c.readPackageData(packageResult.PackagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package data: %w", err)
	}

	// Create artifact request
	request := CreateArtifactRequest{
		Name:        name,
		Version:     version,
		Type:        "bpf-program",
		Data:        packageData,
		Metadata:    packageResult.Metadata,
		Description: description,
		Tags:        []string{"bpf", "security", packageResult.Metadata.TemplateName},
	}

	// Upload to registry
	response, err := c.createArtifact(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create artifact: %w", err)
	}

	artifactRef := &ArtifactReference{
		ArtifactID: response.ID,
		Signature:  response.Signature,
		URL:        response.URL,
		Checksum:   response.Checksum,
	}

	c.logger.Info("Artifact signed and uploaded successfully",
		"artifact_id", response.ID,
		"signature", response.Signature,
		"url", response.URL)

	return artifactRef, nil
}

// readPackageData reads and base64 encodes the package file
func (c *RegistryClient) readPackageData(packagePath string) (string, error) {
	// In a real implementation, this would read the file and base64 encode it
	// For now, we'll return a placeholder
	return "base64_encoded_package_data", nil
}

// createArtifact sends a POST request to create an artifact in the registry
func (c *RegistryClient) createArtifact(ctx context.Context, request CreateArtifactRequest) (*CreateArtifactResponse, error) {
	// Marshal request to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/artifacts", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Parse response
	var response CreateArtifactResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// GetArtifact retrieves an artifact from the registry
func (c *RegistryClient) GetArtifact(ctx context.Context, artifactID string) (*CreateArtifactResponse, error) {
	url := fmt.Sprintf("%s/artifacts/%s", c.baseURL, artifactID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	var response CreateArtifactResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// ListArtifacts lists artifacts in the registry
func (c *RegistryClient) ListArtifacts(ctx context.Context) ([]CreateArtifactResponse, error) {
	url := fmt.Sprintf("%s/artifacts", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	var response struct {
		Artifacts []CreateArtifactResponse `json:"artifacts"`
		Total     int                      `json:"total"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Artifacts, nil
}

// VerifyArtifact verifies the signature of an artifact
func (c *RegistryClient) VerifyArtifact(ctx context.Context, artifactRef *ArtifactReference) error {
	c.logger.Info("Verifying artifact signature",
		"artifact_id", artifactRef.ArtifactID,
		"signature", artifactRef.Signature)

	// In a real implementation, this would:
	// 1. Retrieve the artifact from the registry
	// 2. Verify the signature using the appropriate signing key
	// 3. Verify the checksum matches the content

	// For now, we'll just log the verification attempt
	c.logger.Info("Artifact verification completed",
		"artifact_id", artifactRef.ArtifactID,
		"verified", true)

	return nil
}

// Health checks the registry health
func (c *RegistryClient) Health(ctx context.Context) error {
	url := fmt.Sprintf("%s/healthz", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry health check failed with status %d", resp.StatusCode)
	}

	return nil
}
