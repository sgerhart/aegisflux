# AegisFlux Agent Observability

This document describes the observability features of the AegisFlux Local Agent, including HTTP endpoints, structured logging, systemd integration, and monitoring capabilities.

## Table of Contents

- [HTTP API Endpoints](#http-api-endpoints)
- [Structured Logging](#structured-logging)
- [Systemd Integration](#systemd-integration)
- [Monitoring and Metrics](#monitoring-and-metrics)
- [Installation and Configuration](#installation-and-configuration)
- [Troubleshooting](#troubleshooting)

## HTTP API Endpoints

The agent exposes several HTTP endpoints for monitoring and management:

### Health Check
- **Endpoint**: `GET /healthz`
- **Purpose**: Kubernetes/Docker health checks
- **Response**: Service health status and basic metrics

```bash
curl http://localhost:8080/healthz
```

Example response:
```json
{
  "status": "healthy",
  "host_id": "host-001",
  "timestamp": "2024-01-15T10:30:00Z",
  "uptime": "2h15m30s",
  "version": "1.0.0",
  "loaded_programs": 3
}
```

### Status Information
- **Endpoint**: `GET /status`
- **Purpose**: Detailed status with applied artifacts and TTL information
- **Response**: Complete agent status including loaded programs

```bash
curl http://localhost:8080/status
```

Example response:
```json
{
  "host_id": "host-001",
  "timestamp": "2024-01-15T10:30:00Z",
  "uptime": "2h15m30s",
  "version": "1.0.0",
  "applied_artifacts": [
    {
      "artifact_id": "artifact-123",
      "name": "drop_egress_by_cgroup",
      "version": "1.0.0",
      "status": "running",
      "loaded_at": "2024-01-15T08:15:00Z",
      "ttl_remaining": "45m30s",
      "ttl_seconds": 2730,
      "telemetry": {
        "cpu_percent": 15.5,
        "memory_kb": 2048,
        "violations": 0,
        "errors": 0
      }
    }
  ],
  "total_programs": 1
}
```

### Metrics
- **Endpoint**: `GET /metrics`
- **Purpose**: Prometheus-style metrics for monitoring
- **Response**: Counter and gauge metrics

```bash
curl http://localhost:8080/metrics
```

Example response:
```json
{
  "host_id": "host-001",
  "timestamp": "2024-01-15T10:30:00Z",
  "counters": {
    "total_programs": 3,
    "running_programs": 2,
    "failed_programs": 0,
    "rolled_back_programs": 1
  },
  "gauges": {
    "uptime_seconds": 8130.5,
    "average_cpu_percent": 12.3,
    "total_memory_kb": 6144,
    "total_violations": 5,
    "total_errors": 0
  }
}
```

### Program Details
- **Endpoint**: `GET /programs`
- **Purpose**: Detailed information about loaded programs
- **Response**: Complete program information including parameters

### Rollback History
- **Endpoint**: `GET /rollbacks`
- **Purpose**: History of rollback operations
- **Response**: List of rollback events

### Telemetry Data
- **Endpoint**: `GET /telemetry`
- **Purpose**: Current telemetry data for all programs
- **Response**: Real-time telemetry metrics

### Version Information
- **Endpoint**: `GET /version`
- **Purpose**: Agent version and build information
- **Response**: Version details and uptime

## Structured Logging

The agent uses structured JSON logging with systemd integration:

### Log Levels
- `debug`: Detailed debugging information
- `info`: General operational information
- `warn`: Warning conditions
- `error`: Error conditions

### Log Categories

#### Program Events
```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "info",
  "msg": "BPF program loaded",
  "host_id": "host-001",
  "service": "aegisflux-agent",
  "component": "agent",
  "event": "program_loaded",
  "artifact_id": "artifact-123"
}
```

#### Telemetry Events
```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "debug",
  "msg": "Telemetry data",
  "host_id": "host-001",
  "service": "aegisflux-agent",
  "component": "agent",
  "artifact_id": "artifact-123",
  "telemetry": {
    "cpu_percent": 15.5,
    "memory_kb": 2048,
    "violations": 0,
    "errors": 0
  }
}
```

#### Rollback Events
```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "warn",
  "msg": "Rollback triggered",
  "host_id": "host-001",
  "service": "aegisflux-agent",
  "component": "agent",
  "artifact_id": "artifact-123",
  "reason": "threshold_exceeded",
  "threshold": "cpu_percent",
  "value": 85.0
}
```

#### System Events
```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "info",
  "msg": "Agent started",
  "host_id": "host-001",
  "service": "aegisflux-agent",
  "component": "agent",
  "event": "agent_started"
}
```

### Log Configuration

Configure logging via environment variables:

```bash
# Log level (debug, info, warn, error)
export AGENT_LOG_LEVEL=info

# Log file location (for non-systemd environments)
export AGENT_CACHE_DIR=/var/log/aegisflux
```

## Systemd Integration

The agent provides full systemd integration for production deployments:

### Service Notifications
- `READY=1`: Service is ready to accept requests
- `STOPPING=1`: Service is shutting down
- `RELOADING=1`: Service is reloading configuration
- `STATUS=<message>`: Custom status messages
- `WATCHDOG=1`: Watchdog ping

### Service File
The systemd service file includes:
- Proper dependency management
- Security restrictions
- Resource limits
- Environment configuration
- Watchdog support

### Installation
```bash
# Install as systemd service
sudo ./scripts/install.sh

# Start the service
sudo systemctl start aegisflux-agent

# Check status
sudo systemctl status aegisflux-agent

# View logs
sudo journalctl -u aegisflux-agent -f
```

### Environment Configuration
```bash
# Edit configuration
sudo nano /etc/aegisflux/aegisflux-agent.env

# Reload configuration
sudo systemctl reload aegisflux-agent
```

## Monitoring and Metrics

### Health Monitoring
The agent provides comprehensive health monitoring:

1. **HTTP Health Checks**: `/healthz` endpoint for load balancers
2. **Program Health**: Individual BPF program status
3. **Resource Usage**: CPU, memory, and performance metrics
4. **Error Tracking**: Error rates and failure patterns
5. **Rollback Monitoring**: Automatic rollback triggers and history

### Metrics Collection
Key metrics include:
- Program count and status
- Resource utilization
- Error rates
- Violation counts
- Rollback events
- Uptime and availability

### Alerting
Configure alerts based on:
- High CPU usage (>80%)
- Memory exhaustion
- High error rates
- Program failures
- Rollback events

## Installation and Configuration

### Environment Variables

#### Core Configuration
```bash
# Host identification
export AGENT_HOST_ID=host-001

# Registry configuration
export AGENT_REGISTRY_URL=http://bpf-registry:8084

# Polling configuration
export AGENT_POLL_INTERVAL_SEC=30

# NATS configuration
export AGENT_NATS_URL=nats://nats:4222

# Vault configuration
export AGENT_VAULT_URL=http://vault:8200
export AGENT_VAULT_TOKEN=dev-token
```

#### HTTP Server Configuration
```bash
# HTTP server settings
export AGENT_HTTP_PORT=8080
export AGENT_HTTP_ADDRESS=0.0.0.0
```

#### Logging Configuration
```bash
# Logging settings
export AGENT_LOG_LEVEL=info
export AGENT_CACHE_DIR=/tmp/aegisflux-agent
```

#### Rollback Configuration
```bash
# Rollback thresholds
export AGENT_ROLLBACK_MAX_ERRORS=10
export AGENT_ROLLBACK_MAX_VIOLATIONS=100
export AGENT_ROLLBACK_MAX_CPU_PERCENT=80.0
export AGENT_ROLLBACK_MAX_LATENCY_MS=1000.0
export AGENT_ROLLBACK_MAX_MEMORY_KB=102400
export AGENT_ROLLBACK_VERIFIER_FAILURE=true
export AGENT_ROLLBACK_CHECK_INTERVAL_SEC=30
export AGENT_ROLLBACK_DELAY_SEC=5
```

### Docker Deployment
```bash
# Build and run with Docker
docker build -t aegisflux-agent .
docker run -d \
  --name aegisflux-agent \
  --privileged \
  -p 8080:8080 \
  -e AGENT_HOST_ID=host-001 \
  -e AGENT_REGISTRY_URL=http://bpf-registry:8084 \
  -e AGENT_NATS_URL=nats://nats:4222 \
  aegisflux-agent
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
        image: aegisflux-agent:latest
        ports:
        - containerPort: 8080
        env:
        - name: AGENT_HOST_ID
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: AGENT_REGISTRY_URL
          value: "http://bpf-registry:8084"
        - name: AGENT_NATS_URL
          value: "nats://nats:4222"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

## Troubleshooting

### Common Issues

#### HTTP Server Not Starting
```bash
# Check if port is already in use
sudo netstat -tlnp | grep 8080

# Check logs for errors
sudo journalctl -u aegisflux-agent -f
```

#### Systemd Service Issues
```bash
# Check service status
sudo systemctl status aegisflux-agent

# Check service configuration
sudo systemd-analyze verify /etc/systemd/system/aegisflux-agent.service

# Test service file
sudo systemd-run --service-type=notify aegisflux-agent
```

#### Logging Issues
```bash
# Check log permissions
ls -la /var/log/aegisflux/

# Check systemd journal
sudo journalctl -u aegisflux-agent --since "1 hour ago"

# Test logging configuration
AGENT_LOG_LEVEL=debug aegisflux-agent
```

### Debug Commands

#### Check Agent Status
```bash
# Health check
curl -s http://localhost:8080/healthz | jq

# Detailed status
curl -s http://localhost:8080/status | jq

# Program information
curl -s http://localhost:8080/programs | jq
```

#### Monitor Logs
```bash
# Follow systemd logs
sudo journalctl -u aegisflux-agent -f

# Filter by log level
sudo journalctl -u aegisflux-agent -p warning -f

# Search for specific events
sudo journalctl -u aegisflux-agent --grep "rollback"
```

#### Performance Monitoring
```bash
# Check metrics
curl -s http://localhost:8080/metrics | jq

# Monitor telemetry
curl -s http://localhost:8080/telemetry | jq

# Check rollback history
curl -s http://localhost:8080/rollbacks | jq
```

### Performance Tuning

#### Resource Limits
```bash
# Adjust memory limits
export AGENT_ROLLBACK_MAX_MEMORY_KB=204800

# Adjust CPU thresholds
export AGENT_ROLLBACK_MAX_CPU_PERCENT=70.0

# Adjust polling interval
export AGENT_POLL_INTERVAL_SEC=60
```

#### HTTP Server Tuning
```bash
# Adjust HTTP server settings
export AGENT_HTTP_ADDRESS=127.0.0.1  # Bind to localhost only
export AGENT_HTTP_PORT=8080
```

### Security Considerations

1. **HTTP API Access**: Consider restricting HTTP API access to localhost or specific networks
2. **Log Security**: Ensure log files have appropriate permissions
3. **Systemd Security**: The service file includes security restrictions
4. **Environment Variables**: Secure sensitive configuration like Vault tokens

### Integration with Monitoring Systems

#### Prometheus
The `/metrics` endpoint provides JSON metrics that can be scraped by Prometheus using a custom exporter.

#### Grafana
Create dashboards using the HTTP API endpoints for visualization.

#### ELK Stack
Structured JSON logs integrate well with Elasticsearch for log analysis.

#### AlertManager
Configure alerts based on HTTP endpoint responses and log patterns.
