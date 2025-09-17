# AegisFlux Decision Service

The Decision Service is responsible for creating and managing agentic decision plans based on findings from the Correlator service.

## Features

- **Plan Management**: Create, store, and retrieve decision plans
- **Agentic Pipeline**: Process findings and generate response strategies (stub implementation)
- **NATS Integration**: Publish plan events and subscribe to findings
- **HTTP API**: RESTful API for plan management
- **Memory Store**: Thread-safe in-memory storage with capacity limits

## API Endpoints

### Plans
- `POST /plans` - Create a new decision plan
- `GET /plans` - List all plans
- `GET /plans/{id}` - Get a specific plan

### Health
- `GET /healthz` - Health check
- `GET /readyz` - Readiness check

## Configuration

Environment variables:

- `DECISION_HTTP_ADDR` - HTTP server address (default: `:8083`)
- `DECISION_NATS_URL` - NATS server URL (default: `nats://localhost:4222`)
- `DECISION_MAX_PLANS` - Maximum number of plans to store (default: `1000`)
- `DECISION_LOG_LEVEL` - Log level (default: `INFO`)

## Running the Service

### Prerequisites

1. Go 1.21 or later
2. NATS server running

### Local Development

```bash
# Install dependencies
go mod tidy

# Run the service
go run ./cmd/decision

# Or build and run
go build -o decision ./cmd/decision
./decision
```

### Docker

```bash
# Build the image
docker build -t aegisflux/decision:latest .

# Run the container
docker run -p 8083:8083 \
  -e DECISION_NATS_URL=nats://nats:4222 \
  aegisflux/decision:latest
```

## Creating Plans

### Using Finding ID

```bash
curl -X POST http://localhost:8083/plans \
  -H "Content-Type: application/json" \
  -d '{
    "finding_id": "finding-123",
    "strategy_mode": "conservative"
  }'
```

### Using Inline Finding

```bash
curl -X POST http://localhost:8083/plans \
  -H "Content-Type: application/json" \
  -d '{
    "finding": {
      "id": "finding-123",
      "severity": "high",
      "confidence": 0.8,
      "host_id": "web-01",
      "rule_id": "bash-exec-after-connect"
    },
    "strategy_mode": "aggressive"
  }'
```

## Plan Structure

```json
{
  "id": "plan-123",
  "finding_id": "finding-123",
  "name": "Stub Decision Plan",
  "description": "Plan description",
  "status": "pending",
  "strategy": {
    "mode": "balanced",
    "success_criteria": {
      "min_success_rate": 0.8,
      "max_failure_rate": 0.2,
      "timeout_seconds": 300
    },
    "rollback": {
      "enabled": true,
      "triggers": ["failure_rate_exceeded", "timeout"]
    },
    "control": {
      "manual_approval": false,
      "gates": ["validation", "testing"]
    }
  },
  "steps": ["Analyze finding", "Determine response strategy"],
  "created_at": "2025-09-17T16:30:00Z",
  "expires_at": "2025-09-17T17:30:00Z"
}
```

## Strategy Modes

- `conservative` - Low risk, thorough validation
- `balanced` - Moderate risk, standard procedures
- `aggressive` - Higher risk, faster execution

## Plan Statuses

- `pending` - Plan created, waiting to be executed
- `active` - Plan is currently being executed
- `completed` - Plan executed successfully
- `failed` - Plan execution failed
- `cancelled` - Plan was cancelled

## Development Notes

This is a scaffold implementation. The following components will be implemented in future prompts:

- **Agentic Pipeline**: Real AI-driven plan generation
- **Guardrails**: Safety and compliance checks
- **Plan Execution**: Automated plan execution engine
- **Monitoring**: Plan execution monitoring and metrics
- **Integration**: Full integration with other AegisFlux services

## Testing

```bash
# Test health endpoint
curl http://localhost:8083/healthz

# Test readiness endpoint
curl http://localhost:8083/readyz

# List all plans
curl http://localhost:8083/plans

# Get specific plan
curl http://localhost:8083/plans/plan-123
```