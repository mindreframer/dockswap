# Blue-Green Deployment System - Design Document

## 1. Architecture Overview

### 1.1 System Architecture
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│     s3dock      │───►│  Blue-Green      │───►│   Docker        │
│                 │    │  Deploy Binary   │    │   Containers    │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌──────────────────┐
                       │   SQLite DB      │
                       └──────────────────┘
```

### 1.2 Component Architecture
```
Blue-Green Deploy Binary
├── HTTP Proxy Layer (per-app ports)
├── Container Manager (Docker API)
├── Health Check Engine
├── Deployment State Machine
├── Configuration Manager
├── SQLite Data Layer
├── HTTP API Server
└── CLI Interface
```

## 2. Data Model

### 2.1 SQLite Schema
```sql
-- Application definitions and current state
CREATE TABLE apps (
    name TEXT PRIMARY KEY,
    current_image TEXT,
    desired_image TEXT,
    active_color TEXT CHECK(active_color IN ('blue', 'green')),
    state TEXT CHECK(state IN ('stable', 'deploying', 'health_checking', 'draining', 'switching', 'failed')),
    proxy_port INTEGER UNIQUE,
    blue_port INTEGER,
    green_port INTEGER,
    config_hash TEXT, -- for detecting config changes
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Container instances
CREATE TABLE containers (
    id TEXT PRIMARY KEY, -- {app_name}-{color}-{timestamp}
    app_name TEXT NOT NULL,
    color TEXT NOT NULL CHECK(color IN ('blue', 'green')),
    image TEXT NOT NULL,
    docker_id TEXT UNIQUE,
    status TEXT CHECK(status IN ('starting', 'healthy', 'unhealthy', 'draining', 'stopped')),
    port INTEGER,
    started_at DATETIME,
    stopped_at DATETIME,
    FOREIGN KEY(app_name) REFERENCES apps(name),
    UNIQUE(app_name, color) -- only one blue/green per app at a time
);

-- Deployment audit trail
CREATE TABLE deployments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_name TEXT NOT NULL,
    from_image TEXT,
    to_image TEXT NOT NULL,
    trigger_source TEXT, -- 's3dock', 'manual', 'api'
    status TEXT CHECK(status IN ('started', 'health_checking', 'draining', 'switching', 'completed', 'failed', 'rolled_back')),
    duration_ms INTEGER,
    error_message TEXT,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    FOREIGN KEY(app_name) REFERENCES apps(name)
);

-- Health check results
CREATE TABLE health_checks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    container_id TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    response_time_ms INTEGER,
    status_code INTEGER,
    error_message TEXT,
    checked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(container_id) REFERENCES containers(id)
);

-- Connection tracking for draining
CREATE TABLE active_connections (
    id TEXT PRIMARY KEY, -- connection identifier
    container_id TEXT NOT NULL,
    connection_type TEXT CHECK(connection_type IN ('http', 'websocket')),
    remote_addr TEXT,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(container_id) REFERENCES containers(id)
);
```

### 2.2 Configuration Format
```yaml
# /etc/blue-green/apps/{app-name}.yaml
name: "web-api"
image: "web-api:20250630-1010-a333666"  # default/current image
ports:
  proxy: 8000          # external facing port
  blue: 18000          # internal blue container port
  green: 18001         # internal green container port
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
    - "REDIS_URL=redis://localhost:6379"
    - "LOG_LEVEL=info"
  volumes:
    - "/host/data:/app/data:rw"
    - "/host/logs:/app/logs:rw"
  memory_limit: "512m"
  cpu_limit: "0.5"
  restart_policy: "unless-stopped"
deployment:
  drain_timeout: "30s"
  startup_timeout: "60s"
  health_check_timeout: "120s"
```

## 3. Core Components

### 3.1 HTTP Proxy Layer
```go
type ProxyManager struct {
    apps     map[string]*AppProxy
    db       *SQLiteDB
    logger   *Logger
}

type AppProxy struct {
    appName     string
    proxyPort   int
    activeColor string
    backends    map[string]*Backend // blue/green backends
    server      *http.Server
    upgrader    websocket.Upgrader
    connTracker *ConnectionTracker
}

type Backend struct {
    containerID string
    address     string
    healthy     bool
    draining    bool
}
```

**Responsibilities:**
- Listen on per-application proxy ports
- Route HTTP/WebSocket traffic to active backend
- Preserve headers (X-Forwarded-For, X-Real-IP, cookies, etc.)
- Track active connections for draining
- Handle WebSocket upgrades and bidirectional proxying

### 3.2 Container Manager
```go
type ContainerManager struct {
    docker   *client.Client
    db       *SQLiteDB
    logger   *Logger
}

type ContainerSpec struct {
    Name        string
    Image       string
    Port        int
    Environment []string
    Volumes     []string
    MemoryLimit int64
}
```

**Responsibilities:**
- Create/start/stop Docker containers
- Pull new images
- Monitor container status
- Clean up stopped containers
- Port management and conflict resolution

### 3.3 Health Check Engine
```go
type HealthChecker struct {
    db       *SQLiteDB
    client   *http.Client
    logger   *Logger
    checkers map[string]*AppHealthChecker
}

