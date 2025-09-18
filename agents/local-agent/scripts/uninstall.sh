#!/bin/bash

# AegisFlux Agent Uninstallation Script
# This script removes the AegisFlux agent and its systemd service

set -euo pipefail

# Configuration
BINARY_NAME="aegisflux-agent"
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="aegisflux-agent"
SERVICE_DIR="/etc/systemd/system"
ENV_DIR="/etc/aegisflux"
USER_NAME="aegisflux"
GROUP_NAME="aegisflux"
LOG_DIR="/var/log/aegisflux"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root"
        exit 1
    fi
}

stop_service() {
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        log "Stopping service"
        systemctl stop "$SERVICE_NAME"
    else
        log "Service is not running"
    fi
}

disable_service() {
    if systemctl is-enabled --quiet "$SERVICE_NAME"; then
        log "Disabling service"
        systemctl disable "$SERVICE_NAME"
    else
        log "Service is not enabled"
    fi
}

remove_service() {
    if [[ -f "$SERVICE_DIR/$SERVICE_NAME.service" ]]; then
        log "Removing systemd service"
        rm -f "$SERVICE_DIR/$SERVICE_NAME.service"
        systemctl daemon-reload
    else
        log "Service file not found"
    fi
}

remove_binary() {
    if [[ -f "$INSTALL_DIR/$BINARY_NAME" ]]; then
        log "Removing binary"
        rm -f "$INSTALL_DIR/$BINARY_NAME"
    else
        log "Binary not found"
    fi
}

remove_directories() {
    log "Removing directories"
    
    if [[ -d "$LOG_DIR" ]]; then
        rm -rf "$LOG_DIR"
        log "Removed log directory: $LOG_DIR"
    fi
    
    if [[ -d "$ENV_DIR" ]]; then
        rm -rf "$ENV_DIR"
        log "Removed environment directory: $ENV_DIR"
    fi
}

remove_user() {
    if id "$USER_NAME" &>/dev/null; then
        log "Removing user $USER_NAME"
        userdel "$USER_NAME" 2>/dev/null || true
    else
        log "User $USER_NAME does not exist"
    fi
}

cleanup_firewall() {
    if command -v ufw &> /dev/null; then
        log "Removing UFW firewall rules"
        ufw delete allow 8080/tcp 2>/dev/null || true
    elif command -v firewall-cmd &> /dev/null; then
        log "Removing firewalld rules"
        firewall-cmd --permanent --remove-port=8080/tcp 2>/dev/null || true
        firewall-cmd --reload 2>/dev/null || true
    fi
}

show_completion() {
    log "Uninstallation completed successfully!"
    echo
    echo "The following have been removed:"
    echo "- Systemd service: $SERVICE_NAME"
    echo "- Binary: $INSTALL_DIR/$BINARY_NAME"
    echo "- User: $USER_NAME"
    echo "- Directories: $LOG_DIR, $ENV_DIR"
    echo "- Firewall rules (if configured)"
    echo
    echo "Note: Cache directories in /tmp may still exist and will be cleaned up on reboot"
}

main() {
    log "Starting AegisFlux Agent uninstallation"
    
    check_root
    stop_service
    disable_service
    remove_service
    remove_binary
    remove_directories
    remove_user
    cleanup_firewall
    show_completion
}

main "$@"
