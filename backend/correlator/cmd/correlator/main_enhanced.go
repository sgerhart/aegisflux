package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sgerhart/aegisflux/backend/correlator/internal/api"
	"github.com/sgerhart/aegisflux/backend/correlator/internal/metrics"
	"github.com/sgerhart/aegisflux/backend/correlator/internal/model"
	"github.com/sgerhart/aegisflux/backend/correlator/internal/nats"
	"github.com/sgerhart/aegisflux/backend/correlator/internal/rules"
	"github.com/sgerhart/aegisflux/backend/correlator/internal/store"
	"github.com/nats-io/nats.go"
)

func main() {
	// Set up logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("Starting AegisFlux Correlator Service")

	// Configuration
	natsURL := getEnv("NATS_URL", "nats://localhost:4222")
	httpAddr := getEnv("CORRELATOR_HTTP_ADDR", ":8085")
	rulesDir := getEnv("CORRELATOR_RULES_DIR", "./rules.d")
	hotReload := getEnv("CORRELATOR_HOT_RELOAD", "true") == "true"
	maxPlans := getEnvInt("CORRELATOR_MAX_PLANS", 1000)

	// Connect to NATS
	nc, err := nats.Connect(natsURL)
	if err != nil {
		logger.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	logger.Info("Connected to NATS", "url", natsURL)

	// Create services
	store := store.NewMemoryStore(maxPlans)
	metrics := metrics.NewMetrics()
	ruleLoader := rules.NewLoader(rulesDir, hotReload, 1000, logger)
	overrideManager := rules.NewOverrideManager()
	findingGenerator := rules.NewFindingGenerator(logger)
	findingPublisher := rules.NewFindingPublisher(nc, logger)
	decisionIntegration := rules.NewDecisionIntegration(nc, logger)

	// Load initial rules
	snapshot, err := ruleLoader.LoadSnapshot()
	if err != nil {
		logger.Error("Failed to load rules", "error", err)
		os.Exit(1)
	}
	logger.Info("Loaded rules", "count", len(snapshot.Rules))

	// Start hot reload if enabled
	if hotReload {
		if err := ruleLoader.WatchForChanges(); err != nil {
			logger.Error("Failed to start rule watcher", "error", err)
			os.Exit(1)
		}
		logger.Info("Started rule hot reload")
	}

	// Create HTTP API
	httpAPI := api.NewHTTPAPI(store, ruleLoader, overrideManager, metrics, nc)

	// Set up HTTP server
	mux := http.NewServeMux()
	httpAPI.SetupRoutes(mux)

	server := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}

	// Start HTTP server
	go func() {
		logger.Info("Starting HTTP server", "addr", httpAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	// Create NATS subscriber
	subscriber := nats.NewSubscriber(nc, logger)

	// Set up finding processing
	subscriber.SetupFindingProcessing(store, ruleLoader, findingGenerator, findingPublisher, decisionIntegration)

	// Start NATS subscriber
	go func() {
		logger.Info("Starting NATS subscriber")
		if err := subscriber.Start(); err != nil {
			logger.Error("NATS subscriber error", "error", err)
		}
	}()

	// Wait for shutdown signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	<-shutdown
	logger.Info("Shutdown signal received")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	// Stop NATS subscriber
	if err := subscriber.Stop(); err != nil {
		logger.Error("NATS subscriber shutdown error", "error", err)
	}

	logger.Info("Correlator service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := time.ParseDuration(value); err == nil {
			return int(intValue.Seconds())
		}
	}
	return defaultValue
}
