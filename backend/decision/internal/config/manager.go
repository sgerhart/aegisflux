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

// Manager handles configuration management with live updates
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
		"decision_mode", newConfig.DecisionMode,
		"max_canary_hosts", newConfig.MaxCanaryHosts,
		"default_ttl_seconds", newConfig.DefaultTTLSeconds,
		"never_block_labels", newConfig.NeverBlockLabels)
}

// applyConfigChange applies a specific configuration change to the snapshot
func (m *Manager) applyConfigChange(config *ConfigSnapshot, change *ConfigChangeMessage) {
	switch change.Key {
	case "decision.mode":
		var mode string
		if err := json.Unmarshal(change.Value, &mode); err == nil {
			config.DecisionMode = mode
		}
	case "decision.max_canary_hosts":
		var maxHosts int
		if err := json.Unmarshal(change.Value, &maxHosts); err == nil {
			config.MaxCanaryHosts = maxHosts
		} else if str, err := strconv.Atoi(string(change.Value)); err == nil {
			config.MaxCanaryHosts = str
		}
	case "decision.default_ttl_seconds":
		var ttl int
		if err := json.Unmarshal(change.Value, &ttl); err == nil {
			config.DefaultTTLSeconds = ttl
		} else if str, err := strconv.Atoi(string(change.Value)); err == nil {
			config.DefaultTTLSeconds = str
		}
	case "guardrails.never_block_labels":
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
			DecisionMode:      "suggest",
			MaxCanaryHosts:    5,
			DefaultTTLSeconds: 3600,
			NeverBlockLabels:  []string{"role:db", "role:control-plane"},
		}
	}
	
	snapshot := m.client.GetSnapshotWithFallback(current)
	m.updateConfig(snapshot)
	
	return nil
}
