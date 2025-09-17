package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"aegisflux/backend/decision/internal/agents"
	"aegisflux/backend/decision/internal/api"
	"aegisflux/backend/decision/internal/config"
	"aegisflux/backend/decision/internal/guardrails"
	"aegisflux/backend/decision/internal/store"
)

// AppConfig represents the application configuration from environment variables
type AppConfig struct {
	HTTPAddr      string
	NATSURL       string
	ConfigAPIURL  string
	MaxPlans      int
	LogLevel      string
}

// loadConfig loads configuration from environment variables
func loadConfig() *AppConfig {
	appConfig := &AppConfig{
		HTTPAddr:      getEnv("DECISION_HTTP_ADDR", ":8083"),
		NATSURL:       getEnv("DECISION_NATS_URL", "nats://localhost:4222"),
		ConfigAPIURL:  getEnv("CONFIG_API_URL", "http://localhost:8085"),
		MaxPlans:      getEnvInt("DECISION_MAX_PLANS", 1000),
		LogLevel:      getEnv("DECISION_LOG_LEVEL", "INFO"),
	}
	
	return appConfig
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
		if intValue, err := fmt.Sscanf(value, "%d", &defaultValue); err == nil && intValue == 1 {
			return defaultValue
		}
	}
	return defaultValue
}

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("Starting AegisFlux Decision Service")

	// Load configuration
	appConfig := loadConfig()
	logger.Info("Configuration loaded",
		"http_addr", appConfig.HTTPAddr,
		"nats_url", appConfig.NATSURL,
		"config_api_url", appConfig.ConfigAPIURL,
		"max_plans", appConfig.MaxPlans,
		"log_level", appConfig.LogLevel)

	// Connect to NATS
	natsConn, err := nats.Connect(appConfig.NATSURL)
	if err != nil {
		logger.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer natsConn.Close()
	logger.Info("Connected to NATS")

	// Initialize configuration manager
	configManager := config.NewManager(appConfig.ConfigAPIURL, natsConn, logger)
	
	// Create environment defaults
	envDefaults := &config.ConfigSnapshot{
		DecisionMode:      getEnv("DECISION_MODE", "suggest"),
		MaxCanaryHosts:    getEnvInt("DECISION_MAX_CANARY_HOSTS", 5),
		DefaultTTLSeconds: getEnvInt("DECISION_DEFAULT_TTL_SECONDS", 3600),
		NeverBlockLabels:  []string{"role:db", "role:control-plane"},
	}
	
	// Initialize configuration manager with fallback defaults
	if err := configManager.Initialize(context.Background(), envDefaults); err != nil {
		logger.Warn("Failed to initialize configuration manager, using environment defaults", "error", err)
	}

	// Initialize guardrails with configuration
	guardrailsInstance := guardrails.NewGuardrails(logger)
	guardrailsInstance.UpdateConfig(configManager.GetCurrentConfig())
	
	// Subscribe to configuration changes
	configManager.Subscribe(func(snapshot *config.ConfigSnapshot) {
		guardrailsInstance.UpdateConfig(snapshot)
	})

	// Initialize agent runtime
	agentRuntime, err := agents.NewRuntime(logger)
	if err != nil {
		logger.Error("Failed to initialize agent runtime", "error", err)
		os.Exit(1)
	}
	logger.Info("Agent runtime initialized successfully")

	// Initialize store
	planStore := store.NewMemoryPlanStore(appConfig.MaxPlans, logger, natsConn)
	logger.Info("Memory plan store initialized", "max_plans", appConfig.MaxPlans)

	// Initialize HTTP API
	httpAPI := api.NewHTTPAPI(planStore, logger, natsConn, agentRuntime, guardrailsInstance)

	// Setup HTTP routes
	mux := httpAPI.SetupRoutes()

	// Create HTTP server
	server := &http.Server{
		Addr:    appConfig.HTTPAddr,
		Handler: mux,
	}

	// Start HTTP server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", "addr", appConfig.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Start cleanup goroutine
	cleanupTicker := time.NewTicker(5 * time.Minute)
	defer cleanupTicker.Stop()
	
	go func() {
		for {
			select {
			case <-cleanupTicker.C:
				if err := planStore.Cleanup(context.Background()); err != nil {
					logger.Error("Failed to cleanup expired plans", "error", err)
				}
			}
		}
	}()

	logger.Info("Decision service started successfully")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down decision service")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown failed", "error", err)
	}

	logger.Info("Decision service stopped")
}
