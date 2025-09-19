# AegisFlux Actions API - Registration Backend

The Actions API provides secure agent registration and management capabilities for the AegisFlux system.

## Overview

The Actions API supports:
- **Cryptographic Agent Registration**: Ed25519-based two-phase registration
- **Rich Metadata Collection**: Platform, network, and capability information
- **Agent Management**: List, get, label, and annotate agents
- **In-Memory Storage**: Fast, development-friendly storage (can be extended to persistent storage)

## API Endpoints

### Health Check
```
GET /healthz
```
Returns: `ok`

### Agent Registration

#### Phase 1: Registration Init
```
POST /agents/register/init
```

**Request Body:**
```json
{
  "org_id": "security-team",
  "host_id": "web-server-01",
  "agent_pubkey": "base64-encoded-ed25519-public-key",
  "machine_id_hash": "sha256-hash-of-machine-id",
  "agent_version": "1.0.0",
  "capabilities": {
    "ebpf_loading": true,
    "ebpf_attach": true,
    "map_operations": true,
    "kernel_modules": ["bpf", "netfilter"],
    "supported_hooks": ["xdp", "tc", "tracepoint", "kprobe"],
    "max_programs": 10,
    "max_maps": 50
  },
  "platform": {
    "hostname": "web-server-01",
    "fqdn": "web-server-01.example.com",
    "os": "Ubuntu 22.04 LTS",
    "kernel_version": "5.15.0-91-generic",
    "architecture": "x86_64",
    "cpu_model": "Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz",
    "memory_gb": 16,
    "disk_gb": 512
  },
  "network": {
    "primary_ip": "192.168.1.100",
    "mac_address": "00:1b:44:11:3a:b7",
    "subnet": "192.168.1.0/24",
    "gateway": "192.168.1.1",
    "dns_servers": ["8.8.8.8", "8.8.4.4"],
    "ifaces": {
      "eth0": {
        "addrs": ["192.168.1.100/24"],
        "mac": "00:1b:44:11:3a:b7"
      }
    }
  }
}
```

**Response:**
```json
{
  "registration_id": "12345678-1234-1234-1234-123456789abc",
  "nonce": "base64-encoded-32-byte-random-nonce",
  "server_time": "2025-09-19T12:20:33Z"
}
```

#### Phase 2: Registration Complete
```
POST /agents/register/complete
```

**Request Body:**
```json
{
  "registration_id": "12345678-1234-1234-1234-123456789abc",
  "host_id": "web-server-01",
  "signature": "base64-ed25519-signature-over-(nonce||server_time||host_id)"
}
```

**Response:**
```json
{
  "agent_uid": "87654321-4321-4321-4321-cba987654321",
  "bootstrap_token": "dev-abc123def456"
}
```

### Agent Management

#### List Agents
```
GET /agents
```

**Query Parameters:**
- `label`: Filter by label (exact match)
- `hostname`: Filter by hostname (exact match)
- `host_id`: Filter by host ID (exact match)
- `ip`: Search for IP in network configuration

**Examples:**
```bash
# List all agents
curl -s http://localhost:8083/agents | jq .

# Filter by label
curl -s "http://localhost:8083/agents?label=production" | jq .

# Filter by hostname
curl -s "http://localhost:8083/agents?hostname=web-server-01" | jq .

# Filter by IP
curl -s "http://localhost:8083/agents?ip=192.168.1.100" | jq .
```

**Response:**
```json
{
  "agents": [
    {
      "agent_uid": "87654321-4321-4321-4321-cba987654321",
      "org_id": "security-team",
      "host_id": "web-server-01",
      "hostname": "web-server-01",
      "machine_id_hash": "sha256-hash-of-machine-id",
      "agent_version": "1.0.0",
      "platform": {
        "hostname": "web-server-01",
        "fqdn": "web-server-01.example.com",
        "os": "Ubuntu 22.04 LTS",
        "kernel_version": "5.15.0-91-generic",
        "architecture": "x86_64"
      },
      "network": {
        "primary_ip": "192.168.1.100",
        "mac_address": "00:1b:44:11:3a:b7"
      },
      "labels": ["production", "web-tier"],
      "note": "Production web server for e-commerce platform",
      "created": "2025-09-19T12:20:33Z",
      "last_seen": "2025-09-19T12:25:15Z"
    }
  ],
  "total": 1
}
```

