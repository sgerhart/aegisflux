package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Client handles configuration retrieval from config-api
type Client struct {
	baseURL string
	client  *http.Client
	logger  *slog.Logger
}

// ConfigEntry represents a configuration entry from the API
type ConfigEntry struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	Scope     string          `json:"scope"`
	UpdatedBy string          `json:"updated_by"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// ConfigSnapshot represents the current configuration state
type ConfigSnapshot struct {
	DecisionMode              string    `json:"decision_mode"`
	MaxCanaryHosts            int       `json:"max_canary_hosts"`
	DefaultTTLSeconds         int       `json:"default_ttl_seconds"`
	NeverBlockLabels          []string  `json:"never_block_labels"`
	LastUpdated               time.Time `json:"last_updated"`
}

// NewClient creates a new configuration client
func NewClient(baseURL string, logger *slog.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// GetSnapshot fetches the current configuration snapshot from config-api
func (c *Client) GetSnapshot() (*ConfigSnapshot, error) {
	url := fmt.Sprintf("%s/config", c.baseURL)
	
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("config-api returned status %d", resp.StatusCode)
	}

	var response struct {
		Configs []ConfigEntry `json:"configs"`
		Count   int           `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode config response: %w", err)
	}

	// Parse configuration entries into snapshot
	snapshot := &ConfigSnapshot{
		DecisionMode:      "suggest", // default
		MaxCanaryHosts:    5,         // default
		DefaultTTLSeconds: 3600,      // default
		NeverBlockLabels:  []string{"role:db", "role:control-plane"}, // default
		LastUpdated:       time.Now(),
	}

	for _, config := range response.Configs {
		switch config.Key {
		case "decision.mode":
			var mode string
			if err := json.Unmarshal(config.Value, &mode); err == nil {
				snapshot.DecisionMode = mode
			}
		case "decision.max_canary_hosts":
			var maxHosts int
			if err := json.Unmarshal(config.Value, &maxHosts); err == nil {
				snapshot.MaxCanaryHosts = maxHosts
			} else if str, err := strconv.Atoi(string(config.Value)); err == nil {
				snapshot.MaxCanaryHosts = str
			}
		case "decision.default_ttl_seconds":
			var ttl int
			if err := json.Unmarshal(config.Value, &ttl); err == nil {
				snapshot.DefaultTTLSeconds = ttl
			} else if str, err := strconv.Atoi(string(config.Value)); err == nil {
				snapshot.DefaultTTLSeconds = str
			}
		case "guardrails.never_block_labels":
			var labels []string
			if err := json.Unmarshal(config.Value, &labels); err == nil {
				snapshot.NeverBlockLabels = labels
			}
		}
	}

	c.logger.Info("Configuration snapshot loaded", 
		"decision_mode", snapshot.DecisionMode,
		"max_canary_hosts", snapshot.MaxCanaryHosts,
		"default_ttl_seconds", snapshot.DefaultTTLSeconds,
		"never_block_labels", snapshot.NeverBlockLabels,
		"config_count", response.Count)

	return snapshot, nil
}

// GetSnapshotWithFallback fetches config snapshot with fallback to environment defaults
func (c *Client) GetSnapshotWithFallback(envDefaults *ConfigSnapshot) *ConfigSnapshot {
	snapshot, err := c.GetSnapshot()
	if err != nil {
		c.logger.Warn("Failed to fetch config snapshot, using environment defaults", 
			"error", err,
			"fallback_mode", envDefaults.DecisionMode,
			"fallback_max_canary", envDefaults.MaxCanaryHosts)
		return envDefaults
	}
	return snapshot
}
