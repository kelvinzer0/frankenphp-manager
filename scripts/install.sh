#!/bin/bash
set -e

APP="frankenphp"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Check root
if [ "$(id -u)" -ne 0 ]; then
    error "Run with sudo: sudo bash scripts/install.sh"
fi

# Check Go
if ! command -v go &> /dev/null; then
    error "Go not installed. Install: https://go.dev/dl/"
fi

# Check Docker
if ! docker info &> /dev/null; then
    error "Docker not running. Install: https://docs.docker.com/get-docker/"
fi

# Build
info "Building ${APP}..."
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"
CGO_ENABLED=0 go build -ldflags "-s -w" -o "${APP}" ./cmd/frankenphp/

# Install
info "Installing to ${INSTALL_DIR}..."
mv "${APP}" "${INSTALL_DIR}/${APP}"
chmod 755 "${INSTALL_DIR}/${APP}"

info ""
info "✔ Installed! Run: frankenphp init"
info ""
