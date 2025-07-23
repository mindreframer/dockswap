# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Dockswap is a blue-green deployment tool for containerized applications written in Go 1.24.1. It provides zero-downtime deployments with Docker integration, health checking, and traffic management through an HTTP proxy layer.

## Build and Development Commands

```bash
# Build the project
go build -o dockswap .

# Run the application
go run main.go --help

# Update dependencies
go mod tidy

# Format and vet code
go fmt ./...
go vet ./...
```

The project currently builds to a single binary. No test framework is implemented yet.

## Architecture

### High-Level Structure
The project follows a standard Go CLI application pattern:

- **CLI Layer** (`internal/cli/`): Command parsing and execution with global flags support
- **Configuration System** (`internal/config/`): YAML-based app configuration with validation
- **Core Logic** (planned): Docker management, health checking, deployment state machine
- **Data Layer** (planned): SQLite for deployment history and state

### Configuration Architecture
Applications are configured via YAML files with this structure:
- Docker container settings (limits, environment, volumes)
- Blue/green port allocation
- Health check configuration (endpoint, timeouts, retry logic)
- Deployment behavior (drain timeouts, rollback settings)
- Proxy configuration (host, ports, path routing)

The config system supports:
- Single file loading with `LoadAppConfig()`
- Directory scanning with `LoadAllConfigs()`
- Validation of ports, timeouts, and HTTP status codes
- Duration parsing (e.g., "5s", "30s")

### CLI Structure
Commands follow this pattern:
```bash
dockswap [global-flags] <command> [args] [command-flags]
```

Global flags (`--config`, `--log-level`) are parsed before command routing. All commands are currently implemented as stubs in `internal/cli/commands.go`.

## Key Directories

- **`@spec/`**: Comprehensive specification documents detailing system design, requirements, and implementation phases
- **`internal/cli/`**: CLI implementation with command parsing and stub handlers
- **`internal/config/`**: Configuration structures and YAML loading with validation
- **`cmd/dockswap/`**: Command entry point (currently empty)
- **`pkg/`**: Public packages (reserved for future shared components)

## Development Status

**Current Phase**: Configuration system implementation (recently completed config reader)

**Next Priorities**:
1. Docker integration and container management
2. Health checking engine
3. HTTP proxy and traffic management
4. Deployment state machine
5. SQLite data layer

The project is in early development with basic CLI structure and configuration system established. Most command implementations are placeholder stubs that need actual Docker and deployment logic.

## Configuration Examples

Application configs are stored as YAML files (typically in `/etc/dockswap/apps/`):

```yaml
name: "web-api"
docker:
  memory_limit: "512m"
  environment:
    DATABASE_URL: "postgres://localhost/webapi"
  expose_port: 8080
ports:
  blue: 8081
  green: 8082
health_check:
  endpoint: "/health"
  timeout: "5s"
  retries: 3
```

The system validates port conflicts, timeout formats, and required fields during config loading.