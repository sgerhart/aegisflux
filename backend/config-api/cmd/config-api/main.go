package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"aegisflux/backend/config-api/internal/api"
	"aegisflux/backend/config-api/internal/store"
)

// Configuration from environment variables
type Config struct {
	HTTPAddr      string
	PGHost        string
	PGPort        string
	PGUser        string
	PGPass        string
	PGDB          string
	NATSURL       string
	LogLevel      string
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	config := &Config{
		HTTPAddr:      getEnv("CONFIG_API_HTTP_ADDR", ":8085"),
		PGHost:        getEnv("PG_HOST", "localhost"),
		PGPort:        getEnv("PG_PORT", "5432"),
		PGUser:        getEnv("PG_USER", "postgres"),
		PGPass:        getEnv("PG_PASS", "password"),
		PGDB:          getEnv("PG_DB", "aegisflux"),
		NATSURL:       getEnv("NATS_URL", "nats://localhost:4222"),
		LogLevel:      getEnv("LOG_LEVEL", "INFO"),
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
	// Load configuration
	config := loadConfig()
	
	// Setup logger
	logLevel := slog.LevelInfo
	if config.LogLevel == "DEBUG" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	
	logger.Info("Starting AegisFlux Config API Service", 
		"http_addr", config.HTTPAddr,
		"pg_host", config.PGHost,
		"pg_port", config.PGPort,
		"pg_db", config.PGDB,
		"nats_url", config.NATSURL,
		"log_level", config.LogLevel)

	// Initialize NATS connection
	natsConn, err := nats.Connect(config.NATSURL)
	if err != nil {
		logger.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer natsConn.Close()
	logger.Info("Connected to NATS")

	// Initialize database store
	dbStore, err := store.NewPostgresStore(config.PGHost, config.PGPort, config.PGUser, config.PGPass, config.PGDB, logger)
	if err != nil {
		logger.Error("Failed to initialize database store", "error", err)
		os.Exit(1)
	}
	defer dbStore.Close()
	logger.Info("Connected to PostgreSQL database")

	// Initialize API handler
	apiHandler := api.NewHandler(dbStore, natsConn, logger)

	// Setup HTTP server
	server := &http.Server{
		Addr:         config.HTTPAddr,
		Handler:      apiHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", "addr", config.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	logger.Info("Config API service started successfully")

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server exited")
}
