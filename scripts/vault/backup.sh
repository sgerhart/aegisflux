#!/bin/bash
set -euo pipefail

# Vault Backup Script
# Backs up Vault secrets to a structured JSON file

VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-root}"
BACKUP_DIR="${BACKUP_DIR:-./vault-backups}"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="${BACKUP_DIR}/vault-backup-${TIMESTAMP}.json"

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

# Create backup directory
create_backup_dir() {
    log_info "Creating backup directory: ${BACKUP_DIR}"
    mkdir -p "${BACKUP_DIR}"
}

# Backup a specific secret path
backup_secret_path() {
    local path="$1"
    local output_file="$2"
    
    log_info "Backing up secret path: ${path}"
    
    # List all secrets in the path
    local secrets
    if ! secrets=$(vault kv list -format=json "${path}" 2>/dev/null); then
        log_warning "No secrets found at path: ${path}"
        return 0
    fi
    
    # Parse the list and backup each secret
    echo "${secrets}" | jq -r '.[]' | while read -r secret_name; do
        if [ -n "${secret_name}" ]; then
            local full_path="${path}/${secret_name}"
            log_info "  Backing up: ${full_path}"
            
            # Get the secret data
            local secret_data
            if secret_data=$(vault kv get -format=json "${full_path}" 2>/dev/null); then
                # Add to backup file
                echo "${secret_data}" | jq --arg path "${full_path}" '. + {path: $path}' >> "${output_file}"
            else
                log_warning "    Failed to backup: ${full_path}"
            fi
        fi
    done
}

# Main backup function
backup_vault() {
    log_info "Starting Vault backup..."
    log_info "Vault Address: ${VAULT_ADDR}"
    log_info "Backup File: ${BACKUP_FILE}"
    
    # Initialize backup file
    echo "[]" > "${BACKUP_FILE}"
    
    # Define secret paths to backup
    local secret_paths=(
        "secret/llm"
        "secret/cve-sync"
        "secret/nats"
        "secret/neo4j"
        "secret/registry"
    )
    
    # Backup each path
    for path in "${secret_paths[@]}"; do
        backup_secret_path "${path}" "${BACKUP_FILE}"
    done
    
    # Count backed up secrets
    local secret_count
    secret_count=$(jq length "${BACKUP_FILE}")
    
    log_success "Backup completed successfully!"
    log_success "Backed up ${secret_count} secrets to ${BACKUP_FILE}"
    
    # Show backup file size
    local file_size
    file_size=$(du -h "${BACKUP_FILE}" | cut -f1)
    log_info "Backup file size: ${file_size}"
}

# Create a summary of the backup
create_backup_summary() {
    local summary_file="${BACKUP_DIR}/vault-backup-${TIMESTAMP}-summary.txt"
    
    log_info "Creating backup summary: ${summary_file}"
    
    cat > "${summary_file}" << EOF
Vault Backup Summary
===================
Timestamp: ${TIMESTAMP}
Vault Address: ${VAULT_ADDR}
Backup File: ${BACKUP_FILE}
File Size: $(du -h "${BACKUP_FILE}" | cut -f1)

Secret Counts by Path:
$(jq -r 'group_by(.path) | .[] | "\(.[0].path | split("/")[1]): \(length) secrets"' "${BACKUP_FILE}")

Secret List:
$(jq -r '.[] | "\(.path): \(.data.data | keys | join(", "))"' "${BACKUP_FILE}")

EOF
    
    log_success "Backup summary created: ${summary_file}"
}

# Main execution
main() {
    log_info "Starting Vault backup process..."
    
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
    
    # Perform backup
    check_vault_health
    create_backup_dir
    backup_vault
    create_backup_summary
    
    log_success "Vault backup process completed successfully!"
    log_info "Backup files:"
    log_info "  - ${BACKUP_FILE}"
    log_info "  - ${BACKUP_DIR}/vault-backup-${TIMESTAMP}-summary.txt"
}

# Show usage if help requested
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    cat << EOF
Vault Backup Script

Usage: $0 [options]

Options:
  --help, -h     Show this help message

Environment Variables:
  VAULT_ADDR     Vault server address (default: http://localhost:8200)
  VAULT_TOKEN    Vault authentication token (default: root)
  BACKUP_DIR     Backup directory (default: ./vault-backups)

Examples:
  # Basic backup
  $0

  # Backup to specific directory
  BACKUP_DIR=/tmp/vault-backup $0

  # Backup from different Vault instance
  VAULT_ADDR=https://vault.company.com VAULT_TOKEN=your-token $0

EOF
    exit 0
fi

# Run main function
main "$@"
