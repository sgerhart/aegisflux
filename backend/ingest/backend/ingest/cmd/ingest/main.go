package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aegisflux/backend/ingest/internal/health"
	"aegisflux/backend/ingest/internal/server"
	"aegisflux/backend/ingest/protos"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Read environment variables
	grpcAddr := getEnv("INGEST_GRPC_ADDR", ":50051")
	httpAddr := getEnv("INGEST_HTTP_ADDR", ":9090")
	natsURL := getEnv("NATS_URL", "nats://localhost:4222")

	slog.Info("Starting ingest service",
		"grpc_addr", grpcAddr,
		"http_addr", httpAddr,
		"nats_url", natsURL)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	
	// Register the ingest service
	ingestServer, err := server.NewIngestServer(natsURL, logger)
	if err != nil {
		slog.Error("Failed to create ingest server", "error", err)
		os.Exit(1)
	}
	protos.RegisterIngestServer(grpcServer, ingestServer)

	// Start gRPC server
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		slog.Error("Failed to listen on gRPC address", "error", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("Starting gRPC server", "addr", grpcAddr)
		ingestServer.SetGRPCReady(true)
		if err := grpcServer.Serve(grpcListener); err != nil {
			slog.Error("gRPC server failed", "error", err)
			ingestServer.SetGRPCReady(false)
			os.Exit(1)
		}
	}()

	// Start HTTP server for health checks and metrics
	httpMux := http.NewServeMux()
	
	// Create health server
	healthServer := health.NewHealthServer(ingestServer.GetHealthChecker(), logger)
	healthServer.RegisterRoutes(httpMux)
	
	// Add Prometheus metrics handler
	httpMux.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: httpMux,
	}

	go func() {
		slog.Info("Starting HTTP server", "addr", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down servers...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Close ingest server (including NATS connection)
	if err := ingestServer.Close(); err != nil {
		slog.Error("Failed to close ingest server", "error", err)
	}

	// Shutdown HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("HTTP server shutdown failed", "error", err)
	}

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	slog.Info("Servers stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
