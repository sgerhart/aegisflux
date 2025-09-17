package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
)

func main() {
	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	natsURL := getEnv("NATS_URL", "nats://localhost:4222")
	subject := getEnv("NATS_SUBJECT", "events.raw")

	logger.Info("Connecting to NATS", "url", natsURL)

	// Connect to NATS
	nc, err := nats.Connect(natsURL)
	if err != nil {
		logger.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	logger.Info("Connected to NATS", "url", natsURL)

	// Subscribe to events
	sub, err := nc.Subscribe(subject, func(m *nats.Msg) {
		logger.Info("Received event", 
			"subject", m.Subject,
			"reply", m.Reply,
			"headers", m.Header,
			"data_size", len(m.Data))

		// Try to parse as JSON for pretty printing
		var eventData map[string]interface{}
		if err := json.Unmarshal(m.Data, &eventData); err == nil {
			if prettyJSON, err := json.MarshalIndent(eventData, "", "  "); err == nil {
				fmt.Println("Event JSON:")
				fmt.Println(string(prettyJSON))
			}
		} else {
			fmt.Printf("Raw data: %s\n", string(m.Data))
		}
		fmt.Println("---")
	})
	if err != nil {
		logger.Error("Failed to subscribe", "error", err)
		os.Exit(1)
	}
	defer sub.Unsubscribe()

	logger.Info("Subscribed to events", "subject", subject)
	logger.Info("Waiting for events... (Press Ctrl+C to stop)")

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logger.Info("Shutting down...")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
