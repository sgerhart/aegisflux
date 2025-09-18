# AegisFlux Local Agent

The AegisFlux Local Agent is a Go-based agent that runs on target hosts to automatically load and manage eBPF programs from the BPF Registry. It provides automated lifecycle management, telemetry reporting, and TTL-based program unloading.

## Features

- **Automatic Polling**: Polls the BPF Registry every `AGENT_POLL_INTERVAL_SEC` for new artifacts
- **Artifact Verification**: Verifies artifact signatures using Vault-provided public keys
- **eBPF Program Loading**: Uses libbpf to load and attach BPF programs to kernel hooks
- **Parameter Application**: Applies runtime parameters to BPF maps and programs
- **TTL Management**: Automatically unloads programs after their TTL expires
- **Telemetry Reporting**: Emits comprehensive telemetry to NATS
- **Automatic Rollback**: Monitors telemetry thresholds and automatically unloads problematic programs
- **Orchestrator Integration**: Responds to rollback signals from the orchestrator
- **Threshold Monitoring**: Configurable thresholds for errors, violations, CPU, latency, and memory
- **HTTP API**: RESTful API for health checks, status, metrics, and program management
- **Structured Logging**: JSON logging with systemd integration for production environments
- **Systemd Integration**: Full systemd service support with notifications and watchdog
- **Observability**: Comprehensive monitoring, metrics, and observability features
- **Graceful Shutdown**: Cleanly unloads all programs on shutdown
- **Docker Support**: Containerized deployment with proper privileges

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   BPF Registry  │    │   NATS Cluster  │    │   Vault Server  │
│                 │    │                 │    │                 │
│ - Artifacts     │◄───┤ - Telemetry     │    │ - Signatures    │
│ - Metadata      │    │ - Events        │    │ - Public Keys   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       ▲                       │
         │                       │                       │
         ▼                       │                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Local Agent                                  │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │   Registry  │  │    BPF      │  │ Telemetry   │            │
│  │   Client    │  │   Loader    │  │   Sender    │            │
│  │             │  │             │  │             │            │
│  │ - Polling   │  │ - Loading   │  │ - Reporting │            │
│  │ - Download  │  │ - Attaching │  │ - Events    │            │
│  │ - Verify    │  │ - TTL Mgmt  │  │ - Heartbeat │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────┐
│   Kernel        │
│                 │
│ - XDP Programs  │
│ - LSM Programs  │
│ - Kprobe Hooks  │
└─────────────────┘
```

## HTTP API

The agent exposes a RESTful HTTP API for monitoring and management:

### Endpoints

- **`GET /healthz`** - Health check endpoint (returns 200 if healthy)
- **`GET /status`** - Detailed status with applied artifacts and TTL information
- **`GET /metrics`** - Prometheus-style metrics (counters and gauges)
- **`GET /programs`** - Detailed information about loaded programs
- **`GET /rollbacks`** - History of rollback operations
- **`GET /telemetry`** - Current telemetry data for all programs
- **`GET /version`** - Agent version and build information

### Example Usage

```bash
# Health check
curl http://localhost:8080/healthz

# Get detailed status
curl http://localhost:8080/status | jq

# Get metrics
curl http://localhost:8080/metrics | jq

