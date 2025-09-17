package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"time"

	"aegisflux/backend/ingest/protos"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// EventData represents the JSON structure from the sample file
type EventData struct {
	TS           string                 `json:"ts"`
	HostID       string                 `json:"host_id"`
	PID          int                    `json:"pid"`
	UID          int                    `json:"uid"`
	ContainerID  *string                `json:"container_id"`
	BinaryPath   string                 `json:"binary_path"`
	BinarySHA256 *string                `json:"binary_sha256"`
	EventType    string                 `json:"event_type"`
	Args         map[string]interface{} `json:"args"`
	Context      map[string]interface{} `json:"context"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <json-file>\n", os.Args[0])
		os.Exit(1)
	}

	jsonFile := os.Args[1]
	grpcAddr := getEnv("INGEST_GRPC_ADDR", "localhost:50052")

	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Read and parse JSON file
	events, err := readJSONFile(jsonFile)
	if err != nil {
		logger.Error("Failed to read JSON file", "error", err)
		os.Exit(1)
	}

	logger.Info("Starting event sender", "grpc_addr", grpcAddr, "event_count", len(events))

	// Connect to gRPC server
	conn, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect to gRPC server", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := protos.NewIngestClient(conn)

	// Create streaming context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start streaming
	stream, err := client.PostEvents(ctx)
	if err != nil {
		logger.Error("Failed to create stream", "error", err)
		os.Exit(1)
	}

	// Send events
	successCount := 0
	for i, eventData := range events {
		event := convertToProtoEvent(eventData, i+1)
		
		logger.Info("Sending event", 
			"event_id", event.Id, 
			"event_type", event.Type, 
			"host_id", event.Metadata["host_id"])

		if err := stream.Send(event); err != nil {
			logger.Error("Failed to send event", "error", err, "event_id", event.Id)
			continue
		}
		successCount++
	}

	// Close stream and get response
	ack, err := stream.CloseAndRecv()
	if err != nil {
		logger.Error("Failed to close stream", "error", err)
		os.Exit(1)
	}

	logger.Info("Streaming completed", 
		"total_events", len(events), 
		"successful_events", successCount,
		"ack_ok", ack.Ok,
		"ack_message", ack.Message)
}

func readJSONFile(filename string) ([]EventData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Try to parse as single event first
	var singleEvent EventData
	if err := json.Unmarshal(data, &singleEvent); err == nil {
		return []EventData{singleEvent}, nil
	}

	// Try to parse as array of events
	var events []EventData
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return events, nil
}

func convertToProtoEvent(data EventData, index int) *protos.Event {
	// Parse timestamp
	timestamp := time.Now().UnixMilli()
	if data.TS != "" {
		if t, err := time.Parse(time.RFC3339, data.TS); err == nil {
			timestamp = t.UnixMilli()
		}
	}

	// Create metadata
	metadata := map[string]string{
		"host_id": data.HostID,
		"pid":     strconv.Itoa(data.PID),
		"uid":     strconv.Itoa(data.UID),
	}

	if data.ContainerID != nil {
		metadata["container_id"] = *data.ContainerID
	}
	if data.BinarySHA256 != nil {
		metadata["binary_sha256"] = *data.BinarySHA256
	}

	// Add context fields to metadata
	for k, v := range data.Context {
		if str, ok := v.(string); ok {
			metadata["context_"+k] = str
		} else if str, ok := v.([]interface{}); ok {
			// Convert array to comma-separated string
			var parts []string
			for _, item := range str {
				if s, ok := item.(string); ok {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				metadata["context_"+k] = fmt.Sprintf("%v", parts)
			}
		}
	}

	// Create payload from args and base64 encode it
	payload := []byte{}
	if argsJSON, err := json.Marshal(data.Args); err == nil {
		payload = []byte(base64.StdEncoding.EncodeToString(argsJSON))
	}

	return &protos.Event{
		Id:        fmt.Sprintf("event-%d-%d", time.Now().Unix(), index),
		Type:      data.EventType,
		Source:    data.BinaryPath,
		Timestamp: timestamp,
		Metadata:  metadata,
		Payload:   payload,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
