package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aegisflux/correlator/internal/api"
	"github.com/aegisflux/correlator/internal/config"
	"github.com/aegisflux/correlator/internal/metrics"
	correlatorNats "github.com/aegisflux/correlator/internal/nats"
	"github.com/aegisflux/correlator/internal/rules"
	"github.com/aegisflux/correlator/internal/store"
	"github.com/nats-io/nats.go"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("Starting AegisFlux Correlator Service")

	// Load environment variables with defaults
	httpAddr := getEnv("CORR_HTTP_ADDR", ":8080")
	natsURL := getEnv("CORR_NATS_URL", "nats://localhost:4222")
	configAPIURL := getEnv("CONFIG_API_URL", "http://localhost:8085")
	maxFindings := getEnvInt("CORR_MAX_FINDINGS", 10000)
	dedupeCap := getEnvInt("CORR_DEDUPE_CAP", 100000)
	rulesDir := getEnv("CORR_RULES_DIR", "rules.d")
	hotReload := strings.ToLower(getEnv("CORR_HOT_RELOAD", "false")) == "true"
	debounceMs := getEnvInt("CORR_DEBOUNCE_MS", 1000)

	logger.Info("Configuration loaded",
		"http_addr", httpAddr,
		"nats_url", natsURL,
		"config_api_url", configAPIURL,
		"max_findings", maxFindings,
		"dedupe_cap", dedupeCap,
		"rules_dir", rulesDir,
		"hot_reload", hotReload,
		"debounce_ms", debounceMs)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to NATS
	nc, err := nats.Connect(natsURL)
	if err != nil {
		logger.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	logger.Info("Connected to NATS")

	// Initialize configuration manager
	configManager := config.NewManager(configAPIURL, nc, logger)
	
	// Create environment defaults
	envDefaults := &config.ConfigSnapshot{
		RuleWindowSeconds: getEnvInt("CORR_RULE_WINDOW_SEC", 5),
		MaxFindings:       maxFindings,
		DedupeCap:         dedupeCap,
		HotReload:         hotReload,
		DebounceMs:        debounceMs,
		LabelTTLSeconds:   getEnvInt("CORR_LABEL_TTL_SEC", 300),
		NeverBlockLabels:  []string{"role:db", "role:control-plane"},
	}
	
	// Initialize configuration manager with fallback defaults
	if err := configManager.Initialize(ctx, envDefaults); err != nil {
		logger.Warn("Failed to initialize configuration manager, using environment defaults", "error", err)
	}

	// Get current configuration
	currentConfig := configManager.GetCurrentConfig()
	
	// Create memory store with configuration values
	memoryStore := store.NewMemoryStore(currentConfig.MaxFindings, currentConfig.DedupeCap)
	logger.Info("Memory store initialized", "max_findings", currentConfig.MaxFindings, "dedupe_cap", currentConfig.DedupeCap)

	// Create rule loader with configuration values
	ruleLoader := rules.NewLoader(rulesDir, currentConfig.HotReload, currentConfig.DebounceMs, logger)
	
	// Create metrics
	prometheusMetrics := metrics.NewMetrics()
	
	// Create override manager with metrics
	overrideManager := rules.NewOverrideManagerWithMetrics(logger, prometheusMetrics)
	
	// Load initial rules snapshot
	_, err = ruleLoader.LoadSnapshot()
	if err != nil {
		logger.Error("Failed to load initial rules snapshot", "error", err)
		os.Exit(1)
	}
	
	// Update metrics after loading rules
	snapshot := ruleLoader.GetSnapshot()
	prometheusMetrics.SetRulesLoaded(float64(len(snapshot.Rules)))
	prometheusMetrics.SetRulesOverrides(float64(len(overrideManager.ListOverrides())))
	
	// Start rule file watcher if hot reload is enabled
	if err := ruleLoader.WatchForChanges(); err != nil {
		logger.Error("Failed to start rule watcher", "error", err)
		os.Exit(1)
	}

	// Create NATS subscriber with rule loader, override manager, and metrics
	subscriber := correlatorNats.NewSubscriber(nc, memoryStore, "correlator", ruleLoader, overrideManager, prometheusMetrics, logger)
	
	// Subscribe to configuration changes
	configManager.Subscribe(func(snapshot *config.ConfigSnapshot) {
		logger.Info("Configuration updated, applying changes",
			"rule_window_seconds", snapshot.RuleWindowSeconds,
			"max_findings", snapshot.MaxFindings,
			"dedupe_cap", snapshot.DedupeCap,
			"hot_reload", snapshot.HotReload,
			"debounce_ms", snapshot.DebounceMs,
			"label_ttl_seconds", snapshot.LabelTTLSeconds,
			"never_block_labels", snapshot.NeverBlockLabels)
		
		// Update rule loader configuration if hot reload setting changed
		if snapshot.HotReload != currentConfig.HotReload {
			logger.Info("Hot reload setting changed", "new_value", snapshot.HotReload)
			// Note: Hot reload changes would require restart to take effect
		}
		
		// Update debounce setting if changed
		if snapshot.DebounceMs != currentConfig.DebounceMs {
			logger.Info("Debounce setting changed", "new_value", snapshot.DebounceMs)
			// Note: Debounce changes would require restart to take effect
		}
		
		// Update current config reference
		currentConfig = snapshot
	})
	
	// Start window buffer GC routine
	subscriber.StartGC(30 * time.Second) // GC every 30 seconds
	defer subscriber.StopGC()

	// Create HTTP API
	httpAPI := api.NewHTTPAPI(memoryStore, ruleLoader, overrideManager, prometheusMetrics, nc)
	mux := http.NewServeMux()
	httpAPI.SetupRoutes(mux)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}

	// Start HTTP server
	go func() {
		logger.Info("Starting HTTP server", "addr", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	// Start NATS subscriber
	go func() {
		logger.Info("Starting NATS subscriber")
		if err := subscriber.Subscribe(ctx); err != nil {
			logger.Error("NATS subscriber error", "error", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Correlator service started successfully")
	<-sigChan

	logger.Info("Shutting down correlator service...")

	// Cancel context to stop NATS subscriber
	cancel()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	logger.Info("Correlator service stopped")
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an environment variable as an integer with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
