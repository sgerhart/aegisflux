# Seg Maps API Implementation

This document describes the implementation of the `/seg/maps` API endpoint as specified in `prompts/orchestrator/seg_maps_api.md`.

## Features Implemented

### 1. Schema Validation for MapSnapshot
- **Location**: `internal/api/seg_maps_http.go`
- **Dependency**: `github.com/xeipuuv/gojsonschema v1.2.0`
- **Schema**: Validates against `schemas/mapsnapshot.json`
- **Validation**: Ensures all required fields are present and data types are correct

### 2. NATS Publishing
- **Subject**: `actions.seg.maps`
- **Payload**: JSON containing MapSnapshot, target hosts, and timestamp
- **Integration**: Uses existing NATS connection from orchestrator

### 3. Promote/Rollback Endpoints
- **Endpoints**: 
  - `POST /seg/maps/promote` - For canary/enforce transitions
  - `POST /seg/maps/rollback` - For rollback operations
- **Status**: Stub implementations as requested

## API Endpoints

### POST /seg/maps
Accepts MapSnapshot JSON and publishes to NATS.

**Request Body:**
```json
{
  "version": 1,
  "service_id": 123,
  "ttl_seconds": 300,
  "edges": [
    {
      "dst_cidr": "192.168.1.0/24",
      "proto": "tcp",
      "port": 80
    }
  ],
  "allow_cidrs": [
    {
      "cidr": "10.0.0.0/8",
      "proto": "tcp",
      "port": 443
    }
  ],
  "meta": {
    "description": "Test segmentation map"
  }
}
```

**Query Parameters:**
- `target_host` (optional): Comma-separated list of target hosts. Defaults to `localhost` if not provided.

**Response:**
```json
{
  "accepted": true,
  "service_id": 123,
  "target_hosts": ["host1", "host2"],
  "timestamp": 1640995200
}
```

### POST /seg/maps/promote
Stub endpoint for promote operations.

**Response:**
```json
{
  "status": "promote_stub",
  "message": "Promote endpoint - implementation pending"
}
```

### POST /seg/maps/rollback
Stub endpoint for rollback operations.

**Response:**
```json
{
  "status": "rollback_stub",
  "message": "Rollback endpoint - implementation pending"
}
```

## Usage

### Starting the Server
```bash
# Set NATS URL (optional, defaults to nats://localhost:4222)
export NATS_URL="nats://localhost:4222"

# Set HTTP address (optional, defaults to :8081)
export ORCH_HTTP_ADDR=":8081"

# Run the orchestrator
go run cmd/orchestrator/main.go
```

### Testing the API
Use the provided test script:
```bash
./example_usage.sh
```

Or test manually with curl:
```bash
curl -X POST http://localhost:8081/seg/maps \
  -H "Content-Type: application/json" \
  -d '{"version":1,"service_id":123,"ttl_seconds":300}'
```

## Architecture

### Components

1. **SegMapsHandler**: Main handler for MapSnapshot processing
   - Schema validation
   - NATS publishing
   - Error handling

2. **Server**: HTTP server with routing
   - Integrates SegMapsHandler
   - Provides promote/rollback stubs
   - Health check endpoint

3. **MapSnapshot**: Go struct matching JSON schema
   - Version, ServiceID, TTL
   - Edges and AllowCIDRs arrays
   - Optional metadata

### NATS Message Format
Published to `actions.seg.maps`:
```json
{
  "snapshot": { /* MapSnapshot object */ },
  "target_hosts": ["host1", "host2"],
  "timestamp": "1640995200"
}
```

## Testing

Run the test suite:
```bash
go test ./internal/api/... -v
```

Tests cover:
- Valid MapSnapshot processing
- Invalid JSON handling
- Schema validation errors
- NATS publishing (with mock connection)

## Dependencies

- `github.com/nats-io/nats.go v1.31.0` - NATS client
- `github.com/xeipuuv/gojsonschema v1.2.0` - JSON schema validation
- `github.com/gorilla/mux v1.8.1` - HTTP routing (existing)

## Error Handling

- **400 Bad Request**: Invalid JSON or schema validation failures
- **405 Method Not Allowed**: Non-POST requests to endpoints
- **500 Internal Server Error**: NATS publishing failures

All errors include descriptive messages to help with debugging.
