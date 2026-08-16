# FrankenPHP Manager

A web-based tool to manage your FrankenPHP development servers. Start, stop, and configure multiple PHP servers from a clean web interface.

## Features

- 🖥️ Manage multiple PHP development servers from one dashboard
- ▶️ Start/stop servers with one click
- ⚙️ Configure host, port, document root, and custom commands
- 🔒 Password-protected web UI (Basic Auth)
- 🌐 IPv4, IPv6, and domain name support
- 🔐 ACME/Let's Encrypt integration (optional)
- 🛡️ Input sanitization & rate limiting
- 📦 Single binary, easy to install

## Requirements

- **Go 1.23+** (for building from source)
- **FrankenPHP** installed and in your PATH — [Install FrankenPHP](https://frankenphp.dev/docs/install/)

## Quick Install (Linux)

```bash
git clone https://github.com/kelvinzer0/frankenphp-manager.git
cd frankenphp-manager
sudo bash scripts/install.sh
```

This will:
1. Build the binary
2. Install to `/usr/local/bin/`
3. Create config at `/etc/frankenphp-manager/`
4. Set up a systemd service

Then run it once to set up your username and password:

```bash
sudo frankenphp-manager
```

## Service Management

After installation, manage the service easily:

```bash
# Using systemctl
sudo systemctl start frankenphp-manager
sudo systemctl stop frankenphp-manager
sudo systemctl status frankenphp-manager
sudo systemctl restart frankenphp-manager

# Or using the service wrapper
sudo service frankenphp-manager start
sudo service frankenphp-manager stop
sudo service frankenphp-manager restart
sudo service frankenphp-manager status
sudo service frankenphp-manager logs      # Follow logs
sudo service frankenphp-manager config    # Edit config
```

## Uninstall

```bash
sudo bash scripts/uninstall.sh
```

## Configuration

Config file: `/etc/frankenphp-manager/config.yaml`

```yaml
server:
  # Option 1: Explicit dual-stack (recommended)
  host_ipv4: "0.0.0.0"    # Bind IPv4 on all interfaces
  host_ipv6: "[::]"       # Bind IPv6 on all interfaces
  port: "8080"

  # Option 2: Single address (legacy)
  # host: "[::]"          # [::] = dual-stack, 0.0.0.0 = IPv4 only
auth:
  username: admin
  password_hash: "$2a$10$..."  # bcrypt hash
servers_config_path: /etc/frankenphp-manager/servers.json
```

## API Endpoints

All endpoints require Basic Auth.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/servers` | List all servers |
| POST | `/api/servers` | Create a server |
| PUT | `/api/servers/{id}` | Update a server |
| DELETE | `/api/servers/{id}` | Delete a server |
| POST | `/api/servers/{id}/start` | Start a server |
| POST | `/api/servers/{id}/stop` | Stop a server |
| GET | `/api/servers/{id}/status` | Get server status |
| GET | `/api/settings` | Get management settings |
| PUT | `/api/settings` | Update management settings |
| PUT | `/api/auth` | Update credentials |

## Network Binding (IPv4 / IPv6 / Dual-Stack)

Two modes of operation:

### Mode 1: Explicit Dual-Stack (Recommended)

Set both `host_ipv4` and `host_ipv6` to bind **two separate listeners**:

```yaml
server:
  host_ipv4: "0.0.0.0"   # Listener 1: all IPv4 interfaces
  host_ipv6: "[::]"      # Listener 2: all IPv6 interfaces
  port: "8080"
```

This creates **two independent listeners**, one for each protocol. Works regardless of OS `bindv6only` setting.

### Mode 2: Single Address (Legacy)

Use `host` for a single listener:

```yaml
server:
  host: "[::]"    # Dual-stack via OS kernel (depends on net.ipv6.bindv6only)
  port: "8080"
```

| Host Value | Behavior |
|------------|----------|
| `[::]` | Dual-stack (OS-dependent) |
| `0.0.0.0` | IPv4 only |
| `::1` / `127.0.0.1` | Loopback only |

**Priority:** If `host_ipv4` or `host_ipv6` is set, they take priority over `host`.

## Custom Command Placeholders

When using custom commands, these placeholders are replaced:

| Placeholder | Value |
|-------------|-------|
| `{host}` | Server host |
| `{port}` | Server port |
| `{directory}` | Document root |
| `{listen_addr}` | Full listen address (host:port) |

## Building from Source

```bash
go build -o frankenphp-manager cmd/server/main.go
```

## Tech Stack

- **Backend:** Go (gorilla/mux, certmagic)
- **Frontend:** Vanilla HTML/CSS/JS (embedded)
- **Server:** FrankenPHP

## License

MIT License — see [LICENSE](LICENSE)
