#!/bin/bash
set -euo pipefail

# Vault Populate Script
# Populates Vault secrets from a structured JSON file

VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-root}"
BACKUP_FILE="${BACKUP_FILE:-}"
DRY_RUN="${DRY_RUN:-false}"
FORCE="${FORCE:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Vault is accessible
check_vault_health() {
    log_info "Checking Vault health..."
    
    if ! curl -s "${VAULT_ADDR}/v1/sys/health" > /dev/null; then
        log_error "Cannot connect to Vault at ${VAULT_ADDR}"
        exit 1
    fi
    
    log_success "Vault is accessible"
}

# Validate backup file
validate_backup_file() {
    if [ -z "${BACKUP_FILE}" ]; then
        log_error "BACKUP_FILE environment variable is required"
        exit 1
    fi
    
    if [ ! -f "${BACKUP_FILE}" ]; then
        log_error "Backup file not found: ${BACKUP_FILE}"
        exit 1
    fi
    
    if ! jq empty "${BACKUP_FILE}" 2>/dev/null; then
        log_error "Invalid JSON file: ${BACKUP_FILE}"
        exit 1
    fi
    
    log_success "Backup file validated: ${BACKUP_FILE}"
}

# Check if secret already exists
secret_exists() {
    local path="$1"
    
    if vault kv get "${path}" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Populate a single secret
populate_secret() {
    local secret_data="$1"
    local path
    local data
    
    path=$(echo "${secret_data}" | jq -r '.path')
    data=$(echo "${secret_data}" | jq -r '.data.data')
    
    if [ "${path}" == "null" ] || [ "${data}" == "null" ]; then
        log_warning "Skipping invalid secret data"
        return 0
    fi
    
    # Check if secret already exists
    if secret_exists "${path}"; then
        if [ "${FORCE}" == "true" ]; then
            log_warning "Secret already exists, overwriting: ${path}"
        else
            log_warning "Secret already exists, skipping: ${path} (use --force to overwrite)"
            return 0
        fi
    fi
    
    if [ "${DRY_RUN}" == "true" ]; then
        log_info "DRY RUN: Would populate secret: ${path}"
        echo "${data}" | jq .
        return 0
    fi
    
    # Convert data to key-value pairs for vault kv put
    local kv_args=()
    while IFS= read -r line; do
        local key
        local value
        key=$(echo "${line}" | jq -r '.key')
        value=$(echo "${line}" | jq -r '.value')
        
        if [ "${key}" != "null" ] && [ "${value}" != "null" ]; then
            kv_args+=("${key}=${value}")
        fi
    done < <(echo "${data}" | jq -r 'to_entries[] | {key: .key, value: .value}')
    
    # Write secret to Vault
    if vault kv put "${path}" "${kv_args[@]}" > /dev/null 2>&1; then
        log_success "Populated secret: ${path}"
    else
        log_error "Failed to populate secret: ${path}"
        return 1
    fi
}

# Main populate function
populate_vault() {
    local secret_count
    local success_count=0
    local skip_count=0
    local error_count=0
    
    secret_count=$(jq length "${BACKUP_FILE}")
    
    log_info "Starting Vault population..."
    log_info "Vault Address: ${VAULT_ADDR}"
    log_info "Backup File: ${BACKUP_FILE}"
    log_info "Dry Run: ${DRY_RUN}"
    log_info "Force: ${FORCE}"
    log_info "Total secrets to process: ${secret_count}"
    
    # Process each secret
    while IFS= read -r secret_data; do
        if populate_secret "${secret_data}"; then
            ((success_count++))
        else
            ((error_count++))
        fi
    done < <(jq -c '.[]' "${BACKUP_FILE}")
    
    # Calculate skip count
    skip_count=$((secret_count - success_count - error_count))
    
    # Summary
    log_info "Population completed!"
    log_info "  Success: ${success_count}"
    log_info "  Skipped: ${skip_count}"
    log_info "  Errors: ${error_count}"
    
    if [ "${error_count}" -gt 0 ]; then
        log_warning "Some secrets failed to populate. Check the logs above."
        exit 1
    fi
}

# Create a template file for manual population
create_template() {
    local template_file="${1:-vault-secrets-template.json}"
    
    log_info "Creating template file: ${template_file}"
    
    cat > "${template_file}" << 'EOF'
[
  {
    "path": "secret/llm/openai",
    "data": {
      "data": {
        "api_key": "your-openai-api-key-here",
        "model": "gpt-4",
        "max_tokens": 4000
      }
    }
  },
  {
    "path": "secret/llm/anthropic",
    "data": {
      "data": {
        "api_key": "your-anthropic-api-key-here",
        "model": "claude-3-sonnet",
        "max_tokens": 4000
      }
    }
  },
  {
    "path": "secret/cve-sync/nvd",
    "data": {
      "data": {
        "api_key": "your-nvd-api-key-here",
        "rate_limit": 50,
        "timeout": 30
      }
    }
  },
  {
    "path": "secret/cve-sync/config",
    "data": {
      "data": {
        "max_pages": 10,
        "retry_attempts": 3,
        "cache_ttl": 300
      }
    }
  },
  {
    "path": "secret/nats",
    "data": {
      "data": {
        "creds": "nats://nats:4222"
      }
    }
  },
  {
    "path": "secret/neo4j",
    "data": {
      "data": {
        "user": "neo4j",
        "pass": "password"
      }
    }
  },
  {
    "path": "secret/registry",
    "data": {
      "data": {
        "signing_key": "dev-not-real"
      }
    }
  }
]
EOF
    
    log_success "Template file created: ${template_file}"
    log_info "Edit the template file with your actual secrets, then run:"
    log_info "  BACKUP_FILE=${template_file} $0"
}

# Main execution
main() {
    log_info "Starting Vault populate process..."
    
    # Check prerequisites
    if ! command -v vault &> /dev/null; then
        log_error "Vault CLI not found. Please install Vault CLI."
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        log_error "jq not found. Please install jq."
        exit 1
    fi
    
    # Set Vault environment
    export VAULT_ADDR="${VAULT_ADDR}"
    export VAULT_TOKEN="${VAULT_TOKEN}"
    
    # Authenticate with Vault
    if ! vault auth -no-print "${VAULT_TOKEN}" > /dev/null 2>&1; then
        log_error "Failed to authenticate with Vault"
        exit 1
    fi
    
    # Perform population
    check_vault_health
    validate_backup_file
    populate_vault
    
    log_success "Vault populate process completed successfully!"
}

# Show usage if help requested
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    cat << EOF
Vault Populate Script

Usage: $0 [options]

Options:
  --help, -h     Show this help message
  --template     Create a template file for manual population
  --dry-run      Show what would be populated without actually doing it
  --force        Overwrite existing secrets

Environment Variables:
  VAULT_ADDR     Vault server address (default: http://localhost:8200)
  VAULT_TOKEN    Vault authentication token (default: root)
  BACKUP_FILE    JSON file containing secrets to populate
  DRY_RUN        Set to 'true' for dry run mode
  FORCE          Set to 'true' to overwrite existing secrets

Examples:
  # Populate from backup file
  BACKUP_FILE=./vault-backups/vault-backup-20231201_120000.json $0

  # Dry run to see what would be populated
  BACKUP_FILE=secrets.json DRY_RUN=true $0

  # Force overwrite existing secrets
  BACKUP_FILE=secrets.json FORCE=true $0

  # Create template file
  $0 --template

EOF
    exit 0
fi

# Handle template creation
if [[ "${1:-}" == "--template" ]]; then
    create_template "${2:-vault-secrets-template.json}"
    exit 0
fi

# Handle dry run
if [[ "${1:-}" == "--dry-run" ]]; then
    DRY_RUN="true"
    shift
fi

# Handle force
if [[ "${1:-}" == "--force" ]]; then
    FORCE="true"
    shift
fi

# Run main function
main "$@"
