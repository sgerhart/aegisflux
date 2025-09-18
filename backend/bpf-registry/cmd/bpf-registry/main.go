package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aegisflux/backend/bpf-registry/internal/api"
	"aegisflux/backend/bpf-registry/internal/store"
)

// Configuration from environment variables
type Config struct {
	HTTPAddr    string
	DataDir     string
	LogLevel    string
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	config := &Config{
		HTTPAddr:    getEnv("BPF_REGISTRY_HTTP_ADDR", ":8084"),
		DataDir:     getEnv("BPF_REGISTRY_DATA_DIR", "/data/artifacts"),
		LogLevel:    getEnv("BPF_REGISTRY_LOG_LEVEL", "INFO"),
	}
	
	return config
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("Starting AegisFlux BPF Registry Service")

	// Load configuration
	config := loadConfig()
	logger.Info("Configuration loaded",
		"http_addr", config.HTTPAddr,
		"data_dir", config.DataDir,
		"log_level", config.LogLevel)

	// Create context for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create artifact store
	artifactStore, err := store.NewFileStore(config.DataDir, logger)
	if err != nil {
		logger.Error("Failed to create artifact store", "error", err)
		os.Exit(1)
	}
	logger.Info("Artifact store initialized", "data_dir", config.DataDir)

	// Create HTTP API
	httpAPI := api.NewHTTPAPI(artifactStore, logger)
	mux := httpAPI.SetupRoutes()

	// Create HTTP server
	server := &http.Server{
		Addr:    config.HTTPAddr,
		Handler: mux,
	}

	// Start HTTP server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", "addr", config.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutdown signal received, gracefully shutting down...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown failed", "error", err)
	} else {
		logger.Info("HTTP server shutdown complete")
	}

	logger.Info("BPF Registry service shutdown complete")
}