#### Get Agent Details
```
GET /agents/{agent_uid}
```

**Example:**
```bash
curl -s http://localhost:8083/agents/87654321-4321-4321-4321-cba987654321 | jq .
```

**Response:**
```json
{
  "agent_uid": "87654321-4321-4321-4321-cba987654321",
  "org_id": "security-team",
  "host_id": "web-server-01",
  "hostname": "web-server-01",
  "machine_id_hash": "sha256-hash-of-machine-id",
  "agent_version": "1.0.0",
  "capabilities": {
    "ebpf_loading": true,
    "ebpf_attach": true,
    "map_operations": true,
    "kernel_modules": ["bpf", "netfilter"],
    "supported_hooks": ["xdp", "tc", "tracepoint", "kprobe"],
    "max_programs": 10,
    "max_maps": 50
  },
  "platform": {
    "hostname": "web-server-01",
    "fqdn": "web-server-01.example.com",
    "os": "Ubuntu 22.04 LTS",
    "kernel_version": "5.15.0-91-generic",
    "architecture": "x86_64",
    "cpu_model": "Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz",
    "memory_gb": 16,
    "disk_gb": 512
  },
  "network": {
    "primary_ip": "192.168.1.100",
    "mac_address": "00:1b:44:11:3a:b7",
    "subnet": "192.168.1.0/24",
    "gateway": "192.168.1.1",
    "dns_servers": ["8.8.8.8", "8.8.4.4"],
    "ifaces": {
      "eth0": {
        "addrs": ["192.168.1.100/24"],
        "mac": "00:1b:44:11:3a:b7"
      }
    }
  },
  "labels": ["production", "web-tier"],
  "note": "Production web server for e-commerce platform",
  "created": "2025-09-19T12:20:33Z",
  "last_seen": "2025-09-19T12:25:15Z"
}
```

#### Update Agent Labels
```
PUT /agents/{agent_uid}/labels
```

**Request Body:**
```json
{
  "add": ["critical", "ssl-enabled"],
  "remove": ["staging"]
}
```

**Example:**
```bash
curl -s -X PUT http://localhost:8083/agents/87654321-4321-4321-4321-cba987654321/labels \
  -H 'Content-Type: application/json' \
  -d '{"add": ["critical", "ssl-enabled"], "remove": ["staging"]}' | jq .
```

**Response:**
```json
{
  "agent_uid": "87654321-4321-4321-4321-cba987654321",
  "labels": ["production", "web-tier", "critical", "ssl-enabled"]
}
```

#### Update Agent Note
```
PUT /agents/{agent_uid}/note
```

**Request Body:**
```json
{
  "note": "Updated: Added SSL certificate and security patches"
}
```

**Example:**
```bash
curl -s -X PUT http://localhost:8083/agents/87654321-4321-4321-4321-cba987654321/note \
  -H 'Content-Type: application/json' \
  -d '{"note": "Updated: Added SSL certificate and security patches"}' | jq .
```

**Response:**
```json
{
  "agent_uid": "87654321-4321-4321-4321-cba987654321",
  "note": "Updated: Added SSL certificate and security patches"
}
```

## Complete Registration Example

Here's a complete example of registering an agent with rich metadata:

### 1. Generate Ed25519 Key Pair (Agent Side)
```bash
# Generate private key
openssl genpkey -algorithm Ed25519 -out agent_private.pem

# Extract public key
openssl pkey -in agent_private.pem -pubout -outform DER | base64
```

