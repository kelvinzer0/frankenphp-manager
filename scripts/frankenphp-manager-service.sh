#!/bin/bash
#
# frankenphp-manager service wrapper
#
# Usage: service frankenphp-manager {start|stop|restart|status|logs|config}
#

APP_NAME="frankenphp-manager"
CONFIG_DIR="/etc/${APP_NAME}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

usage() {
    echo ""
    echo -e "${CYAN}Usage:${NC} service ${APP_NAME} {command}"
    echo ""
    echo "Commands:"
    echo "  start     Start the service"
    echo "  stop      Stop the service"
    echo "  restart   Restart the service"
    echo "  status    Show service status"
    echo "  logs      Follow service logs (Ctrl+C to exit)"
    echo "  config    Edit configuration file"
    echo ""
    exit 1
}

case "${1}" in
    start)
        echo -e "${GREEN}Starting ${APP_NAME}...${NC}"
        systemctl start "${APP_NAME}"
        sleep 1
        if systemctl is-active --quiet "${APP_NAME}"; then
            echo -e "${GREEN}✓ ${APP_NAME} started successfully${NC}"
            # Show the listening address from config
            PORT=$(grep -oP 'port:\s*"\K[0-9]+' "${CONFIG_DIR}/config.yaml" 2>/dev/null || echo "8080")
            HOST=$(grep -oP 'host:\s*\K[0-9.]+' "${CONFIG_DIR}/config.yaml" 2>/dev/null || echo "0.0.0.0")
            echo -e "  Web UI: http://${HOST}:${PORT}"
        else
            echo -e "${RED}✗ Failed to start. Check: journalctl -u ${APP_NAME} -n 20${NC}"
            exit 1
        fi
        ;;
    stop)
        echo -e "${YELLOW}Stopping ${APP_NAME}...${NC}"
        systemctl stop "${APP_NAME}"
        echo -e "${GREEN}✓ ${APP_NAME} stopped${NC}"
        ;;
    restart)
        echo -e "${YELLOW}Restarting ${APP_NAME}...${NC}"
        systemctl restart "${APP_NAME}"
        sleep 1
        if systemctl is-active --quiet "${APP_NAME}"; then
            echo -e "${GREEN}✓ ${APP_NAME} restarted successfully${NC}"
        else
            echo -e "${RED}✗ Failed to restart. Check: journalctl -u ${APP_NAME} -n 20${NC}"
            exit 1
        fi
        ;;
    status)
        systemctl status "${APP_NAME}" --no-pager
        ;;
    logs)
        echo -e "${CYAN}Following logs for ${APP_NAME} (Ctrl+C to exit)...${NC}"
        journalctl -u "${APP_NAME}" -f --no-pager
        ;;
    config)
        if [ ! -f "${CONFIG_DIR}/config.yaml" ]; then
            echo -e "${RED}Config file not found at ${CONFIG_DIR}/config.yaml${NC}"
            exit 1
        fi
        ${EDITOR:-nano} "${CONFIG_DIR}/config.yaml"
        echo -e "${YELLOW}Config updated. Run: service ${APP_NAME} restart${NC}"
        ;;
    *)
        usage
        ;;
esac