# Get program details
curl http://localhost:8080/programs | jq
```

For detailed API documentation, see [OBSERVABILITY.md](docs/OBSERVABILITY.md).

## Configuration

The agent is configured through environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT_HOST_ID` | `localhost` | Unique identifier for this host |
| `AGENT_REGISTRY_URL` | `http://localhost:8084` | BPF Registry API URL |
| `AGENT_POLL_INTERVAL_SEC` | `30` | Polling interval in seconds |
| `AGENT_NATS_URL` | `nats://localhost:4222` | NATS connection URL |
| `AGENT_VAULT_URL` | `http://localhost:8200` | Vault server URL |
| `AGENT_VAULT_TOKEN` | `dev-token` | Vault authentication token |
| `AGENT_PUBLIC_KEY` | `` | Public key for signature verification |
| `AGENT_CACHE_DIR` | `/tmp/aegisflux-agent` | Cache directory for artifacts |
| `AGENT_MAX_PROGRAMS` | `10` | Maximum concurrent programs |
| `AGENT_DEFAULT_TTL_SEC` | `3600` | Default TTL in seconds |
| `AGENT_LOG_LEVEL` | `info` | Logging level |
| `AGENT_TELEMETRY_SUBJECT` | `agent.telemetry` | NATS subject for telemetry |
| `AGENT_HTTP_PORT` | `8080` | HTTP API server port |
| `AGENT_HTTP_ADDRESS` | `` | HTTP API server address (empty = all interfaces) |
| `AGENT_ROLLBACK_MAX_ERRORS` | `10` | Maximum errors before rollback |
| `AGENT_ROLLBACK_MAX_VIOLATIONS` | `100` | Maximum violations before rollback |
| `AGENT_ROLLBACK_MAX_CPU_PERCENT` | `80.0` | Maximum CPU usage before rollback |
| `AGENT_ROLLBACK_MAX_LATENCY_MS` | `1000.0` | Maximum latency before rollback |
| `AGENT_ROLLBACK_MAX_MEMORY_KB` | `102400` | Maximum memory usage before rollback |
| `AGENT_ROLLBACK_VERIFIER_FAILURE` | `true` | Rollback on verifier failures |
| `AGENT_ROLLBACK_CHECK_INTERVAL_SEC` | `30` | Telemetry check interval |
| `AGENT_ROLLBACK_DELAY_SEC` | `5` | Rollback delay after threshold exceeded |

## Building

### Prerequisites

- Go 1.21 or later
- Docker (optional)
- Make (optional)

### Using Make

```bash
# Build the agent
make build

# Build for multiple platforms
make build-all

# Run in development mode
make run-dev

# Build Docker image
make docker-build

# Run Docker container
make docker-run
```

### Manual Build

```bash
# Download dependencies
go mod download

# Build the agent
go build -o local-agent .

# Run the agent
./local-agent
```

## Usage

### Local Development

```bash
# Set environment variables
export AGENT_HOST_ID="host-001"
export AGENT_REGISTRY_URL="http://localhost:8084"
export AGENT_NATS_URL="nats://localhost:4222"

# Run the agent
./local-agent
```

### Docker Deployment

```bash
# Build the image
docker build -t aegisflux/local-agent:latest .

# Run with privileged access (required for eBPF)
docker run --rm -it \
  --privileged \
  -v /sys/fs/bpf:/sys/fs/bpf \
  -e AGENT_HOST_ID=host-001 \
  -e AGENT_REGISTRY_URL=http://registry:8084 \
  -e AGENT_NATS_URL=nats://nats:4222 \
  aegisflux/local-agent:latest
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: aegisflux-agent
spec:
  selector:
    matchLabels:
      app: aegisflux-agent
  template:
    metadata:
      labels:
        app: aegisflux-agent
    spec:
      containers:
      - name: agent
        image: aegisflux/local-agent:latest
        securityContext:
          privileged: true
        env:
        - name: AGENT_HOST_ID
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: AGENT_REGISTRY_URL
          value: "http://bpf-registry:8084"
        - name: AGENT_NATS_URL
          value: "nats://nats:4222"
        volumeMounts:
        - name: bpf-fs
          mountPath: /sys/fs/bpf
      volumes:
      - name: bpf-fs
        hostPath:
          path: /sys/fs/bpf
```

## Operation

### Polling Loop

The agent continuously polls the BPF Registry for artifacts assigned to its host ID:

1. **Health Check**: Verifies registry connectivity
2. **Artifact Retrieval**: Gets list of artifacts for this host
3. **Processing**: Downloads, verifies, and loads each artifact
4. **Telemetry**: Reports status and metrics

### Program Loading

For each artifact:

1. **Download**: Downloads `artifact.tar.zst` from registry
2. **Verification**: Verifies signature using Vault public key
3. **Extraction**: Extracts BPF object and metadata
4. **Loading**: Uses libbpf to load the BPF program
5. **Attachment**: Attaches program to appropriate kernel hooks
6. **Parameter Application**: Applies runtime parameters to maps
7. **TTL Timer**: Sets timer for automatic unloading