### 2. Register Agent (Phase 1)
```bash
curl -s -X POST http://localhost:8083/agents/register/init \
  -H 'Content-Type: application/json' \
  -d '{
    "org_id": "security-team",
    "host_id": "web-server-01",
    "agent_pubkey": "MCowBQYDK2VwAyEAGb9ECWmEzf6FQbrBZ9w7lshQhqowtrbLDFw4rXAxZuE=",
    "machine_id_hash": "sha256-hash-of-machine-id",
    "agent_version": "1.0.0",
    "capabilities": {
      "ebpf_loading": true,
      "ebpf_attach": true,
      "kernel_modules": ["bpf", "netfilter"],
      "supported_hooks": ["xdp", "tc"],
      "max_programs": 10
    },
    "platform": {
      "hostname": "web-server-01",
      "fqdn": "web-server-01.example.com",
      "os": "Ubuntu 22.04 LTS",
      "kernel_version": "5.15.0-91-generic",
      "architecture": "x86_64"
    },
    "network": {
      "primary_ip": "192.168.1.100",
      "mac_address": "00:1b:44:11:3a:b7",
      "ifaces": {
        "eth0": {
          "addrs": ["192.168.1.100/24"],
          "mac": "00:1b:44:11:3a:b7"
        }
      }
    }
  }' | jq .
```

### 3. Complete Registration (Phase 2)
```bash
# Sign the challenge (nonce + server_time + host_id)
# This would be done programmatically in the agent
curl -s -X POST http://localhost:8083/agents/register/complete \
  -H 'Content-Type: application/json' \
  -d '{
    "registration_id": "12345678-1234-1234-1234-123456789abc",
    "host_id": "web-server-01",
    "signature": "base64-ed25519-signature"
  }' | jq .
```

### 4. Add Labels and Notes
```bash
# Add labels
curl -s -X PUT http://localhost:8083/agents/87654321-4321-4321-4321-cba987654321/labels \
  -H 'Content-Type: application/json' \
  -d '{"add": ["production", "web-tier", "critical"]}' | jq .

# Add note
curl -s -X PUT http://localhost:8083/agents/87654321-4321-4321-4321-cba987654321/note \
  -H 'Content-Type: application/json' \
  -d '{"note": "Production web server for e-commerce platform"}' | jq .
```

### 5. Query Agents
```bash
# List all production agents
curl -s "http://localhost:8083/agents?label=production" | jq .

# Find agent by IP
curl -s "http://localhost:8083/agents?ip=192.168.1.100" | jq .
```

## Error Responses

### 400 Bad Request
```json
{
  "error": "Invalid JSON"
}
```

### 401 Unauthorized
```json
{
  "error": "signature verify failed"
}
```

### 404 Not Found
```json
{
  "error": "Agent not found"
}
```

### 405 Method Not Allowed
```json
{
  "error": "Method not allowed"
}
```

## Security Features

- **Ed25519 Digital Signatures**: Cryptographic proof of agent identity
- **Nonce-based Challenge/Response**: Prevents replay attacks
- **Bootstrap Tokens**: Secure tokens for future operations
- **In-Memory Storage**: Fast access with automatic cleanup on restart

## Storage

Currently uses in-memory storage for development. For production, this can be extended to:
- PostgreSQL for persistent storage
- Redis for caching and session management
- Vault for secure key storage

## Development

### Building
```bash
cd backend/actions-api
go build -o actions-api ./cmd/actions-api
```

### Running
```bash
./actions-api
```

### Environment Variables
- `ACTIONS_ADDR`: Listen address (default: `:8083`)

## Future Enhancements

- [ ] Persistent storage backend
- [ ] Agent heartbeat and health monitoring
- [ ] Role-based access control
- [ ] Audit logging
- [ ] Rate limiting
- [ ] Metrics and monitoring
- [ ] Agent configuration management
- [ ] Bulk operations for labels and notes

