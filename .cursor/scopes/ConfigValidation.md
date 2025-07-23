# Dockswap Config Directory Validation & Auto-Creation Spec

## Purpose
Ensure the Dockswap config directory is valid and ready for use by:
- Automatically creating required subfolders if missing.
- Validating the content and schema of YAML config files.

## Required Folder Structure
- `apps/` (for per-app YAML configs)
- `state/` (for state files, if needed)
- `caddy/` (for Caddy config files)

If any of these folders are missing, they must be created automatically at startup.

## YAML Content Validation
- All `.yml` or `.yaml` files in `apps/` must be valid YAML.
- Each app config must conform to the expected schema (see below).
- If any file is invalid or missing required fields, return a clear error indicating which file and what is wrong.

### Example App Config Schema
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
- Required top-level fields: `name`, `docker`, `ports`, `health_check`
- `docker` must have at least `memory_limit`, `environment`, `expose_port`
- `ports` must have `blue` and `green`
- `health_check` must have `endpoint`, `timeout`, `retries`

## Success Criteria
- All required folders are present (auto-created if missing).
- All app YAML files are valid and conform to schema.
- Clear errors for any missing/invalid files or folders.
- Validation runs at startup and can be unit tested.

## Scope
- Implement a function (e.g., `ValidateAndPrepareConfigDir(path string) error`)
- Auto-create missing folders (`apps/`, `state/`, `caddy/`)
- Validate all YAML files in `apps/` for schema
- Unit tests for all error and valid cases

## Technical Considerations
- Use `os.MkdirAll` for folder creation
- Use `gopkg.in/yaml.v3` for YAML parsing
- Use Go structs for schema validation
- Return errors with file/folder context
- Should be easily testable (can use temp dirs)

## Out of Scope
- Loading or parsing non-app configs (e.g., global.yml)
- Auto-creating example YAML files (unless specified)
- UI/CLI prompts for missing configs

## Testability
- Unit tests must cover:
  - All folders missing (all created)
  - Some folders missing (created)
  - Invalid YAML file (error)
  - Missing required fields (error)
  - All valid (no error)
- Tests should use temp dirs and files, no real user data 