# Ingest Service

A gRPC service for ingesting events with streaming support, JSON Schema validation, and NATS publishing.

## Features

- gRPC server for streaming event ingestion
- HTTP health check and readiness endpoints
- Prometheus metrics collection
- JSON Schema validation for events
- NATS publishing with automatic reconnection
- JSON structured logging with slog
- Environment-based configuration
- Graceful shutdown handling

## Running Locally

### Prerequisites

- Go 1.21 or later
- Protocol Buffers compiler (`protoc`)
- NATS server (optional, for full functionality)

### Build and Run

```bash
# Navigate to the ingest service directory
cd backend/ingest/backend/ingest

# Build the service
go build ./cmd/ingest

# Run the service
./ingest
```

### Environment Variables

- `INGEST_GRPC_ADDR`: gRPC server address (default: `:50051`)
- `INGEST_HTTP_ADDR`: HTTP server address (default: `:9090`)
- `NATS_URL`: NATS server URL (default: `nats://localhost:4222`)

### Example Usage

```bash
# Run with custom ports
INGEST_GRPC_ADDR=:9090 INGEST_HTTP_ADDR=:9091 ./ingest

# Run with custom NATS URL
NATS_URL=nats://nats-server:4222 ./ingest

# Check health endpoints
curl http://localhost:9090/healthz    # Health check (gRPC + NATS)
curl http://localhost:9090/readyz     # Readiness check (NATS + Schema)
curl http://localhost:9090/metrics    # Prometheus metrics
```

### Docker

The service includes a multi-stage Dockerfile that creates a minimal, secure container:

- **Base image**: `gcr.io/distroless/static-debian11` (minimal, no shell)
- **Build stage**: `golang:1.25-alpine` for compilation
- **Exposed ports**: 50051 (gRPC), 9090 (HTTP/metrics)
- **Size**: ~20MB final image

```bash
# Build Docker image
docker build -t aegisflux-ingest .

# Run with Docker
docker run -p 50051:50051 -p 9090:9090 \
  -e NATS_URL=nats://host.docker.internal:4222 \
  aegisflux-ingest

# Run with custom ports
docker run -p 50052:50051 -p 9091:9090 \
  -e NATS_URL=nats://host.docker.internal:4222 \
  aegisflux-ingest
```

### NATS Setup

To run with full NATS functionality:

```bash
# Install NATS server
brew install nats-server

# Start NATS server
nats-server

# Run the ingest service
./ingest
```

### gRPC Service

The service implements the `Ingest` service with a `PostEvents` streaming RPC:

```protobuf
service Ingest {
  rpc PostEvents(stream Event) returns (Ack);
}
```

### Event Schema

Events are validated against a JSON Schema with the following requirements:

- **Required fields**: `id`, `type`, `source`, `timestamp`
- **Event types**: `["security", "audit", "performance", "application", "system"]`
- **Timestamp**: Unix timestamp in milliseconds (minimum: 1)
- **Metadata**: Optional key-value string pairs
- **Payload**: Optional base64-encoded string

### Health Endpoints

- **GET /healthz**: Returns `200 {"ok":true}` if both gRPC and NATS are healthy
- **GET /readyz**: Returns `200 {"ok":true}` when NATS is connected and schema is compiled
- **GET /metrics**: Returns Prometheus metrics in text format

### Prometheus Metrics

- **events_total**: Total number of events processed successfully
- **events_invalid_total**: Total number of invalid events rejected
- **nats_publish_errors_total**: Total number of NATS publish errors

### NATS Publishing

Events are published to NATS with:

- **Subject**: `events.raw`
- **Headers**: 
  - `x-host-id`: Host ID from event metadata (if present)
  - `x-event-id`: Event ID
  - `x-event-type`: Event type
  - `x-event-source`: Event source
  - `x-timestamp`: Event timestamp
- **Payload**: JSON-encoded Event

### Development

To regenerate protobuf stubs:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       protos/ingest.proto
```

To run tests:

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/validate/ -v
go test ./internal/nats/ -v
```

## Architecture

- `cmd/ingest/main.go`: Application entry point
- `internal/server/grpc.go`: gRPC server implementation
- `internal/validate/schema.go`: JSON Schema validation
- `internal/nats/publish.go`: NATS publishing
- `internal/health/`: Health check endpoints and status management
- `internal/metrics/`: Prometheus metrics collection
- `protos/`: Protocol buffer definitions and generated code
- `schemas/`: JSON Schema definitions

## Error Handling

- **Validation errors**: Returns `InvalidArgument` gRPC status with warning logs
- **Publishing errors**: Returns `Unavailable` gRPC status with error logs
- **NATS connection**: Fails fast on startup if NATS unavailable
- **Timeouts**: 2-second timeout for per-event publishing
- **Graceful shutdown**: Handles SIGINT/SIGTERM signals and closes NATS connection

## Logging

- **Structured logging**: All logs include `event_id`, `event_type`, and `host_id` fields
- **Event processing**: Info-level logs for each event processed
- **Validation failures**: Warning-level logs with detailed error information
- **Publishing failures**: Error-level logs with context and error details
- **Graceful shutdown**: Info-level logs for connection cleanup
