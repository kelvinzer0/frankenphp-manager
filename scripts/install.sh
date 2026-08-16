#!/bin/bash
set -e

APP_NAME="frankenphp-manager"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/${APP_NAME}"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

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

# Check Go installation
if ! command -v go &> /dev/null; then
    error "Go is not installed. Please install Go first: https://go.dev/dl/"
fi

info "Building ${APP_NAME}..."
cd "$PROJECT_ROOT"
CGO_ENABLED=0 go build -ldflags "-s -w" -o "${APP_NAME}" cmd/server/main.go

info "Installing binary to ${INSTALL_DIR}..."
mv "${APP_NAME}" "${INSTALL_DIR}/${APP_NAME}"
chmod 755 "${INSTALL_DIR}/${APP_NAME}"

info "Creating config directory ${CONFIG_DIR}..."
mkdir -p "${CONFIG_DIR}"
chmod 700 "${CONFIG_DIR}"

# Install default config if not exists
if [ ! -f "${CONFIG_DIR}/config.yaml" ]; then
    info "Creating default config file..."
    cat > "${CONFIG_DIR}/config.yaml" << 'EOF'
server:
  host: ""
  host_ipv4: ""
  host_ipv6: ""
  port: "8080"
auth:
  username: ""
  password_hash: ""
servers_config_path: /etc/frankenphp-manager/servers.json
acme:
  enabled: false
  email: ""
  domains: []
  storage_path: /etc/frankenphp-manager/certs
EOF
    chmod 600 "${CONFIG_DIR}/config.yaml"
    warn "Default config created at ${CONFIG_DIR}/config.yaml"
    warn "You need to run '${APP_NAME}' once to set up username and password."
else
    info "Existing config file found, keeping it."
fi

info "Installing systemd service..."
cat > "${SERVICE_FILE}" << EOF
[Unit]
Description=FrankenPHP Manager - Web-based PHP Server Manager
After=network.target
Documentation=https://github.com/kelvinzer0/frankenphp-manager

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${APP_NAME}
WorkingDirectory=${CONFIG_DIR}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${APP_NAME}

# Security hardening
NoNewPrivileges=false
ProtectSystem=strict
ReadWritePaths=${CONFIG_DIR}
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

info "Reloading systemd..."
systemctl daemon-reload

info "Enabling ${APP_NAME} service..."
systemctl enable "${APP_NAME}"

info "Installing service wrapper..."
# Install the service command wrapper so users can run:
#   service frankenphp-manager {start|stop|restart|status|logs|config}
cp "${SCRIPT_DIR}/frankenphp-manager-service.sh" "${INSTALL_DIR}/${APP_NAME}-service"
chmod 755 "${INSTALL_DIR}/${APP_NAME}-service"
# Also create a symlink in /usr/sbin for the 'service' command to find it
# The 'service' command on most Linux distros looks in /etc/init.d/ or uses systemctl
# We create a simple wrapper in /usr/sbin for convenience
cat > "/usr/sbin/${APP_NAME}" << WRAPPER
#!/bin/bash
exec "${INSTALL_DIR}/${APP_NAME}-service" "\\$@"
WRAPPER
chmod 755 "/usr/sbin/${APP_NAME}"

info ""
info "=========================================="
info " Installation complete!"
info "=========================================="
info ""
info " To start the service:"
info "   sudo systemctl start ${APP_NAME}"
info ""
info " Or use the service command:"
info "   sudo service ${APP_NAME} start"
info ""
info " To configure:"
info "   sudo nano ${CONFIG_DIR}/config.yaml"
info ""
info " To view logs:"
info "   journalctl -u ${APP_NAME} -f"
info ""
