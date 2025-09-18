package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"aegisflux/backend/orchestrator/internal/decision"
	"aegisflux/backend/orchestrator/internal/ebpf"
	"aegisflux/backend/orchestrator/internal/rollout"
)

func main() {
	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("Starting AegisFlux Orchestrator")

	// Load configuration from environment
	config := loadConfig()

	// Create eBPF orchestrator
	ebpfOrchestrator, err := ebpf.NewOrchestrator(logger, config)
	if err != nil {
		logger.Error("Failed to create eBPF orchestrator", "error", err)
		os.Exit(1)
	}

	// Create rollout manager
	rolloutManager, err := rollout.NewBPFRolloutManager(logger, config)
	if err != nil {
		logger.Error("Failed to create rollout manager", "error", err)
		os.Exit(1)
	}

	// Create decision processor
	decisionProcessor, err := decision.NewDecisionProcessor(logger, config, ebpfOrchestrator, rolloutManager)
	if err != nil {
		logger.Error("Failed to create decision processor", "error", err)
		os.Exit(1)
	}

	// Create HTTP server
	mux := http.NewServeMux()
	
	// Setup decision integration API
	decisionAPI := decision.NewAPI(logger, decisionProcessor)
	decisionAPI.SetupRoutes(mux)
	
	// Setup rollout API
	rolloutAPI := rollout.NewRolloutAPI(logger, rolloutManager)
	rolloutAPI.SetupRoutes(mux)

	server := &http.Server{
		Addr:    config.HTTPAddr,
		Handler: mux,
	}

	// Start HTTP server
	go func() {
		logger.Info("Starting HTTP server", "addr", config.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	// Start decision processor
	if err := decisionProcessor.Start(context.Background()); err != nil {
		logger.Error("Failed to start decision processor", "error", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("Shutting down orchestrator")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30)
	defer cancel()

	if err := decisionProcessor.Stop(); err != nil {
		logger.Error("Failed to stop decision processor", "error", err)
	}

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Failed to shutdown HTTP server", "error", err)
	}

	logger.Info("Orchestrator shutdown complete")
}

// Config holds the orchestrator configuration
type Config struct {
	NATSURL          string
	HTTPAddr         string
	BPFRegistryURL   string
	DecisionAPIURL   string
	VaultAddr        string
	VaultToken       string
	LogLevel         string
	TemplateDir      string
	CacheDir         string
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	return &Config{
		NATSURL:          getEnv("ORCHESTRATOR_NATS_URL", "nats://localhost:4222"),
		HTTPAddr:         getEnv("ORCHESTRATOR_HTTP_ADDR", ":8084"),
		BPFRegistryURL:   getEnv("BPF_REGISTRY_URL", "http://localhost:8090"),
		DecisionAPIURL:   getEnv("DECISION_API_URL", "http://localhost:8083"),
		VaultAddr:        getEnv("VAULT_ADDR", "http://localhost:8200"),
		VaultToken:       getEnv("VAULT_TOKEN", "root"),
		LogLevel:         getEnv("ORCHESTRATOR_LOG_LEVEL", "info"),
		TemplateDir:      getEnv("ORCHESTRATOR_TEMPLATE_DIR", "/templates"),
		CacheDir:         getEnv("ORCHESTRATOR_CACHE_DIR", "/tmp/orchestrator"),
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
