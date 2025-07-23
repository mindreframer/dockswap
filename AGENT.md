## Dockswap – Project Overview

**Dockswap** is a blue-green deployment tool for containerized applications, written in Go. It aims to provide zero-downtime deployments with Docker integration, health checking, and traffic management via Caddy Server.

---

### High-Level Structure

- **CLI Layer** (`internal/cli/`): Parses commands and global flags. *Most commands are currently stubs and do not perform real deployments yet.*
- **Configuration System** (`internal/config/`): Loads and validates YAML app configs (see example below).
- **Docker Integration** (`internal/docker/`): Contains real logic for managing containers, health checks, and orchestrating deployments, but not yet wired up to the CLI.
- **Caddy Integration** (`internal/caddy/`): For HTTP proxy and traffic switching (planned).
- **Deployment State Machine** (`internal/deployment/`): Models deployment process and state transitions.
- **Data Layer**: Planned for future (SQLite for deployment history).

---

### Expectations & Status
- work in small, incremental steps
- make sure code compiles
- implement tests along each features
- make sure tests are passing

---

### Makefile Commands

- `make build` – Build the dockswap binary
- `make run` – Run the CLI with `--help`
- `make fmt` – Format code
- `make vet` – Run `go vet`
- `make test` – Run tests (basic, not comprehensive)
- `make lint` – Run linter (if installed)
- `make mod-tidy` – Tidy Go modules
- `make clean` – Remove build artifacts
- `make all` – Run format, vet, test, and build

---

### Example App Config (YAML)

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

---

### How to Build & Run

```bash
make build
./dockswap --help
```

---

### Key Notes

- **Most CLI commands are placeholders**: They print simulated output and do not interact with Docker yet.
- **Docker logic is implemented**: Real container management and health checks exist in `internal/docker/`, but are not yet connected to the CLI.
- **Specs & Design Docs**: See `@spec/` for requirements and design details.
- **Development is ongoing**: See Makefile and code comments for guidance.
