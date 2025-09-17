# AegisFlux Correlator Service

The Correlator Service is responsible for consuming enriched events from the ETL pipeline and applying correlation rules to generate security findings.

## Features

- **Async NATS Consumer**: Subscribes to `events.enriched` subject with queue-based processing
- **Memory Store**: Thread-safe ring buffer for findings with LRU-based deduplication
- **Correlation Rules**: Simple rule engine for detecting suspicious activities
- **HTTP API**: REST endpoints for querying findings and health checks
- **Graceful Shutdown**: Proper cleanup on service termination

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   ETL-Enrich    │───▶│      NATS        │───▶│   Correlator    │
│   Service       │    │ events.enriched  │    │   Service       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                                         │
                                                         ▼
                                               ┌─────────────────┐
                                               │  Memory Store   │
                                               │ (Ring Buffer +  │
                                               │  LRU Cache)     │
                                               └─────────────────┘
                                                         │
                                                         ▼
                                               ┌─────────────────┐
                                               │   HTTP API      │
                                               │ /findings       │
                                               │ /healthz        │
                                               └─────────────────┘
```

## Configuration

Environment variables:

- `CORR_HTTP_ADDR`: HTTP server address (default: `:8080`)
- `CORR_NATS_URL`: NATS server URL (default: `nats://localhost:4222`)
- `CORR_MAX_FINDINGS`: Maximum findings to store in memory (default: `10000`)
- `CORR_DEDUPE_CAP`: Deduplication cache capacity (default: `100000`)

## API Endpoints

### GET /findings
Retrieve security findings with optional filters:

- `?host_id=<host>` - Filter by host ID
- `?severity=<level>` - Filter by minimum severity (low, medium, high, critical)
- `?limit=<number>` - Limit number of results

Example:
```bash
curl "http://localhost:8080/findings?severity=high&limit=10"
```

### POST /findings/reset
Clear all stored findings:

```bash
curl -X POST http://localhost:8080/findings/reset
```

### GET /healthz
Health check endpoint:

```bash
curl http://localhost:8080/healthz
```

### GET /readyz
Readiness check endpoint:

```bash
curl http://localhost:8080/readyz
```

## Correlation Rules

The service currently implements simple correlation rules:

1. **Suspicious Binary Execution**: Detects execution of binaries from suspicious paths
   - Paths: `/tmp/`, `/var/tmp/`, `/dev/shm/`, `/home/`, `/root/`
   - Severity: High
   - Confidence: 0.8

2. **Unusual Network Connection**: Detects connections to unusual ports
   - Ports: 4444, 5555, 6666, 7777, 8888, 9999
   - Severity: Medium
   - Confidence: 0.6

## Data Models

### Event
```json
{
  "timestamp": "2025-09-17T02:30:00Z",
  "host_id": "host-001",
  "event_type": "exec",
  "binary_path": "/tmp/suspicious_binary",
  "args": {"pid": 1234},
  "context": {"env": "prod"}
}
```

### Finding
```json
{
  "id": "host-001-suspicious_binary_exec-20250917-023000",
  "severity": "high",
  "confidence": 0.8,
  "status": "open",
  "host_id": "host-001",
  "cve": "",
  "evidence": [
    {
      "type": "event",
      "description": "Suspicious binary executed",
      "data": {"binary_path": "/tmp/suspicious_binary"},
      "timestamp": "2025-09-17T02:30:00Z"
    }
  ],
  "timestamp": "2025-09-17T02:30:01Z",
  "rule_id": "suspicious_binary_exec",
  "ttl_seconds": 3600
}
```

## Running the Service

### Prerequisites
- Go 1.21+
- NATS server running
- ETL-Enrich service publishing to `events.enriched`

### Development
```bash
cd backend/correlator
go mod tidy
go run cmd/correlator/main.go
```

### Production Build
```bash
cd backend/correlator
go build -o correlator cmd/correlator/main.go
./correlator
```

### Docker (if needed)
```bash
cd backend/correlator
docker build -t aegisflux-correlator .
docker run -p 8080:8080 aegisflux-correlator
```

## Testing

### Manual Testing
1. Start the correlator service
2. Send enriched events through the ETL pipeline
3. Check findings via HTTP API:
   ```bash
   curl http://localhost:8080/findings
   ```

### Health Check
```bash
curl http://localhost:8080/healthz
```

Expected response:
```json
{
  "status": "healthy",
  "timestamp": "2025-09-17T02:30:00Z",
  "stats": {
    "total_findings": 5,
    "max_findings": 10000,
    "dedupe_cap": 100000,
    "dedupe_size": 5
  }
}
```

## Integration with AegisFlux Stack

The correlator service integrates with the broader AegisFlux stack:

1. **ETL-Enrich Service** publishes enriched events to NATS
2. **Correlator Service** consumes these events and generates findings
3. **HTTP API** provides access to findings for downstream consumers
4. **Memory Store** provides fast access to recent findings with deduplication

This creates a complete security event processing pipeline from raw events to actionable findings.
