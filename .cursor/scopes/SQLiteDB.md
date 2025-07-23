# Dockswap SQLite DB Spec

## Purpose & User Problem

Enable robust, auditable, and reliable blue-green deployments for multiple containerized applications by persisting deployment state, config history, and event logs. The DB supports zero-downtime deploys, troubleshooting, and future extensibility.

## Success Criteria
- All deployment state transitions and events are recorded and queryable.
- Every app config version is stored with timestamp and SHA for change detection.
- The current deployment state for each app is always available.
- Full deployment history (including rollbacks) is queryable.
- Schema supports multiple apps, no user/actor tracking, and no UI/dashboard requirements.

## Scope & Constraints
- **In scope:**
  - Multi-app support
  - Full config and deployment history
  - State machine event logging
  - Current deployment state tracking
- **Out of scope:**
  - User/actor tracking
  - UI/dashboard-specific tables
  - Container logs, metrics, or secrets

## Technical Considerations
- Use SQLite (single file, local, transactional)
- Store raw YAML config and SHA for each version
- Store event payloads as JSON (for flexibility)
- Use foreign keys for referential integrity
- Optimize for write-heavy, read-light workloads
- Plan for future schema migrations

## Proposed Schema

### 1. `app_configs`
| Column         | Type      | Notes                        |
| --------------|-----------|------------------------------|
| id            | INTEGER   | PK, autoincrement            |
| app_name      | TEXT      | Indexed                      |
| config_yaml   | TEXT      | Raw YAML                     |
| config_sha    | TEXT      | SHA256 of YAML, indexed      |
| created_at    | DATETIME  | When config was added        |

- One row per config version per app.
- Query latest by `app_name` and max(`created_at`).

### 2. `deployments`
| Column         | Type      | Notes                        |
| --------------|-----------|------------------------------|
| id            | INTEGER   | PK, autoincrement            |
| app_name      | TEXT      | Indexed                      |
| config_id     | INTEGER   | FK to app_configs(id)        |
| image         | TEXT      | Docker image                 |
| started_at    | DATETIME  | Deployment start             |
| ended_at      | DATETIME  | Deployment end (nullable)    |
| status        | TEXT      | success, failed, rolled_back |
| active_color  | TEXT      | blue/green                   |
| rollback_of   | INTEGER   | FK to deployments(id), nullable |

- One row per deployment attempt.
- Links to config used and (optionally) rollback source.

### 3. `deployment_events`
| Column         | Type      | Notes                        |
| --------------|-----------|------------------------------|
| id            | INTEGER   | PK, autoincrement            |
| deployment_id | INTEGER   | FK to deployments(id)        |
| app_name      | TEXT      | Indexed                      |
| event_type    | TEXT      | From state_machine.go        |
| payload       | TEXT      | JSON-encoded event data      |
| error         | TEXT      | Error message, if any        |
| created_at    | DATETIME  | Timestamp                    |

- One row per state machine transition/event.
- Allows full timeline reconstruction.

### 4. `current_state`
| Column         | Type      | Notes                        |
| --------------|-----------|------------------------------|
| app_name      | TEXT      | PK                           |
| deployment_id | INTEGER   | FK to deployments(id)        |
| active_color  | TEXT      | blue/green                   |
| image         | TEXT      | Current image                |
| status        | TEXT      | stable, failed, etc.         |
| updated_at    | DATETIME  | Last update                  |

- One row per app, always up-to-date.
- Updated on every deployment state change.

## Example Queries / Use Cases

- **Get current state for all apps:**
  ```sql
  SELECT * FROM current_state;
  ```
- **Get deployment history for an app:**
  ```sql
  SELECT * FROM deployments WHERE app_name = 'web-api' ORDER BY started_at DESC;
  ```
- **Get all events for a deployment:**
  ```sql
  SELECT * FROM deployment_events WHERE deployment_id = ? ORDER BY created_at ASC;
  ```
- **Get config history for an app:**
  ```sql
  SELECT * FROM app_configs WHERE app_name = 'web-api' ORDER BY created_at DESC;
  ```
- **Detect config changes:**
  - Compare latest `config_sha` to previous for an app.

## Out of Scope
- User/actor tracking
- UI/dashboard tables
- Container logs, metrics, secrets

## Future Considerations
- Add schema versioning for migrations
- Add indices for performance as needed
- Consider WAL mode for high write concurrency 