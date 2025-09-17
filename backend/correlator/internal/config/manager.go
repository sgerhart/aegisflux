package config

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Manager handles configuration management with live updates for correlator
type Manager struct {
	client        *Client
	nats          *nats.Conn
	logger        *slog.Logger
	currentConfig *ConfigSnapshot
	mu            sync.RWMutex
	subscribers   []func(*ConfigSnapshot)
}

// ConfigChangeMessage represents a configuration change from NATS
type ConfigChangeMessage struct {
	Key        string          `json:"key"`
	Value      json.RawMessage `json:"value"`
	Scope      string          `json:"scope"`
	UpdatedBy  string          `json:"updated_by"`
	Timestamp  int64           `json:"timestamp"`
}

// NewManager creates a new configuration manager
func NewManager(configAPIURL string, nats *nats.Conn, logger *slog.Logger) *Manager {
	return &Manager{
		client:      NewClient(configAPIURL, logger),
		nats:        nats,
		logger:      logger,
		subscribers: make([]func(*ConfigSnapshot), 0),
	}
}

// Initialize loads the initial configuration snapshot and sets up NATS subscription
func (m *Manager) Initialize(ctx context.Context, envDefaults *ConfigSnapshot) error {
	// Load initial configuration snapshot
	m.logger.Info("Loading initial configuration snapshot")
	snapshot := m.client.GetSnapshotWithFallback(envDefaults)
	m.updateConfig(snapshot)

	// Subscribe to configuration changes
	if err := m.subscribeToConfigChanges(ctx); err != nil {
		m.logger.Error("Failed to subscribe to config changes", "error", err)
		return err
	}

	m.logger.Info("Configuration manager initialized successfully")
	return nil
}

// GetCurrentConfig returns the current configuration snapshot
func (m *Manager) GetCurrentConfig() *ConfigSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a copy to prevent external modification
	if m.currentConfig == nil {
		return nil
	}
	
	config := *m.currentConfig
	return &config
}

// Subscribe adds a callback function that will be called when configuration changes
func (m *Manager) Subscribe(callback func(*ConfigSnapshot)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.subscribers = append(m.subscribers, callback)
}

// subscribeToConfigChanges subscribes to NATS config.changed messages
func (m *Manager) subscribeToConfigChanges(ctx context.Context) error {
	_, err := m.nats.Subscribe("config.changed", func(msg *nats.Msg) {
		m.handleConfigChange(msg.Data)
	})
	
	if err != nil {
		return err
	}

	m.logger.Info("Subscribed to config.changed NATS subject")
	return nil
}

// handleConfigChange processes incoming configuration change messages
func (m *Manager) handleConfigChange(data []byte) {
	var change ConfigChangeMessage
	if err := json.Unmarshal(data, &change); err != nil {
		m.logger.Error("Failed to unmarshal config change message", "error", err)
		return
	}

	m.logger.Info("Received configuration change", 
		"key", change.Key,
		"updated_by", change.UpdatedBy,
		"timestamp", change.Timestamp)

	// Get current config and apply the change
	m.mu.Lock()
	current := m.currentConfig
	if current == nil {
		current = &ConfigSnapshot{}
	}
	
	// Create a copy to modify
	newConfig := *current
	
	// Apply the specific configuration change
	m.applyConfigChange(&newConfig, &change)
	newConfig.LastUpdated = time.Unix(change.Timestamp, 0)
	
	m.currentConfig = &newConfig
	m.mu.Unlock()

	// Notify subscribers
	m.notifySubscribers(&newConfig)
	
	m.logger.Info("Configuration updated live", 
		"key", change.Key,
		"rule_window_seconds", newConfig.RuleWindowSeconds,
		"max_findings", newConfig.MaxFindings,
		"dedupe_cap", newConfig.DedupeCap,
		"hot_reload", newConfig.HotReload,
		"debounce_ms", newConfig.DebounceMs,
		"label_ttl_seconds", newConfig.LabelTTLSeconds,
		"never_block_labels", newConfig.NeverBlockLabels)
}

