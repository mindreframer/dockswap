# Dockswap Configuration System - Requirements Document

## 1. Overview

The dockswap configuration system manages both system-level configuration (how dockswap operates) and application-level configuration (how individual apps are deployed). This document defines requirements for the system configuration layer only.

## 2. Functional Requirements

### 2.1 Configuration File Management
- **REQ-001**: Support YAML configuration file format
- **REQ-002**: Accept explicit configuration file path via CLI flag
- **REQ-003**: Search default configuration file locations in priority order
- **REQ-004**: Validate configuration file syntax and required fields
- **REQ-005**: Provide clear error messages for invalid configurations
- **REQ-006**: Support configuration file hot reload without restart

### 2.2 Configuration Override Hierarchy
- **REQ-007**: Load configuration in priority order: CLI flags > environment variables > config file > defaults
- **REQ-008**: Support environment variable overrides for all configuration options
- **REQ-009**: Support CLI flag overrides for common configuration options
- **REQ-010**: Merge configuration from multiple sources without conflicts
- **REQ-011**: Display effective configuration for debugging purposes

### 2.3 System Configuration Categories
- **REQ-012**: Configure file system paths (data directory, apps directory, state directory)
- **REQ-013**: Configure SQLite database location and options
- **REQ-014**: Configure API server settings (listen address, port, timeouts)
- **REQ-015**: Configure Docker integration settings (socket path, API version, timeouts)
- **REQ-016**: Configure logging settings (level, output file, format)
- **REQ-017**: Configure deployment behavior (concurrency, timeouts, cleanup policies)

### 2.4 Directory Structure Management
- **REQ-018**: Create required directories if they don't exist
- **REQ-019**: Validate directory permissions for read/write access
- **REQ-020**: Support both absolute and relative path configurations
- **REQ-021**: Expand environment variables in path configurations
- **REQ-022**: Validate that configured paths don't conflict

### 2.5 Configuration Validation
- **REQ-023**: Validate all file paths are accessible
- **REQ-024**: Validate network addresses and ports are available
- **REQ-025**: Validate Docker socket connectivity
- **REQ-026**: Validate SQLite database can be created/opened
- **REQ-027**: Validate configuration value ranges and formats
- **REQ-028**: Fail fast on invalid configuration during startup

## 3. Non-Functional Requirements

### 3.1 Usability
- **NFR-001**: Provide sensible defaults for all configuration options
- **NFR-002**: Minimize required configuration for basic usage
- **NFR-003**: Support configuration-free operation for development
- **NFR-004**: Generate example configuration files
- **NFR-005**: Provide configuration validation command

### 3.2 Maintainability
- **NFR-006**: Use standard YAML syntax without custom extensions
- **NFR-007**: Support configuration versioning for future changes
- **NFR-008**: Provide configuration migration tools when needed
- **NFR-009**: Document all configuration options with examples

### 3.3 Security
- **NFR-010**: Protect sensitive configuration values (if any)
- **NFR-011**: Validate file permissions on configuration files
- **NFR-012**: Support secure defaults (localhost-only API binding)
- **NFR-013**: Warn about insecure configuration options

## 4. Configuration Schema

### 4.1 Required Configuration Sections
- **System settings**: Data storage, logging, basic operation
- **API settings**: HTTP server configuration
- **Docker settings**: Docker daemon integration
- **Deployment settings**: Deployment behavior and limits

### 4.2 Default Configuration Locations (Priority Order)
1. Explicit path via `--config` flag
2. `./dockswap.yaml` (current working directory)
3. `/etc/dockswap/dockswap.yaml` (system-wide)
4. `$HOME/.config/dockswap/dockswap.yaml` (user-specific)
5. Built-in defaults (no file required)

### 4.3 Environment Variable Convention
- **PREFIX**: All environment variables prefixed with `DOCKSWAP_`
- **NAMING**: Uppercase, underscore-separated, matching YAML structure
- **NESTING**: Use double underscore for nested configuration (e.g., `DOCKSWAP_API__PORT`)

## 5. CLI Integration Requirements

### 5.1 Configuration Commands
- **REQ-029**: `dockswap config show` - Display effective configuration
- **REQ-030**: `dockswap config validate` - Validate configuration without starting
- **REQ-031**: `dockswap config example` - Generate example configuration file
- **REQ-032**: `dockswap config paths` - Show configuration file search paths

### 5.2 Configuration Flags
- **REQ-033**: `--config PATH` - Specify configuration file location
- **REQ-034**: `--data-dir PATH` - Override data directory
- **REQ-035**: `--api-port PORT` - Override API port
- **REQ-036**: `--log-level LEVEL` - Override log level
- **REQ-037**: `--dry-run` - Validate configuration and exit

## 6. Error Handling Requirements

### 6.1 Configuration Loading Errors
- **REQ-038**: Graceful handling of missing configuration files
- **REQ-039**: Clear error messages for YAML syntax errors
- **REQ-040**: Specific errors for missing required configuration
- **REQ-041**: Warnings for deprecated configuration options
- **REQ-042**: Suggestions for fixing common configuration errors

### 6.2 Runtime Configuration Errors
- **REQ-043**: Handle directory permission changes gracefully
- **REQ-044**: Detect and report SQLite database corruption
- **REQ-045**: Handle Docker daemon disconnection
- **REQ-046**: Validate configuration changes during hot reload

## 7. Success Criteria

### 7.1 Ease of Use
- New users can run dockswap with zero configuration
- Common configuration changes require only single parameter updates
- Configuration errors provide actionable error messages
- Documentation examples work without modification

### 7.2 Operational Requirements
- Configuration loading completes in < 1 second
- Hot reload of configuration without service interruption
- Configuration validation catches 95% of common errors
- All configuration options have working defaults

## 8. Out of Scope

### 8.1 Excluded Features
- **EXC-001**: Dynamic configuration updates via API (beyond hot reload)
- **EXC-002**: Configuration encryption or secrets management
- **EXC-003**: Configuration templating or variable substitution
- **EXC-004**: Remote configuration sources (URLs, databases)
- **EXC-005**: Configuration format conversion (YAML to JSON, etc.)
- **EXC-006**: GUI configuration management tools