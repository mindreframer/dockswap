# Dockswap

Dockswap is a blue-green deployment tool designed for containerized applications. It provides zero-downtime deployments with robust health checking, connection draining, and seamless Docker integration. Dockswap is written in Go and operates as a single binary, making it easy to deploy and manage across Linux-based environments.

---

## Key Features

- Zero-downtime deployments: Safely switch between blue and green environments with connection draining.
- Tight Docker integration: Manages containers directly using the Docker API.
- Health checks: Automated, configurable HTTP health checks before promoting new containers.
- Reverse proxy: Handles HTTP and WebSocket traffic, routing requests to the active environment.
- Configuration: Simple YAML files for per-application settings.
- Persistence: Uses SQLite for state, deployment history, and health check results.
- API and CLI: Provides both a REST API and a command-line interface for operational tasks.

---

## How Dockswap Works

Dockswap manages two sets of containers (blue and green) for each application. When a new deployment is triggered, it starts the standby environment, runs health checks, and only switches traffic if the new version is healthy. Active connections are drained before the old environment is stopped, ensuring zero downtime.

---

## Example Usage Flow

### 1. Define Application Configuration

Create a YAML file for your application (e.g., `/etc/blue-green/apps/web-api.yaml`):

```yaml
name: "web-api"
image: "myapp:latest"
ports:
  proxy: 8000
  blue: 18000
  green: 18001
health_check:
  path: "/health"
  method: "GET"
  timeout: "5s"
  interval: "2s"
  healthy_threshold: 3
  unhealthy_threshold: 2
  expected_status: 200
docker:
  environment:
    - "DATABASE_URL=postgres://user:pass@host:5432/db"
  volumes:
    - "/host/data:/app/data:rw"
  memory_limit: "512m"
  cpu_limit: "0.5"
deployment:
  drain_timeout: "30s"
  startup_timeout: "60s"
  health_check_timeout: "120s"
```

### 2. Start Dockswap

```sh
dockswap --config-dir /etc/blue-green/apps
```

Dockswap will load all application configurations, initialize the database, and start listening on the configured proxy ports.

### 3. Deploy a New Version

You can trigger a deployment using the CLI or API.

CLI Example:

```sh
dockswap deploy web-api myapp:v1.2.3
```

API Example:

```sh
curl -X POST http://localhost:8080/api/apps/web-api/deploy \
  -d '{"desired_image": "myapp:v1.2.3"}'
```

Dockswap will pull the new image, start the standby container, run health checks, and switch traffic if healthy.

### 4. Check Status

CLI:

```sh
dockswap status web-api
```

API:

```sh
curl http://localhost:8080/api/apps/web-api/status
```

### 5. Rollback or Manual Switch

If needed, you can manually switch traffic or rollback:

```sh
dockswap switch web-api blue
```

---

## System Requirements

- Linux-based host
- Docker daemon
- Go (for building from source) or prebuilt binary
- SQLite (bundled, no external database required)

---

## Getting Started

1. Install Dockswap (build from source or download a release).
2. Write YAML configuration files for your applications.
3. Start the Dockswap binary, pointing to your config directory.
4. Use the CLI or API to manage deployments and monitor status.

For detailed requirements, design, and implementation plan, see the `@spec/0-init/` folder.

