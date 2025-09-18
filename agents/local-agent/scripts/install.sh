#!/bin/bash

# AegisFlux Agent Installation Script
# This script installs the AegisFlux agent as a systemd service

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
CACHE_DIR="/tmp/aegisflux-agent"

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

check_systemd() {
    if ! command -v systemctl &> /dev/null; then
        error "systemd is not available on this system"
        exit 1
    fi
}

create_user() {
    if ! id "$USER_NAME" &>/dev/null; then
        log "Creating user $USER_NAME"
        useradd --system --no-create-home --shell /bin/false "$USER_NAME"
    else
        log "User $USER_NAME already exists"
    fi
}

install_binary() {
    if [[ ! -f "$BINARY_NAME" ]]; then
        error "Binary $BINARY_NAME not found in current directory"
        error "Please build the agent first: make build"
        exit 1
    fi
    
    log "Installing binary to $INSTALL_DIR"
    cp "$BINARY_NAME" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    chown root:root "$INSTALL_DIR/$BINARY_NAME"
}

install_service() {
    log "Installing systemd service"
    cp "systemd/$SERVICE_NAME.service" "$SERVICE_DIR/"
    
    # Enable the service
    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME"
    
    log "Service installed and enabled"
}

install_environment() {
    log "Creating environment directory"
    mkdir -p "$ENV_DIR"
    
    if [[ ! -f "$ENV_DIR/$SERVICE_NAME.env" ]]; then
        log "Installing environment template"
        cp "systemd/$SERVICE_NAME.env" "$ENV_DIR/"
        chown root:root "$ENV_DIR/$SERVICE_NAME.env"
        chmod 644 "$ENV_DIR/$SERVICE_NAME.env"
        
        warn "Please edit $ENV_DIR/$SERVICE_NAME.env to configure the agent"
    else
        log "Environment file already exists"
    fi
}

create_directories() {
    log "Creating directories"
    mkdir -p "$LOG_DIR"
    mkdir -p "$CACHE_DIR"
    
    chown "$USER_NAME:$GROUP_NAME" "$LOG_DIR"
    chown "$USER_NAME:$GROUP_NAME" "$CACHE_DIR"
    chmod 755 "$LOG_DIR"
    chmod 755 "$CACHE_DIR"
}

configure_firewall() {
    if command -v ufw &> /dev/null; then
        log "Configuring UFW firewall"
        ufw allow 8080/tcp comment "AegisFlux Agent HTTP API"
    elif command -v firewall-cmd &> /dev/null; then
        log "Configuring firewalld"
        firewall-cmd --permanent --add-port=8080/tcp
        firewall-cmd --reload
    else
        warn "No supported firewall found, please configure manually if needed"
    fi
}

show_status() {
    log "Installation completed successfully!"
    echo
    echo "Service status:"
    systemctl status "$SERVICE_NAME" --no-pager || true
    echo
    echo "Next steps:"
    echo "1. Edit $ENV_DIR/$SERVICE_NAME.env to configure the agent"
    echo "2. Start the service: systemctl start $SERVICE_NAME"
    echo "3. Check logs: journalctl -u $SERVICE_NAME -f"
    echo "4. Check health: curl http://localhost:8080/healthz"
    echo
    echo "Useful commands:"
    echo "  systemctl start $SERVICE_NAME     # Start the service"
    echo "  systemctl stop $SERVICE_NAME      # Stop the service"
    echo "  systemctl restart $SERVICE_NAME   # Restart the service"
    echo "  systemctl status $SERVICE_NAME    # Check service status"
    echo "  journalctl -u $SERVICE_NAME -f    # Follow logs"
}

main() {
    log "Starting AegisFlux Agent installation"
    
    check_root
    check_systemd
    create_user
    install_binary
    install_service
    install_environment
    create_directories
    configure_firewall
    show_status
}

main "$@"
