# Color-Specific Environment Variable Overrides

## Overview
This feature allows users to define color-specific environment variable overrides in their YAML configuration, enabling different environment settings for blue and green deployments while maintaining shared base configuration.

## Motivation
In blue-green deployments, containers often need different configuration values based on their deployment color (blue/green). Common use cases include:
- Different internal ports (PORT env var matching the container's internal port)
- Different database connection strings for testing isolation
- Different feature flags or configuration endpoints
- Color-specific logging or monitoring settings

## Current State
Currently, the `docker.environment` field only supports static key-value pairs that are identical for both blue and green deployments.

## Proposed Solution

### YAML Configuration Structure
```yaml
name: "my_app"
image: "my_app:20250722-1453-3f3af16"
ports:
  proxy: 8001
  blue: 18001
  green: 18002
docker:
  environment:
    DATABASE_URL: "ecto://postgres:postgres@host.docker.internal:5466/my_app_dev"
    SECRET_KEY_BASE: "JH6SfaiDXEfDZS95zbBy7Psc3iROy8ZvmZcvM3NzH514FEd1vkuYZDfVirNyoBNq"
    PHX_SERVER: "true"
    MIX_ENV: "prod"
    PORT: "4000"  # base/default value
  environment_overrides:
    blue:
      PORT: "18001"           # overrides PORT for blue deployment
      LOG_LEVEL: "debug"      # blue-specific setting
    green:
      PORT: "18002"           # overrides PORT for green deployment
      LOG_LEVEL: "info"       # green-specific setting
  memory_limit: "512m"
  cpu_limit: "0.5"
  expose_port: 8001
  restart_policy: "unless-stopped"
```

### Environment Resolution Logic
1. Start with base `docker.environment` variables
2. Apply color-specific overrides from `docker.environment_overrides[color]`
3. Color-specific values take precedence over base values
4. Variables not specified in overrides retain their base values

### Example Resolution
**Base environment:**
```
DATABASE_URL=ecto://postgres:postgres@host.docker.internal:5466/my_app_dev
SECRET_KEY_BASE=JH6SfaiDXEfDZS95zbBy7Psc3iROy8ZvmZcvM3NzH514FEd1vkuYZDfVirNyoBNq
PHX_SERVER=true
MIX_ENV=prod
PORT=4000
```

**Blue deployment environment:**
```
DATABASE_URL=ecto://postgres:postgres@host.docker.internal:5466/my_app_dev
SECRET_KEY_BASE=JH6SfaiDXEfDZS95zbBy7Psc3iROy8ZvmZcvM3NzH514FEd1vkuYZDfVirNyoBNq
PHX_SERVER=true
MIX_ENV=prod
PORT=18001        # overridden
LOG_LEVEL=debug   # added
```

**Green deployment environment:**
```
DATABASE_URL=ecto://postgres:postgres@host.docker.internal:5466/my_app_dev
SECRET_KEY_BASE=JH6SfaiDXEfDZS95zbBy7Psc3iROy8ZvmZcvM3NzH514FEd1vkuYZDfVirNyoBNq
PHX_SERVER=true
MIX_ENV=prod
PORT=18002        # overridden
LOG_LEVEL=info    # added
```

## Implementation Details

### Go Struct Changes
```go
type Docker struct {
    RestartPolicy        string                      `yaml:"restart_policy"`
    PullPolicy          string                       `yaml:"pull_policy"`
    MemoryLimit         string                       `yaml:"memory_limit"`
    CPULimit            string                       `yaml:"cpu_limit"`
    Environment         map[string]string            `yaml:"environment"`
    EnvironmentOverrides map[string]map[string]string `yaml:"environment_overrides"`
    Volumes             []string                     `yaml:"volumes"`
    ExposePort          int                          `yaml:"expose_port"`
    Network             string                       `yaml:"network"`
}
```

### New Utility Function
```go
// GetEnvironmentForColor merges base environment with color-specific overrides
func (d *Docker) GetEnvironmentForColor(color string) map[string]string {
    result := make(map[string]string)
    
    // Copy base environment
    for k, v := range d.Environment {
        result[k] = v
    }
    
    // Apply color-specific overrides
    if colorOverrides, exists := d.EnvironmentOverrides[color]; exists {
        for k, v := range colorOverrides {
            result[k] = v
        }
    }
    
    return result
}
```

### Validation Rules
1. `environment_overrides` is optional
2. Only "blue" and "green" colors are supported as keys
3. Override values must be strings (same as base environment)
4. Empty override maps are allowed
5. Override keys can be new variables or override existing base variables

### Backward Compatibility
- Existing configurations without `environment_overrides` continue to work unchanged
- The `environment` field remains required for base environment variables
- No breaking changes to existing YAML structure

## Testing Requirements

### Unit Tests
1. YAML parsing with environment overrides
2. Environment merging logic for blue/green colors
3. Validation of override structure
4. Backward compatibility with configs lacking overrides
5. Edge cases (empty overrides, invalid colors, etc.)

### Integration Tests
1. Full configuration loading with overrides
2. Environment resolution in deployment scenarios

## Migration Path
1. Implement struct changes and utility functions
2. Update YAML parsing and validation
3. Add comprehensive tests
4. Update example configurations
5. No migration needed for existing configs (backward compatible)

## Future Considerations
- Potential extension to support other colors beyond blue/green
- Template support for dynamic environment variable generation
- Validation against port conflicts with override PORT values