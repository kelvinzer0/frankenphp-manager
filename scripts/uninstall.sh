#!/bin/bash
set -e

APP="frankenphp"
INSTALL_DIR="/usr/local/bin"

RED='\033[0;31m'; GREEN='\033[0;32m'; NC='\033[0m'
info() { echo -e "${GREEN}[INFO]${NC} $1"; }

if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}[ERROR]${NC} Run with sudo"; exit 1
fi

echo ""
echo -e "${RED}This will remove:${NC}"
echo "  • ${INSTALL_DIR}/${APP} (binary)"
echo "  • ~/.frankenphp/ (all config + project data)"
echo ""
read -p "Continue? (y/N): " confirm
if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
    info "Cancelled."; exit 0
fi

# Remove binary
rm -f "${INSTALL_DIR}/${APP}"
info "Binary removed"

# Ask about config
read -p "Remove ~/.frankenphp/ config directory? (y/N): " rmconfig
if [ "$rmconfig" = "y" ] || [ "$rmconfig" = "Y" ]; then
    rm -rf ~/.frankenphp/
    info "Config removed"
else
    info "Config kept at ~/.frankenphp/"
fi

info "✔ Uninstalled"
