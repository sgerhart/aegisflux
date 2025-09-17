# Event Sender

A simple Go client that streams events from JSON files to the ingest service.

## Usage

```bash
# Send single event from JSON file
go run cmd/sender/main.go /path/to/event.json

# Send multiple events from JSON array
go run cmd/sender/main.go /path/to/events.json

# Use custom gRPC address
INGEST_GRPC_ADDR=localhost:50052 go run cmd/sender/main.go /path/to/event.json
```

## JSON Format

The sender expects JSON files with the following structure:

### Single Event
```json
{
  "ts": "2025-09-16T12:00:01Z",
  "host_id": "host-001",
  "pid": 1234,
  "uid": 1001,
  "container_id": null,
  "binary_path": "/usr/bin/bash",
  "binary_sha256": null,
  "event_type": "exec",
  "args": {"argv":["/usr/bin/bash","-c","echo hi"]},
  "context": {"kernel":"6.6","tags":["dev"]}
}
```

### Multiple Events
```json
[
  { /* event 1 */ },
  { /* event 2 */ },
  { /* event 3 */ }
]
```

## Event Conversion

The sender converts JSON events to protobuf format:

- **id**: Generated as `event-{timestamp}-{index}`
- **type**: Uses `event_type` from JSON
- **source**: Uses `binary_path` from JSON
- **timestamp**: Parses `ts` field (RFC3339 format)
- **metadata**: Includes `host_id`, `pid`, `uid`, `container_id`, `binary_sha256`, and context fields
- **payload**: Base64-encoded JSON of the `args` field

## Environment Variables

- `INGEST_GRPC_ADDR`: gRPC server address (default: `localhost:50052`)

## Examples

```bash
# Test with sample event
go run cmd/sender/main.go ../../../data/samples/event_exec.json

# Test with multiple events
go run cmd/sender/main.go ../../../data/samples/events_multiple.json
```

