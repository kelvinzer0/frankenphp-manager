#!/bin/bash
set -e

APP_NAME="frankenphp-manager"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/${APP_NAME}"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    error "This script must be run as root (use sudo)"
fi

# Ask for confirmation
echo ""
warn "This will remove ${APP_NAME} and ALL its configuration."
read -p "Are you sure? (y/N): " confirm
if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
    info "Cancelled."
    exit 0
fi

# Stop and disable service
if systemctl is-active --quiet "${APP_NAME}" 2>/dev/null; then
    info "Stopping ${APP_NAME} service..."
    systemctl stop "${APP_NAME}"
fi

if systemctl is-enabled --quiet "${APP_NAME}" 2>/dev/null; then
    info "Disabling ${APP_NAME} service..."
    systemctl disable "${APP_NAME}"
fi

# Remove service file
if [ -f "${SERVICE_FILE}" ]; then
    info "Removing systemd service..."
    rm -f "${SERVICE_FILE}"
fi

# Reload systemd
info "Reloading systemd..."
systemctl daemon-reload

# Remove binary
if [ -f "${INSTALL_DIR}/${APP_NAME}" ]; then
    info "Removing binary from ${INSTALL_DIR}..."
    rm -f "${INSTALL_DIR}/${APP_NAME}"
fi

# Remove service wrapper
if [ -f "${INSTALL_DIR}/${APP_NAME}-service" ]; then
    info "Removing service wrapper..."
    rm -f "${INSTALL_DIR}/${APP_NAME}-service"
fi
if [ -f "/usr/sbin/${APP_NAME}" ]; then
    rm -f "/usr/sbin/${APP_NAME}"
fi

# Ask about config removal
echo ""
read -p "Also remove config directory ${CONFIG_DIR}? (y/N): " remove_config
if [ "$remove_config" = "y" ] || [ "$remove_config" = "Y" ]; then
    if [ -d "${CONFIG_DIR}" ]; then
        info "Removing config directory..."
        rm -rf "${CONFIG_DIR}"
    fi
else
    info "Config directory kept at ${CONFIG_DIR}"
fi

info ""
info "=========================================="
info " Uninstallation complete!"
info "=========================================="
info ""
