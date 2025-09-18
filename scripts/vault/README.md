# Vault Management Scripts

This directory contains scripts for managing HashiCorp Vault secrets in the AegisFlux project.

## Scripts Overview

### 1. `backup.sh` - Vault Backup Script
Backs up Vault secrets to a structured JSON file for disaster recovery and migration.

### 2. `populate.sh` - Vault Populate Script
Populates Vault secrets from a structured JSON file (created by backup or manually).

### 3. `dev-bootstrap.sh` - Development Bootstrap Script
Initializes Vault with development secrets and policies.

### 4. `vault-secrets-template.json` - Secrets Template
Template file containing the structure for all AegisFlux secrets.

## Quick Start

### 1. Backup Vault Secrets
```bash
# Basic backup
./backup.sh

# Backup to specific directory
BACKUP_DIR=/tmp/vault-backup ./backup.sh

# Backup from different Vault instance
VAULT_ADDR=https://vault.company.com VAULT_TOKEN=your-token ./backup.sh
```

### 2. Populate Vault Secrets
```bash
# Populate from backup file
BACKUP_FILE=./vault-backups/vault-backup-20231201_120000.json ./populate.sh

# Dry run to see what would be populated
BACKUP_FILE=secrets.json DRY_RUN=true ./populate.sh

# Force overwrite existing secrets
BACKUP_FILE=secrets.json FORCE=true ./populate.sh

# Create template file
./populate.sh --template
```

### 3. Development Setup
```bash
# Bootstrap Vault with development secrets
./dev-bootstrap.sh

# Or use the Makefile
make vault-bootstrap
```

## Secret Structure

The scripts manage the following secrets in Vault:

### LLM API Keys
- **Path**: `secret/llm/openai`
  - `api_key`: OpenAI API key
  - `model`: Model name (e.g., "gpt-4")
  - `max_tokens`: Maximum tokens per request

- **Path**: `secret/llm/anthropic`
  - `api_key`: Anthropic API key
  - `model`: Model name (e.g., "claude-3-sonnet")
  - `max_tokens`: Maximum tokens per request

### CVE Sync Configuration
- **Path**: `secret/cve-sync/nvd`
  - `api_key`: NVD API key for higher rate limits
  - `rate_limit`: Requests per minute (default: 50)
  - `timeout`: Request timeout in seconds (default: 30)

- **Path**: `secret/cve-sync/config`
  - `max_pages`: Maximum pages to fetch (default: 10)
  - `retry_attempts`: Number of retry attempts (default: 3)
  - `cache_ttl`: Cache TTL in seconds (default: 300)

### Infrastructure Secrets
- **Path**: `secret/nats`
  - `creds`: NATS connection string

- **Path**: `secret/neo4j`
  - `user`: Neo4j username
  - `pass`: Neo4j password

- **Path**: `secret/registry`
  - `signing_key`: BPF registry signing key

## Environment Variables

### Common Variables
- `VAULT_ADDR`: Vault server address (default: `http://localhost:8200`)
- `VAULT_TOKEN`: Vault authentication token (default: `root`)

### Backup Script Variables
- `BACKUP_DIR`: Directory to store backup files (default: `./vault-backups`)

### Populate Script Variables
- `BACKUP_FILE`: JSON file containing secrets to populate
- `DRY_RUN`: Set to `true` for dry run mode
- `FORCE`: Set to `true` to overwrite existing secrets

## Usage Examples

### Development Workflow
```bash
# 1. Start Vault
make vault-up

# 2. Bootstrap with development secrets
make vault-bootstrap

# 3. Backup the setup
./backup.sh

# 4. Test the backup by restoring to a new Vault
# (In a new terminal with different VAULT_ADDR)
BACKUP_FILE=./vault-backups/vault-backup-*.json ./populate.sh
```

### Production Migration
```bash
# 1. Backup production Vault
VAULT_ADDR=https://prod-vault.company.com VAULT_TOKEN=prod-token ./backup.sh

# 2. Populate staging Vault
VAULT_ADDR=https://staging-vault.company.com VAULT_TOKEN=staging-token \
BACKUP_FILE=./vault-backups/vault-backup-*.json ./populate.sh

# 3. Verify secrets
vault kv list secret/
```

### Manual Secret Management
```bash
# 1. Create template file
./populate.sh --template

# 2. Edit template with real secrets
vim vault-secrets-template.json

# 3. Populate Vault
BACKUP_FILE=vault-secrets-template.json ./populate.sh
```

## Security Considerations

### Development vs Production

**Development:**
- Uses `-dev` mode with auto-unsealing
- Root token: `root`
- No TLS (HTTP only)
- Data stored in memory/volume

**Production:**
- Use proper unsealing (multiple keys)
- Enable TLS
- Use AppRole or JWT authentication
- Regular secret rotation
- Audit logging enabled

### Best Practices

1. **Never commit real secrets to version control**
2. **Use least privilege policies**
3. **Rotate secrets regularly**
4. **Monitor access logs**
5. **Use environment-specific Vault instances**
6. **Encrypt backup files**
7. **Store backup files securely**

## Troubleshooting

### Common Issues

**Vault Connection Failed:**
```bash
# Check Vault health
curl http://localhost:8200/v1/sys/health

# Check Vault status
vault status
```

**Authentication Failed:**
```bash
# Check token validity
vault token lookup

# Check token capabilities
vault token capabilities secret/data/llm/openai
```

**Secret Not Found:**
```bash
# List available secrets
vault kv list secret/

# Check specific secret
vault kv get secret/llm/openai
```

**Permission Denied:**
```bash
# Check policy attached to token
vault token lookup -format=json | jq .data.policies

# Verify policy allows the operation
vault policy read cve-sync
```

### Debug Mode

Enable debug logging:
```bash
export VAULT_LOG_LEVEL=debug
./backup.sh
```

## File Formats

### Backup File Format
```json
[
  {
    "path": "secret/llm/openai",
    "data": {
      "data": {
        "api_key": "sk-...",
        "model": "gpt-4",
        "max_tokens": 4000
      }
    }
  }
]
```

### Template File Format
Same as backup file format, but with placeholder values for manual editing.

## Integration with Services

Each service in AegisFlux is configured to:
1. Connect to Vault using service token
2. Cache secrets locally with TTL
3. Handle secret rotation gracefully
4. Log secret access (without values)

### Service Configuration
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
