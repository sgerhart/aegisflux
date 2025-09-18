# AegisFlux BPF Registry Service

The BPF Registry Service manages eBPF artifacts including programs, maps, and tracepoints. It provides a RESTful API for storing, retrieving, and managing eBPF artifacts with metadata, signing, and host association.

## Features

- **Artifact Storage**: Store eBPF artifacts as compressed tar.zst files
- **Metadata Management**: Rich metadata including version, architecture, kernel version
- **Host Association**: Associate artifacts with specific hosts
- **Vault Signing**: Sign artifacts with Vault (stub implementation)
- **RESTful API**: HTTP API for all operations
- **Health Checks**: Built-in health monitoring

## API Endpoints

### Health Check
```
GET /healthz
```
Returns service health status.

### Create Artifact
```
POST /artifacts
Content-Type: application/json

{
  "name": "example_program",
  "version": "1.0.0",
  "description": "Example eBPF program",
  "type": "program",
  "architecture": "x86_64",
  "kernel_version": "5.4.0",
  "metadata": {
    "author": "example@company.com",
    "license": "GPL-2.0"
  },
  "tags": ["security", "monitoring"],
  "data": "base64_encoded_artifact_data"
}
```

### Get Artifact Metadata
```
GET /artifacts/{id}
```
Returns artifact metadata.

### Get Artifact Binary
```
GET /artifacts/{id}/binary
```
Downloads the artifact binary as a tar.zst file.

### Get Artifacts for Host
```
GET /artifacts/for-host/{host_id}
```
Returns list of artifacts associated with a specific host.

## Configuration

The service can be configured using environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `BPF_REGISTRY_HTTP_ADDR` | `:8084` | HTTP server address |
| `BPF_REGISTRY_DATA_DIR` | `/data/artifacts` | Directory for storing artifacts |
| `BPF_REGISTRY_LOG_LEVEL` | `INFO` | Log level (DEBUG, INFO, WARN, ERROR) |

## Running the Service

### Using Docker

```bash
# Build the image
docker build -t aegisflux-bpf-registry .

# Run the container
docker run -d \
  --name bpf-registry \
  -p 8084:8084 \
  -v /path/to/artifacts:/data/artifacts \
  -e BPF_REGISTRY_DATA_DIR=/data/artifacts \
  aegisflux-bpf-registry
```

### Using Docker Compose

```bash
# Add to your docker-compose.yml
services:
  bpf-registry:
    build: ./backend/bpf-registry
    ports:
      - "8084:8084"
    volumes:
      - ./data/artifacts:/data/artifacts
    environment:
      - BPF_REGISTRY_DATA_DIR=/data/artifacts
```

### Running Locally

```bash
# Install dependencies
go mod tidy

# Run the service
go run ./cmd/bpf-registry
```

## Data Storage

Artifacts are stored in the filesystem with the following structure:

```
/data/artifacts/
├── artifact_1234567890/
│   ├── metadata.json      # Artifact metadata
│   └── artifact.tar.zst   # Compressed artifact binary
├── artifact_1234567891/
│   ├── metadata.json
│   └── artifact.tar.zst
└── ...
```

## Security

- **Vault Integration**: Artifacts are signed with Vault keys (stub implementation)
- **Checksum Validation**: SHA256 checksums for integrity verification
- **Access Control**: Future versions will include authentication and authorization

## Development

### Project Structure

```
backend/bpf-registry/
├── cmd/bpf-registry/       # Main application
├── internal/
│   ├── api/               # HTTP API handlers
│   ├── model/             # Data models
│   └── store/             # Storage implementation
├── Dockerfile             # Container definition
└── README.md              # This file
```

### Building

```bash
# Build the binary
go build -o bpf-registry ./cmd/bpf-registry

# Run tests
go test ./...
```

## Example Usage

### Creating an Artifact

```bash
curl -X POST http://localhost:8084/artifacts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "security_monitor",
    "version": "1.0.0",
    "description": "Security monitoring eBPF program",
    "type": "program",
    "architecture": "x86_64",
    "kernel_version": "5.4.0",
    "data": "dGVzdCBkYXRh"  # Base64 encoded data
  }'
```

### Retrieving Artifact Metadata

```bash
curl http://localhost:8084/artifacts/artifact_1234567890
```

### Downloading Artifact Binary

```bash
curl -O http://localhost:8084/artifacts/artifact_1234567890/binary
```

## Future Enhancements

- [ ] Authentication and authorization
- [ ] Real Vault integration for signing
- [ ] Artifact versioning and rollback
- [ ] Metrics and monitoring
- [ ] Multi-part upload for large artifacts
- [ ] Artifact search and filtering
- [ ] Host deployment tracking