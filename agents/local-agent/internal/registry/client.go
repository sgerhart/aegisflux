package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"aegisflux/agents/local-agent/internal/types"
)

// Client handles communication with the BPF registry
type Client struct {
	logger      *slog.Logger
	baseURL     string
	httpClient  *http.Client
	hostID      string
}

// NewClient creates a new registry client
func NewClient(logger *slog.Logger, baseURL, hostID string) *Client {
	return &Client{
		logger:  logger,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		hostID: hostID,
	}
}

// GetArtifactsForHost polls the registry for artifacts assigned to this host
func (c *Client) GetArtifactsForHost(ctx context.Context) ([]types.ArtifactInfo, error) {
	url := fmt.Sprintf("%s/artifacts/for-host/%s", c.baseURL, c.hostID)
	
	c.logger.Debug("Polling registry for artifacts", "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	var registryResp types.RegistryResponse
	if err := json.NewDecoder(resp.Body).Decode(&registryResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info("Retrieved artifacts from registry",
		"count", len(registryResp.Artifacts),
		"total", registryResp.Total)

	return registryResp.Artifacts, nil
}

// DownloadArtifact downloads an artifact from the registry
func (c *Client) DownloadArtifact(ctx context.Context, artifactID string) ([]byte, error) {
	url := fmt.Sprintf("%s/artifacts/%s/binary", c.baseURL, artifactID)
	
	c.logger.Debug("Downloading artifact", "artifact_id", artifactID, "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Info("Downloaded artifact",
		"artifact_id", artifactID,
		"size", len(data))

	return data, nil
}

// GetArtifactInfo gets metadata for a specific artifact
func (c *Client) GetArtifactInfo(ctx context.Context, artifactID string) (*types.ArtifactInfo, error) {
	url := fmt.Sprintf("%s/artifacts/%s", c.baseURL, artifactID)
	
	c.logger.Debug("Getting artifact info", "artifact_id", artifactID, "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	var artifact types.ArtifactInfo
	if err := json.NewDecoder(resp.Body).Decode(&artifact); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("Retrieved artifact info",
		"artifact_id", artifactID,
		"name", artifact.Name,
		"version", artifact.Version)

	return &artifact, nil
}

// VerifySignature verifies the signature of an artifact (dev stub implementation)
func (c *Client) VerifySignature(ctx context.Context, data []byte, signature string, publicKey string) error {
	// This is a dev stub implementation
	// In production, you would:
	// 1. Get the public key from Vault
	// 2. Verify the signature using proper cryptographic verification
	// 3. Validate the signature against the artifact data
	
	c.logger.Debug("Verifying signature (dev stub)",
		"data_size", len(data),
		"signature_length", len(signature),
		"public_key_length", len(publicKey))

	// For now, just check that we have a signature
	if signature == "" {
		return fmt.Errorf("empty signature")
	}

	if publicKey == "" {
		c.logger.Warn("No public key provided, skipping signature verification (dev mode)")
		return nil
	}

	// Simulate verification delay
	time.Sleep(100 * time.Millisecond)

	c.logger.Info("Signature verification passed (dev stub)")
	return nil
}

// HealthCheck checks if the registry is healthy
func (c *Client) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/healthz", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry health check failed with status %d", resp.StatusCode)
	}

	c.logger.Debug("Registry health check passed")
	return nil
}
