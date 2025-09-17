package main

import (
	"log/slog"
	"os"
	"time"

	"aegisflux/backend/ingest/internal/metrics"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	
	// Create metrics instance
	m := metrics.NewMetrics()
	
	logger.Info("Testing metrics collection...")
	
	// Simulate some events
	for i := 0; i < 5; i++ {
		m.IncrementEventsTotal()
		logger.Info("Incremented events_total")
		time.Sleep(100 * time.Millisecond)
	}
	
	// Simulate some invalid events
	for i := 0; i < 2; i++ {
		m.IncrementEventsInvalid()
		logger.Info("Incremented events_invalid_total")
		time.Sleep(100 * time.Millisecond)
	}
	
	// Simulate some NATS publish errors
	for i := 0; i < 1; i++ {
		m.IncrementNatsPublishErrors()
		logger.Info("Incremented nats_publish_errors_total")
		time.Sleep(100 * time.Millisecond)
	}
	
	logger.Info("Metrics test completed. Check /metrics endpoint to see the counters.")
}
