# Ingest Service

A gRPC service for ingesting events with streaming support.

## Features

- gRPC server for streaming event ingestion
- HTTP health check endpoint
- JSON structured logging with slog
- Environment-based configuration
- Graceful shutdown handling

## Running Locally

### Prerequisites

- Go 1.21 or later
- Protocol Buffers compiler (`protoc`)

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

- `INGEST_GRPC_ADDR`: gRPC server address (default: `:8080`)
- `INGEST_HTTP_ADDR`: HTTP server address (default: `:8081`)
- `NATS_URL`: NATS server URL (default: `nats://localhost:4222`)

### Example Usage

```bash
# Run with custom ports
INGEST_GRPC_ADDR=:9090 INGEST_HTTP_ADDR=:9091 ./ingest

# Check health
curl http://localhost:8081/healthz
```

### gRPC Service

The service implements the `Ingest` service with a `PostEvents` streaming RPC:

```protobuf
service Ingest {
  rpc PostEvents(stream Event) returns (Ack);
}
```

### Development

To regenerate protobuf stubs:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       protos/ingest.proto
```

## Architecture

- `cmd/ingest/main.go`: Application entry point
- `internal/server/grpc.go`: gRPC server implementation
- `protos/`: Protocol buffer definitions and generated code
- `Validator` and `Publisher` interfaces for extensibility

