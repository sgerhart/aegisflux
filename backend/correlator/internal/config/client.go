package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"log/slog"
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

// ConfigSnapshot represents the current configuration state for correlator
type ConfigSnapshot struct {
	RuleWindowSeconds    int      `json:"rule_window_seconds"`
	MaxFindings         int      `json:"max_findings"`
	DedupeCap           int      `json:"dedupe_cap"`
	HotReload           bool     `json:"hot_reload"`
	DebounceMs          int      `json:"debounce_ms"`
	LabelTTLSeconds     int      `json:"label_ttl_seconds"`
	NeverBlockLabels    []string `json:"never_block_labels"`
	LastUpdated         time.Time `json:"last_updated"`
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
		RuleWindowSeconds: 5,    // default
		MaxFindings:       10000, // default
		DedupeCap:         100000, // default
		HotReload:         false, // default
		DebounceMs:        1000, // default
		LabelTTLSeconds:   300,  // default 5 minutes
		NeverBlockLabels:  []string{"role:db", "role:control-plane"}, // default
		LastUpdated:       time.Now(),
	}

	for _, config := range response.Configs {
		switch config.Key {
		case "correlator.rule_window_seconds":
			var windowSec int
			if err := json.Unmarshal(config.Value, &windowSec); err == nil {
				snapshot.RuleWindowSeconds = windowSec
			} else if str, err := strconv.Atoi(string(config.Value)); err == nil {
				snapshot.RuleWindowSeconds = str
			}
		case "correlator.max_findings":
			var maxFindings int
			if err := json.Unmarshal(config.Value, &maxFindings); err == nil {
				snapshot.MaxFindings = maxFindings
			} else if str, err := strconv.Atoi(string(config.Value)); err == nil {
				snapshot.MaxFindings = str
			}
		case "correlator.dedupe_cap":
			var dedupeCap int
			if err := json.Unmarshal(config.Value, &dedupeCap); err == nil {
				snapshot.DedupeCap = dedupeCap
			} else if str, err := strconv.Atoi(string(config.Value)); err == nil {
				snapshot.DedupeCap = str
			}
		case "correlator.hot_reload":
			var hotReload bool
			if err := json.Unmarshal(config.Value, &hotReload); err == nil {
				snapshot.HotReload = hotReload
			} else if str := string(config.Value); str == "true" || str == "1" {
				snapshot.HotReload = true
			}
		case "correlator.debounce_ms":
			var debounceMs int
			if err := json.Unmarshal(config.Value, &debounceMs); err == nil {
				snapshot.DebounceMs = debounceMs
			} else if str, err := strconv.Atoi(string(config.Value)); err == nil {
				snapshot.DebounceMs = str
			}
		case "correlator.label_ttl_seconds":
			var labelTTL int
			if err := json.Unmarshal(config.Value, &labelTTL); err == nil {
				snapshot.LabelTTLSeconds = labelTTL
			} else if str, err := strconv.Atoi(string(config.Value)); err == nil {
				snapshot.LabelTTLSeconds = str
			}
		case "correlator.never_block_labels":
			var labels []string
			if err := json.Unmarshal(config.Value, &labels); err == nil {
				snapshot.NeverBlockLabels = labels
			}
		}
	}

	c.logger.Info("Configuration snapshot loaded", 
		"rule_window_seconds", snapshot.RuleWindowSeconds,
		"max_findings", snapshot.MaxFindings,
		"dedupe_cap", snapshot.DedupeCap,
		"hot_reload", snapshot.HotReload,
		"debounce_ms", snapshot.DebounceMs,
		"label_ttl_seconds", snapshot.LabelTTLSeconds,
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
			"fallback_rule_window_seconds", envDefaults.RuleWindowSeconds,
			"fallback_max_findings", envDefaults.MaxFindings,
			"fallback_dedupe_cap", envDefaults.DedupeCap)
		return envDefaults
	}
	return snapshot
}
