package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"aegisflux/agents/local-agent/internal/agent"
	"aegisflux/agents/local-agent/internal/config"
	"aegisflux/agents/local-agent/internal/logging"
)

func main() {
	// Load configuration first
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Setup structured logger
	logger := logging.NewLogger(cfg)

	logger.LogSystemEvent("agent_started")

	// Log configuration (without sensitive data)
	logger.LogSystemEvent("config_loaded",
		"host_id", cfg.HostID,
		"registry_url", cfg.RegistryURL,
		"poll_interval", cfg.PollInterval,
		"nats_url", cfg.NATSURL,
		"cache_dir", cfg.CacheDir,
		"max_programs", cfg.MaxPrograms)

	// Create agent
	agentInstance, err := agent.New(logger.Logger, cfg)
	if err != nil {
		logger.Error("Failed to create agent", "error", err)
		os.Exit(1)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Start the agent
	logger.Info("Starting agent main loop")
	if err := agentInstance.Run(ctx); err != nil {
		logger.Error("Agent run failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Agent shutdown complete")
}