### Telemetry

The agent emits telemetry events to NATS:

- **Program Events**: `loaded`, `unloaded`, `error`
- **Runtime Metrics**: CPU usage, memory, packet counts
- **Agent Heartbeat**: Health status and loaded program count
- **Violation Reports**: Policy violations and errors

### TTL Management

Programs are automatically unloaded when:

- TTL expires (configurable per program)
- Agent receives shutdown signal
- Program limit exceeded (new programs replace old ones)
- Loading errors occur

### Automatic Rollback

The agent monitors telemetry data and automatically unloads programs when thresholds are exceeded:

- **Error Threshold**: Too many errors detected
- **Violation Threshold**: Too many policy violations
- **CPU Threshold**: Excessive CPU usage
- **Latency Threshold**: High latency detected
- **Memory Threshold**: Excessive memory usage
- **Verifier Failure**: BPF verifier failures

Rollback events are emitted to NATS with status `rolled_back` and include:
- Rollback reason and threshold
- Telemetry values that triggered rollback
- Timestamp and metadata

### Orchestrator Integration

The agent subscribes to orchestrator rollback signals:
- Subject: `orchestrator.rollback.{host_id}`
- Responds to manual rollback requests
- Integrates with decision engine rollback policies

## Monitoring

### Logs

The agent logs structured JSON to stdout:

```json
{
  "level": "info",
  "msg": "BPF program loaded successfully",
  "artifact_id": "artifact-123",
  "name": "drop_egress_by_cgroup",
  "version": "1.0.0",
  "ttl": "1h0m0s"
}
```

### Telemetry Events

```json
{
  "type": "program_telemetry",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "artifact_id": "artifact-123",
    "host_id": "host-001",
    "status": "running",
    "cpu_percent": 0.5,
    "memory_kb": 1024,
    "packets_processed": 1000,
    "violations": 0,
    "errors": 0,
    "latency_ms": 0.1
  },
  "metadata": {
    "host_id": "host-001",
    "agent_version": "1.0.0"
  }
}
```

## Troubleshooting

### Common Issues

1. **Permission Denied**: Ensure the agent runs with sufficient privileges for eBPF
2. **Registry Connection**: Check network connectivity and registry URL
3. **NATS Connection**: Verify NATS server is accessible
4. **Signature Verification**: Ensure Vault is accessible and public key is correct
5. **Program Loading**: Check kernel version compatibility and eBPF support

### Debug Mode

Enable debug logging:

```bash
export AGENT_LOG_LEVEL=debug
./local-agent
```

### Health Checks

The agent provides health information through logs and telemetry. Monitor for:

- Successful registry polling
- Program loading success/failure rates
- Telemetry emission frequency
- TTL expiration events

## Security

- **Signature Verification**: All artifacts are cryptographically verified
- **Non-root Execution**: Runs as non-root user in containers
- **Minimal Privileges**: Only requests necessary kernel capabilities
- **Secure Communication**: Uses TLS for registry and NATS connections
- **Token Management**: Vault tokens should be rotated regularly

## Development

### Project Structure

```
agents/local-agent/
├── main.go                 # Entry point
├── internal/
│   ├── agent/             # Main agent logic
│   ├── bpf/               # BPF loading and management
│   ├── config/            # Configuration management
│   ├── registry/          # Registry client
│   ├── telemetry/         # Telemetry reporting
│   └── types/             # Type definitions
├── go.mod                 # Go module definition
├── Makefile              # Build automation
├── Dockerfile            # Container image
└── README.md             # This file
```

### Adding Features

1. **New Program Types**: Extend BPF loader for additional attachment types
2. **Enhanced Telemetry**: Add custom metrics and events
3. **Configuration**: Add new environment variables and validation
4. **Error Handling**: Improve error recovery and reporting
5. **Testing**: Add unit and integration tests

## License

This project is part of AegisFlux and follows the same license terms.