// applyConfigChange applies a specific configuration change to the snapshot
func (m *Manager) applyConfigChange(config *ConfigSnapshot, change *ConfigChangeMessage) {
	switch change.Key {
	case "correlator.rule_window_seconds":
		var windowSec int
		if err := json.Unmarshal(change.Value, &windowSec); err == nil {
			config.RuleWindowSeconds = windowSec
		} else if str, err := strconv.Atoi(string(change.Value)); err == nil {
			config.RuleWindowSeconds = str
		}
	case "correlator.max_findings":
		var maxFindings int
		if err := json.Unmarshal(change.Value, &maxFindings); err == nil {
			config.MaxFindings = maxFindings
		} else if str, err := strconv.Atoi(string(change.Value)); err == nil {
			config.MaxFindings = str
		}
	case "correlator.dedupe_cap":
		var dedupeCap int
		if err := json.Unmarshal(change.Value, &dedupeCap); err == nil {
			config.DedupeCap = dedupeCap
		} else if str, err := strconv.Atoi(string(change.Value)); err == nil {
			config.DedupeCap = str
		}
	case "correlator.hot_reload":
		var hotReload bool
		if err := json.Unmarshal(change.Value, &hotReload); err == nil {
			config.HotReload = hotReload
		} else if str := string(change.Value); str == "true" || str == "1" {
			config.HotReload = true
		}
	case "correlator.debounce_ms":
		var debounceMs int
		if err := json.Unmarshal(change.Value, &debounceMs); err == nil {
			config.DebounceMs = debounceMs
		} else if str, err := strconv.Atoi(string(change.Value)); err == nil {
			config.DebounceMs = str
		}
	case "correlator.label_ttl_seconds":
		var labelTTL int
		if err := json.Unmarshal(change.Value, &labelTTL); err == nil {
			config.LabelTTLSeconds = labelTTL
		} else if str, err := strconv.Atoi(string(change.Value)); err == nil {
			config.LabelTTLSeconds = str
		}
	case "correlator.never_block_labels":
		var labels []string
		if err := json.Unmarshal(change.Value, &labels); err == nil {
			config.NeverBlockLabels = labels
		}
	default:
		m.logger.Debug("Ignoring unknown configuration key", "key", change.Key)
	}
}

// updateConfig updates the current configuration and notifies subscribers
func (m *Manager) updateConfig(config *ConfigSnapshot) {
	m.mu.Lock()
	m.currentConfig = config
	m.mu.Unlock()
	
	m.notifySubscribers(config)
}

// notifySubscribers notifies all subscribers of configuration changes
func (m *Manager) notifySubscribers(config *ConfigSnapshot) {
	m.mu.RLock()
	subscribers := make([]func(*ConfigSnapshot), len(m.subscribers))
	copy(subscribers, m.subscribers)
	m.mu.RUnlock()

	for _, callback := range subscribers {
		go func(cb func(*ConfigSnapshot)) {
			defer func() {
				if r := recover(); r != nil {
					m.logger.Error("Panic in config subscriber callback", "panic", r)
				}
			}()
			cb(config)
		}(callback)
	}
}

// Refresh fetches a fresh configuration snapshot from the config-api
func (m *Manager) Refresh() error {
	m.logger.Info("Refreshing configuration from config-api")
	
	// Get current config as fallback
	current := m.GetCurrentConfig()
	if current == nil {
		current = &ConfigSnapshot{
			RuleWindowSeconds: 5,
			MaxFindings:       10000,
			DedupeCap:         100000,
			HotReload:         false,
			DebounceMs:        1000,
			LabelTTLSeconds:   300,
			NeverBlockLabels:  []string{"role:db", "role:control-plane"},
		}
	}
	
	snapshot := m.client.GetSnapshotWithFallback(current)
	m.updateConfig(snapshot)
	
	return nil
}
