# FrankenPHP Manager

CLI tool to manage PHP development servers powered by **FrankenPHP**.  
Auto-provisions MySQL, Redis, and multi-version PHP — all via Docker.

## Requirements

- **Docker** (running)
- No Go needed for end users (install script provides pre-built binary)

## Quick Install

```bash
git clone https://github.com/kelvinzer0/frankenphp-manager.git
cd frankenphp-manager
bash scripts/install.sh
```

Or build from source:

```bash
go build -o frankenphp ./cmd/frankenphp/
sudo mv frankenphp /usr/local/bin/
```

## First-Time Setup

```bash
frankenphp init
```

This will:
1. Check Docker is running
2. Create a shared Docker network
3. Start MySQL container (port 13306)
4. Start Redis container (port 16379)

## Usage

### Add a project

```bash
frankenphp add /path/to/php/project
```

Interactive wizard will ask:
- Project name (auto-detected from directory)
- PHP version (auto-detected from `composer.json`)
- Port (auto-assigned from 8000+)
- MySQL database (auto-created)
- Redis (optional)

### Manage projects

```bash
frankenphp list                  # List all projects
frankenphp start myapp           # Start a project
frankenphp stop myapp            # Stop a project
frankenphp restart myapp         # Restart a project
frankenphp start all             # Start all projects
frankenphp stop all              # Stop all projects
frankenphp rm myapp              # Remove a project
```

### View logs

```bash
frankenphp log myapp             # View last 100 lines
frankenphp log myapp -f          # Follow in real-time
frankenphp log myapp -t 50       # Last 50 lines
frankenphp log all -f            # All projects, real-time
```

### Switch PHP version

```bash
frankenphp use myapp 8.4         # Switch to PHP 8.4
frankenphp use myapp 8.1         # Switch to PHP 8.1
```

Supported: `8.1`, `8.2`, `8.3`, `8.4`

### Database management

```bash
frankenphp manage createdb myapp # Create project database
frankenphp manage dropdb myapp   # Drop project database
frankenphp manage listdb         # List all databases
frankenphp manage query mydb "SELECT * FROM users"
frankenphp manage mysql          # MySQL connection info
frankenphp manage redis          # Redis connection info
```

### System status

```bash
frankenphp status                # Show Docker, MySQL, Redis, projects
```

### Destroy everything

```bash
frankenphp destroy               # Remove all containers, volumes, config
frankenphp destroy -f            # Skip confirmation
```

## Architecture

```
Docker Network: frankenphp-net
├── frankenphp-mysql     (shared MySQL 8.0, port 13306)
├── frankenphp-redis     (shared Redis 7, port 16379)
├── frankenphp-myapp     (PHP 8.3, port 8000)
├── frankenphp-api       (PHP 8.4, port 8001)
└── frankenphp-legacy    (PHP 8.1, port 8002)
```

Each project runs its own FrankenPHP container with the specified PHP version.  
MySQL and Redis are shared across all projects.

## Config

Config file: `~/.frankenphp/config.yaml`

```yaml
mysql_root_pass: "frankenphp"
mysql_user: "frankenphp"
mysql_pass: "frankenphp"
mysql_port: "13306"
redis_port: "16379"
projects:
  myapp:
    name: myapp
    directory: /home/user/projects/myapp
    php_version: "8.3"
    port: "8000"
    db_name: myapp
    use_redis: true
```

## Project Auto-Detection

When adding a project, the tool auto-detects:
- **PHP version** from `composer.json`
- **Framework** from project files (Laravel, Symfony, WordPress)
- **Next available port** starting from 8000

## HTTPS / ACME (Let's Encrypt)

Three HTTPS modes per project:

### `http` — Local dev (default)
```bash
frankenphp add /path/to/project
# Choose: HTTPS mode → http
# Serves on http://localhost:8000
```

### `selfsigned` — Local HTTPS testing
```bash
frankenphp add /path/to/project
# Choose: HTTPS mode → selfsigned
# Serves on https://localhost:8000 (self-signed cert)
# Browser will show warning — this is expected
```

### `acme` — Production (Let's Encrypt)
```bash
frankenphp add /path/to/project
# Choose: HTTPS mode → acme
# Enter domain: example.com
# Enter email: admin@example.com
# Serves on https://example.com (real cert, auto-renewed)
```

**Requirements for ACME:**
- Domain must point to your server's public IP
- Ports 80 and 443 must be accessible from the internet
- Let's Encrypt will verify domain ownership via HTTP-01 challenge

### Switch HTTPS mode later
```bash
frankenphp tls myapp selfsigned    # Switch to self-signed
frankenphp tls myapp acme          # Switch to ACME
frankenphp tls myapp http          # Back to HTTP only
```

## License

MIT License — see [LICENSE](LICENSE)
