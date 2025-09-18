package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the agent configuration
type Config struct {
	HostID           string        `json:"host_id"`
	RegistryURL      string        `json:"registry_url"`
	PollInterval     time.Duration `json:"poll_interval"`
	NATSURL          string        `json:"nats_url"`
	VaultURL         string        `json:"vault_url"`
	VaultToken       string        `json:"vault_token"`
	PublicKey        string        `json:"public_key,omitempty"`
	CacheDir         string        `json:"cache_dir"`
	MaxPrograms      int           `json:"max_programs"`
	DefaultTTL       time.Duration `json:"default_ttl"`
	LogLevel         string        `json:"log_level"`
	TelemetrySubject string        `json:"telemetry_subject"`

	// HTTP server configuration
	HTTPPort    int    `json:"http_port"`
	HTTPAddress string `json:"http_address"`
	
	// Rollback thresholds
	RollbackMaxErrors       int     `json:"rollback_max_errors"`
	RollbackMaxViolations   int     `json:"rollback_max_violations"`
	RollbackMaxCPUPercent   float64 `json:"rollback_max_cpu_percent"`
	RollbackMaxLatencyMs    float64 `json:"rollback_max_latency_ms"`
	RollbackMaxMemoryKB     int64   `json:"rollback_max_memory_kb"`
	RollbackVerifierFailure bool    `json:"rollback_verifier_failure"`
	RollbackCheckInterval   int     `json:"rollback_check_interval_sec"`
	RollbackDelaySec        int     `json:"rollback_delay_sec"`
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		HostID:           getEnv("AGENT_HOST_ID", "localhost"),
		RegistryURL:      getEnv("AGENT_REGISTRY_URL", "http://localhost:8084"),
		PollInterval:     getDurationEnv("AGENT_POLL_INTERVAL_SEC", 30*time.Second),
		NATSURL:          getEnv("AGENT_NATS_URL", "nats://localhost:4222"),
		VaultURL:         getEnv("AGENT_VAULT_URL", "http://localhost:8200"),
		VaultToken:       getEnv("AGENT_VAULT_TOKEN", "dev-token"),
		PublicKey:        getEnv("AGENT_PUBLIC_KEY", ""),
		CacheDir:         getEnv("AGENT_CACHE_DIR", "/tmp/aegisflux-agent"),
		MaxPrograms:      getIntEnv("AGENT_MAX_PROGRAMS", 10),
		DefaultTTL:       getDurationEnv("AGENT_DEFAULT_TTL_SEC", 3600*time.Second),
		LogLevel:         getEnv("AGENT_LOG_LEVEL", "info"),
		TelemetrySubject: getEnv("AGENT_TELEMETRY_SUBJECT", "agent.telemetry"),

		// HTTP server configuration
		HTTPPort:    getIntEnv("AGENT_HTTP_PORT", 8080),
		HTTPAddress: getEnv("AGENT_HTTP_ADDRESS", ""),
		
		// Rollback thresholds
		RollbackMaxErrors:       getIntEnv("AGENT_ROLLBACK_MAX_ERRORS", 10),
		RollbackMaxViolations:   getIntEnv("AGENT_ROLLBACK_MAX_VIOLATIONS", 100),
		RollbackMaxCPUPercent:   getFloat64Env("AGENT_ROLLBACK_MAX_CPU_PERCENT", 80.0),
		RollbackMaxLatencyMs:    getFloat64Env("AGENT_ROLLBACK_MAX_LATENCY_MS", 1000.0),
		RollbackMaxMemoryKB:     getInt64Env("AGENT_ROLLBACK_MAX_MEMORY_KB", 100*1024),
		RollbackVerifierFailure: getBoolEnv("AGENT_ROLLBACK_VERIFIER_FAILURE", true),
		RollbackCheckInterval:   getIntEnv("AGENT_ROLLBACK_CHECK_INTERVAL_SEC", 30),
		RollbackDelaySec:        getIntEnv("AGENT_ROLLBACK_DELAY_SEC", 5),
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.HostID == "" {
		return fmt.Errorf("host_id cannot be empty")
	}
	if c.RegistryURL == "" {
		return fmt.Errorf("registry_url cannot be empty")
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("poll_interval must be positive")
	}
	if c.NATSURL == "" {
		return fmt.Errorf("nats_url cannot be empty")
	}
	if c.CacheDir == "" {
		return fmt.Errorf("cache_dir cannot be empty")
	}
	if c.MaxPrograms <= 0 {
		return fmt.Errorf("max_programs must be positive")
	}
	if c.DefaultTTL <= 0 {
		return fmt.Errorf("default_ttl must be positive")
	}
	return nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getIntEnv gets an integer environment variable with a default value
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getDurationEnv gets a duration environment variable with a default value
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}

// getFloat64Env gets a float64 environment variable with a default value
func getFloat64Env(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// getInt64Env gets an int64 environment variable with a default value
func getInt64Env(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getBoolEnv gets a bool environment variable with a default value
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
