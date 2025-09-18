package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("Starting AegisFlux Orchestrator (Simplified)")

	// Load configuration from environment
	config := loadConfig()

	// Create HTTP server with basic endpoints
	mux := http.NewServeMux()
	
	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","service":"orchestrator"}`)
	})

	// Basic eBPF rendering endpoint
	mux.HandleFunc("/render", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("Received eBPF render request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"rendered","artifact_id":"test-artifact-123"}`)
	})

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

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("Shutting down orchestrator")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Failed to shutdown HTTP server", "error", err)
	}

	logger.Info("Orchestrator shutdown complete")
}

// Config holds the orchestrator configuration
type Config struct {
	HTTPAddr string
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	return &Config{
		HTTPAddr: getEnv("ORCHESTRATOR_HTTP_ADDR", ":8084"),
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
