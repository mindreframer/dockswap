# Dockswap

Dockswap is a blue-green deployment CLI tool for containerized applications. It provides zero-downtime deployments with robust health checking, connection draining, and seamless Docker integration. Dockswap is written in Go and operates as a single binary, making it easy to use across Linux-based environments.

---

## Key Features

- **Zero-downtime deployments:** Safely switch between blue and green environments with connection draining.
- **Tight Docker integration:** Manages containers directly using the Docker API.
- **Health checks:** Automated, configurable HTTP health checks before promoting new containers.
- **Caddy integration:** Uses Caddy as a reverse proxy to switch HTTP/WebSocket traffic between blue/green environments.
- **Simple configuration:** Per-app YAML files in a dedicated config directory.
- **Persistence:** Uses SQLite for state, deployment history, and health check results.
- **CLI-first:** All operations are performed via the command-line interface.

---

## How Dockswap Works

Dockswap manages two sets of containers (blue and green) for each application. When a new deployment is triggered, it starts the standby environment, runs health checks, and only switches traffic (via Caddy) if the new version is healthy. Active connections are drained before the old environment is stopped, ensuring zero downtime.

---

## Example Usage Flow

### 1. Prepare the Config Directory

Create a config directory (e.g., `~/dockswap-cfg/`). Dockswap expects the following structure:

```
~/dockswap-cfg/
├── apps/
│   └── web-api.yaml
├── state/
└── caddy/
```

### 2. Define Application Configuration

Each app has its own YAML file in `apps/`. Example (`apps/web-api.yaml`):

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

### 3. Run Dockswap CLI

```sh
dockswap --config ~/dockswap-cfg
```

- Dockswap will auto-create missing folders (`apps/`, `state/`, `caddy/`) if needed.
- It will validate all YAML files in `apps/` for correctness.
- The SQLite DB will be stored at `~/dockswap-cfg/dockswap.db`.

### 4. Deploy a New Version

```sh
dockswap deploy web-api myapp:v1.2.3
```

Dockswap will pull the new image, start the standby container, run health checks, and switch Caddy traffic if healthy.

### 5. Check Status

```sh
dockswap status web-api
```

### 6. Rollback or Manual Switch

```sh
dockswap switch web-api blue
```

---

## System Requirements

- Linux-based host
- Docker daemon
- Go (for building from source) or prebuilt binary
- SQLite (bundled, no external database required)
- Caddy (for HTTP proxying; managed by Dockswap)

---

## Getting Started

1. Install Dockswap (build from source or download a release).
2. Create a config directory and write YAML configuration files for your applications in `apps/`.
3. Run the Dockswap CLI, pointing to your config directory.
4. Use the CLI to manage deployments and monitor status.

For detailed requirements, design, and implementation plan, see the `@spec/0-init/` folder.