type AppHealthChecker struct {
    appName    string
    config     HealthCheckConfig
    ticker     *time.Ticker
    stopCh     chan struct{}
}
```

**Responsibilities:**
- Concurrent health checking for all containers
- HTTP-based health checks with configurable parameters
- Store health check results in database
- Trigger state changes based on health status
- Exponential backoff on failures

### 3.4 Deployment State Machine
```go
type DeploymentManager struct {
    db              *SQLiteDB
    containerMgr    *ContainerManager
    healthChecker   *HealthChecker
    proxyMgr        *ProxyManager
    logger          *Logger
    activeDeployments map[string]*DeploymentState
}

type DeploymentState struct {
    AppName      string
    FromImage    string
    ToImage      string
    Status       DeploymentStatus
    StartedAt    time.Time
    StandbyColor string
    ActiveColor  string
}
```

**State Flow:**
```
STABLE → NEW_IMAGE_DETECTED → STARTING_STANDBY → HEALTH_CHECKING → 
DRAINING_ACTIVE → SWITCHING → CLEANUP → STABLE
                ↓ (on failure)
              FAILED → CLEANUP → STABLE
```

**Responsibilities:**
- Orchestrate deployment workflow
- Manage state transitions
- Handle deployment failures and cleanup
- Coordinate between all components

### 3.5 Configuration Manager
```go
type ConfigManager struct {
    configDir   string
    db          *SQLiteDB
    logger      *Logger
    watcher     *fsnotify.Watcher
    apps        map[string]*AppConfig
}
```

**Responsibilities:**
- Load and validate YAML configurations
- Watch for configuration file changes
- Update database with configuration changes
- Trigger redeployments on config changes

## 4. API Design

### 4.1 HTTP API Endpoints
```
POST /api/apps/{name}/deploy
  Body: {"desired_image": "myapp:v1.2.3"}
  Response: {"deployment_id": 123, "status": "started"}

GET /api/apps
  Response: [{"name": "web-api", "status": "stable", "current_image": "myapp:v1.2.2", ...}]

GET /api/apps/{name}/status
  Response: {"name": "web-api", "status": "stable", "active_color": "blue", ...}

GET /api/apps/{name}/history
  Query: ?limit=10&offset=0
  Response: [{"deployment_id": 123, "from_image": "v1.2.2", "to_image": "v1.2.3", ...}]

GET /api/apps/{name}/health
  Response: {"blue": {"healthy": true, "last_check": "..."}, "green": {...}}

POST /api/apps/{name}/switch
  Body: {"color": "green"}  # manual switch

GET /health
  Response: {"status": "healthy", "apps": {...}}  # system health
```

### 4.2 CLI Interface
```bash
blue-green status [app-name]
blue-green deploy <app-name> <image>
blue-green history <app-name> [--limit 10]
blue-green health <app-name>
blue-green switch <app-name> <color>
blue-green logs <app-name> [--follow]
blue-green config reload [app-name]
```

## 5. Process Flow

### 5.1 Deployment Process
1. **Trigger**: s3dock calls `POST /api/apps/web-api/deploy`
2. **Validation**: Check if app exists, validate image format
3. **State Transition**: STABLE → STARTING_STANDBY
4. **Container Start**: Pull image, start standby container
5. **Health Check**: Wait for health checks to pass
6. **Traffic Switch**: Update proxy to route to standby
7. **Connection Drain**: Drain connections from active container
8. **Cleanup**: Stop old container, update state to STABLE
9. **Audit**: Log deployment completion

### 5.2 Health Check Process
1. **Concurrent Checking**: Check all containers per their intervals
2. **HTTP Request**: GET {container_ip}:{port}{health_path}
3. **Evaluation**: Check status code, response time, retries
4. **State Update**: Update container health status in database
5. **Notification**: Trigger deployment state changes if needed

### 5.3 Connection Draining Process
1. **Stop New Connections**: Proxy stops routing new requests
2. **Track Active**: Monitor active HTTP requests and WebSocket connections
3. **Graceful Wait**: Wait for connections to complete naturally
4. **Hard Timeout**: Force close remaining connections after timeout
5. **Container Stop**: Stop container once all connections drained

## 6. Error Handling

### 6.1 Deployment Failures
- **Image Pull Failure**: Retry with exponential backoff, fail after 3 attempts
- **Container Start Failure**: Log error, clean up, mark deployment as failed
- **Health Check Failure**: Wait for timeout, do not promote, clean up standby
- **Proxy Switch Failure**: Rollback proxy configuration, mark as failed

### 6.2 Runtime Failures
- **Docker Daemon Down**: Log error, attempt reconnection, maintain last known state
- **SQLite Lock**: Retry with backoff, use WAL mode for better concurrency
- **Network Issues**: Circuit breaker pattern for health checks

## 7. Performance Considerations

### 7.1 Scalability
- Support 50+ applications per binary instance
- Handle 1000+ concurrent connections per application
- Process health checks for 100+ containers concurrently

### 7.2 Resource Usage
- Memory: < 100MB base + 10MB per application
- CPU: < 5% during steady state, < 50% during deployments
- Disk: SQLite database growth < 1GB per year with rotation

### 7.3 Network
- Proxy latency overhead < 1ms
- Health check intervals configurable (default 2s)
- Connection pooling for Docker API calls