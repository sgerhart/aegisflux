# Vault Development Runbook

This document provides step-by-step instructions for working with HashiCorp Vault in the AegisFlux development environment.

## Prerequisites

- Docker and Docker Compose installed
- Vault CLI installed (`vault` command available)
- `jq` installed for JSON parsing

## Starting Vault

### Using Makefile (Recommended)
```bash
# Start Vault service
make vault-up

# Start Vault and bootstrap it with initial secrets
make vault-bootstrap
```

### Manual Docker Compose
```bash
# Start Vault service
docker compose -f infra/compose/docker-compose.yml up -d vault

# Wait for Vault to be ready
sleep 5
```

## Initial Setup and Bootstrap

### 1. Bootstrap Vault with Initial Configuration
```bash
# Run the bootstrap script to set up secrets and policies
./scripts/vault/dev-bootstrap.sh
```

This script will:
- Enable KV v2 secrets engine at `secret/`
- Create example secrets for LLM, NATS, Neo4j, and registry
- Create service-specific policies
- Generate tokens for services

### 2. Verify Setup
```bash
# Check Vault status
vault status

# List available secrets
vault kv list secret/

# View a specific secret
vault kv get secret/llm/openai
```

## Working with Secrets

### Adding New Secrets

#### Using Vault CLI
```bash
# Set Vault address and token
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=root

# Add a new secret
vault kv put secret/your-service/config \
  api_key="your-api-key" \
  database_url="your-db-url" \
  timeout=30

# Add nested secrets
vault kv put secret/llm/anthropic \
  api_key="your-anthropic-key" \
  model="claude-3"
```

#### Using Vault UI
1. Open http://localhost:8200 in your browser
2. Login with token: `root`
3. Navigate to "Secrets" → "secret/"
4. Click "Create secret" to add new secrets

### Reading Secrets

#### From Command Line
```bash
# Read a secret
vault kv get secret/llm/openai

# Read specific field
vault kv get -field=api_key secret/llm/openai

# Read as JSON
vault kv get -format=json secret/llm/openai
```

#### From Application Code
```go
// Example Go code for reading secrets
import "github.com/hashicorp/vault/api"

client, err := api.NewClient(&api.Config{
    Address: "http://vault:8200",
})
if err != nil {
    log.Fatal(err)
}

client.SetToken(os.Getenv("VAULT_TOKEN"))

secret, err := client.KVv2("secret").Get(context.Background(), "llm/openai")
if err != nil {
    log.Fatal(err)
}

apiKey := secret.Data["api_key"].(string)
```

## Policy Management

### Creating Policies

#### 1. Create Policy File
```hcl
# decision-policy.hcl
path "secret/data/llm/*" {
  capabilities = ["read"]
}

path "secret/data/registry/*" {
  capabilities = ["read"]
}
```

#### 2. Apply Policy
```bash
vault policy write decision decision-policy.hcl
```

### Managing Tokens

#### Create Service Token
```bash
# Create token with specific policy
vault token create -policy=decision -ttl=24h

# Create token with multiple policies
vault token create -policy=decision -policy=correlator -ttl=24h
```

#### Revoke Token
```bash
# Revoke specific token
vault token revoke <token-id>

# Revoke all tokens for a policy
vault token revoke -policy=decision
```

## Secret Rotation

### Manual Rotation

#### 1. Update Secret
```bash
# Rotate API key
vault kv put secret/llm/openai \
  api_key="new-api-key-$(date +%s)" \
  rotated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
```

#### 2. Notify Services
```bash
# Services should periodically refresh their tokens
# or use Vault agent for automatic renewal
```

### Automated Rotation (Production)

For production environments, consider implementing:
- Vault Agent for automatic token renewal
- Dynamic secrets for databases
- Automated secret rotation policies

## Troubleshooting

### Common Issues

#### Vault Not Starting
```bash
# Check container logs
docker logs aegisflux-vault

# Check if port is already in use
lsof -i :8200
```

#### Authentication Issues
```bash
# Verify token is valid
vault token lookup

# Check token capabilities
vault token capabilities secret/data/llm/openai
```

#### Permission Denied
```bash
# Check policy attached to token
vault token lookup -format=json | jq .data.policies

# Verify policy allows the operation
vault policy read decision
```

### Debugging

#### Enable Debug Logging
```bash
# Set debug environment variable
export VAULT_LOG_LEVEL=debug

# Or in docker-compose.yml
environment:
  - VAULT_LOG_LEVEL=debug
```

#### Health Check
```bash
# Check Vault health
curl http://localhost:8200/v1/sys/health

# Check seal status
vault status
```

## Security Considerations

### Development vs Production

#### Development (Current Setup)
- Uses `-dev` mode with auto-unsealing
- Root token: `root`
- No TLS (HTTP only)
- Data stored in memory/volume

#### Production Recommendations
- Use proper unsealing (multiple keys)
- Enable TLS
- Use AppRole or JWT authentication
- Regular secret rotation
- Audit logging enabled

### Best Practices

1. **Never commit secrets to version control**
2. **Use least privilege policies**
3. **Rotate secrets regularly**
4. **Monitor access logs**
5. **Use environment-specific Vault instances**

## Integration with Services

### Service Configuration

Each service should be configured to:
1. Connect to Vault using service token
2. Cache secrets locally with TTL
3. Handle secret rotation gracefully
4. Log secret access (without values)

### Example Environment Variables
```bash
# For services
VAULT_ADDR=http://vault:8200
VAULT_TOKEN=<service-token>
VAULT_CACHE_TTL=300s
```

## Cleanup

### Stop Vault
```bash
# Stop Vault service
docker compose -f infra/compose/docker-compose.yml down vault

# Remove volumes (WARNING: deletes all data)
docker volume rm compose_vault-data
```

### Reset Vault
```bash
# Stop and remove Vault
docker compose -f infra/compose/docker-compose.yml down vault
docker volume rm compose_vault-data

# Start fresh
make vault-bootstrap
```
