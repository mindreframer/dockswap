# Dockswap Configuration System - Design Document

## 1. Architecture Overview

### 1.1 Configuration Flow
```
CLI Flags → Environment Variables → Config File → Built-in Defaults
    ↓              ↓                    ↓              ↓
    └──────────────┴────────────────────┴──────────────┘
                           │
                    Configuration Merger
                           │
                    ┌──────▼──────┐
                    │  Effective  │
                    │Configuration│
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ Validation  │
                    │   Engine    │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ Application │
                    │ Components  │
                    └─────────────┘
```

### 1.2 Component Responsibilities
- **ConfigLoader**: Finds and loads configuration files
- **ConfigMerger**: Combines configuration from multiple sources
- **ConfigValidator**: Validates merged configuration
- **PathManager**: Manages directory creation and validation
- **HotReloader**: Handles configuration file changes

## 2. Configuration Schema Design

### 2.1 Full Configuration Structure
```yaml
# /etc/dockswap/dockswap.yaml
version: "1"  # Configuration schema version

system:
  data_dir: "/var/lib/dockswap"
  apps_config_dir: "/etc/dockswap/apps"
  state_dir: "/etc/dockswap/state"
  pid_file: "/var/run/dockswap.pid"
  create_dirs: true  # Auto-create missing directories

logging:
  level: "info"  # trace, debug, info, warn, error
  format: "json"  # json, text
  output: "file"  # file, stdout, stderr
  file: "/var/log/dockswap.log"
  max_size: "100MB"
  max_age: "30d"
  max_backups: 5
  compress: true

api:
  listen_address: "127.0.0.1"
  port: 9999
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "60s"
  enable_cors: false
  cors_origins: ["*"]

database:
  sqlite_path: "/var/lib/dockswap/dockswap.db"
  wal_mode: true
  busy_timeout: "5s"
  max_connections: 10
  backup_interval: "1h"
  backup_retention: "7d"

docker:
  socket_path: "/var/run/docker.sock"
  api_version: "1.41"
  timeout: "60s"
  registry_auth: false
  pull_timeout: "300s"
  network_name: "dockswap"
  cleanup_images: false

deployment:
  max_concurrent: 3
  default_drain_timeout: "30s"
  default_health_timeout: "120s"
  container_stop_timeout: "30s"
  cleanup_on_failure: true
  retry_attempts: 3
  retry_delay: "5s"

monitoring:
  enable_metrics: true
  metrics_port: 9998
  health_check_interval: "30s"
  collect_container_stats: true
```

### 2.2 Environment Variable Mapping
```go
// Environment variable to config path mapping
var envMapping = map[string]string{
    "DOCKSWAP_DATA_DIR":           "system.data_dir",
    "DOCKSWAP_APPS_CONFIG_DIR":    "system.apps_config_dir",
    "DOCKSWAP_STATE_DIR":          "system.state_dir",
    "DOCKSWAP_LOG_LEVEL":          "logging.level",
    "DOCKSWAP_LOG_FILE":           "logging.file",
    "DOCKSWAP_API_PORT":           "api.port",
    "DOCKSWAP_API_ADDRESS":        "api.listen_address",
    "DOCKSWAP_SQLITE_PATH":        "database.sqlite_path",
    "DOCKSWAP_DOCKER_SOCKET":      "docker.socket_path",
    "DOCKSWAP_MAX_CONCURRENT":     "deployment.max_concurrent",
}
```

## 3. Implementation Design

### 3.1 Configuration Structures
```go
// Root configuration structure
type Config struct {
    Version    string            `yaml:"version"`
    System     SystemConfig      `yaml:"system"`
    Logging    LoggingConfig     `yaml:"logging"`
    API        APIConfig         `yaml:"api"`
    Database   DatabaseConfig    `yaml:"database"`
    Docker     DockerConfig      `yaml:"docker"`
    Deployment DeploymentConfig  `yaml:"deployment"`
    Monitoring MonitoringConfig  `yaml:"monitoring"`
}

// System configuration
type SystemConfig struct {
    DataDir       string `yaml:"data_dir"`
    AppsConfigDir string `yaml:"apps_config_dir"`
    StateDir      string `yaml:"state_dir"`
    PIDFile       string `yaml:"pid_file"`
    CreateDirs    bool   `yaml:"create_dirs"`
}

// API configuration
type APIConfig struct {
    ListenAddress string        `yaml:"listen_address"`
    Port          int           `yaml:"port"`
    ReadTimeout   time.Duration `yaml:"read_timeout"`
    WriteTimeout  time.Duration `yaml:"write_timeout"`
    IdleTimeout   time.Duration `yaml:"idle_timeout"`
    EnableCORS    bool          `yaml:"enable_cors"`
    CORSOrigins   []string      `yaml:"cors_origins"`
}

// Additional config structs for other sections...
```

### 3.2 Configuration Loading Logic
```go
type ConfigManager struct {
    configPath     string
    config         *Config
    validators     []ConfigValidator
    reloadChan     chan struct{}
    mu             sync.RWMutex
}

func NewConfigManager(configPath string) *ConfigManager {
    return &ConfigManager{
        configPath: configPath,
        validators: []ConfigValidator{
            &PathValidator{},
            &NetworkValidator{},
            &DockerValidator{},
            &DatabaseValidator{},
        },
    }
}

func (cm *ConfigManager) Load() (*Config, error) {
    // 1. Start with defaults
    config := DefaultConfig()
    
    // 2. Find and load config file
    if configPath := cm.findConfigFile(); configPath != "" {
        if err := cm.loadFromFile(config, configPath); err != nil {
            return nil, fmt.Errorf("loading config file: %w", err)
        }
    }
    
    // 3. Apply environment variables
    if err := cm.loadFromEnv(config); err != nil {
        return nil, fmt.Errorf("loading from environment: %w", err)
    }
    
    // 4. Apply CLI flags
    if err := cm.loadFromFlags(config); err != nil {
        return nil, fmt.Errorf("loading from flags: %w", err)
    }
    
    // 5. Validate merged configuration
    if err := cm.validate(config); err != nil {
        return nil, fmt.Errorf("config validation: %w", err)
    }
    
    // 6. Expand paths and create directories
    if err := cm.setupPaths(config); err != nil {
        return nil, fmt.Errorf("setting up paths: %w", err)
    }
    
    cm.mu.Lock()
    cm.config = config
    cm.mu.Unlock()
    
    return config, nil
}
```

### 3.3 Configuration File Discovery
```go
func (cm *ConfigManager) findConfigFile() string {
    searchPaths := []string{
        cm.configPath,                              // Explicit --config flag
        "./dockswap.yaml",                         // Current directory
        "/etc/dockswap/dockswap.yaml",            // System-wide
        filepath.Join(os.Getenv("HOME"), ".config/dockswap/dockswap.yaml"), // User
    }
    
    for _, path := range searchPaths {
        if path == "" {
            continue
        }
        
        expanded := os.ExpandEnv(path)
        if _, err := os.Stat(expanded); err == nil {
            return expanded
        }
    }
    
    return "" // No config file found, use defaults
}
```

### 3.4 Configuration Validation Framework
```go
type ConfigValidator interface {
    Validate(config *Config) error
    Name() string
}

type PathValidator struct{}

func (pv *PathValidator) Validate(config *Config) error {
    paths := []struct {
        name string
        path string
        needsWrite bool
    }{
        {"data_dir", config.System.DataDir, true},
        {"apps_config_dir", config.System.AppsConfigDir, false},
        {"state_dir", config.System.StateDir, true},
    }
    
    for _, p := range paths {
        if err := validatePath(p.path, p.needsWrite); err != nil {
            return fmt.Errorf("%s (%s): %w", p.name, p.path, err)
        }
    }
    
    return nil
}

type NetworkValidator struct{}

func (nv *NetworkValidator) Validate(config *Config) error {
    // Validate API port availability
    addr := fmt.Sprintf("%s:%d", config.API.ListenAddress, config.API.Port)
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        return fmt.Errorf("API address %s not available: %w", addr, err)
    }
    listener.Close()
    
    return nil
}
```

### 3.5 Hot Reload Implementation
```go
func (cm *ConfigManager) StartHotReload(ctx context.Context) error {
    if cm.configPath == "" {
        return nil // No config file to watch
    }
    
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    
    if err := watcher.Add(filepath.Dir(cm.configPath)); err != nil {
        return err
    }
    
    go func() {
        defer watcher.Close()
        
        for {
            select {
            case event := <-watcher.Events:
                if event.Name == cm.configPath && event.Op&fsnotify.Write == fsnotify.Write {
                    cm.handleConfigChange()
                }
            case err := <-watcher.Errors:
                log.Error("Config watcher error: %v", err)
            case <-ctx.Done():
                return
            }
        }
    }()
    
    return nil
}

func (cm *ConfigManager) handleConfigChange() {
    log.Info("Configuration file changed, reloading...")
    
    newConfig, err := cm.Load()
    if err != nil {
        log.Error("Failed to reload configuration: %v", err)
        return
    }
    
    // Notify components of config change
    select {
    case cm.reloadChan <- struct{}{}:
    default:
        // Non-blocking send
    }
    
    log.Info("Configuration reloaded successfully")
}
```

## 4. Default Configuration

### 4.1 Built-in Defaults
```go
func DefaultConfig() *Config {
    return &Config{
        Version: "1",
        System: SystemConfig{
            DataDir:       "/var/lib/dockswap",
            AppsConfigDir: "/etc/dockswap/apps",
            StateDir:      "/etc/dockswap/state",
            PIDFile:       "/var/run/dockswap.pid",
            CreateDirs:    true,
        },
        Logging: LoggingConfig{
            Level:      "info",
            Format:     "json",
            Output:     "file",
            File:       "/var/log/dockswap.log",
            MaxSize:    "100MB",
            MaxAge:     "30d",
            MaxBackups: 5,
            Compress:   true,
        },
        API: APIConfig{
            ListenAddress: "127.0.0.1",
            Port:          9999,
            ReadTimeout:   30 * time.Second,
            WriteTimeout:  30 * time.Second,
            IdleTimeout:   60 * time.Second,
            EnableCORS:    false,
        },
        Database: DatabaseConfig{
            SQLitePath:      "/var/lib/dockswap/dockswap.db",
            WALMode:         true,
            BusyTimeout:     5 * time.Second,
            MaxConnections:  10,
            BackupInterval:  time.Hour,
            BackupRetention: 7 * 24 * time.Hour,
        },
        Docker: DockerConfig{
            SocketPath:    "/var/run/docker.sock",
            APIVersion:    "1.41",
            Timeout:       60 * time.Second,
            PullTimeout:   300 * time.Second,
            NetworkName:   "dockswap",
            CleanupImages: false,
        },
        Deployment: DeploymentConfig{
            MaxConcurrent:       3,
            DefaultDrainTimeout: 30 * time.Second,
            DefaultHealthTimeout: 120 * time.Second,
            ContainerStopTimeout: 30 * time.Second,
            CleanupOnFailure:    true,
            RetryAttempts:       3,
            RetryDelay:          5 * time.Second,
        },
        Monitoring: MonitoringConfig{
            EnableMetrics:         true,
            MetricsPort:          9998,
            HealthCheckInterval:  30 * time.Second,
            CollectContainerStats: true,
        },
    }
}
```

## 5. CLI Integration

### 5.1 Configuration Commands
```go
func configShowCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "show",
        Short: "Display effective configuration",
        RunE: func(cmd *cobra.Command, args []string) error {
            config, err := loadConfig()
            if err != nil {
                return err
            }
            
            output, _ := yaml.Marshal(config)
            fmt.Print(string(output))
            return nil
        },
    }
}

func configValidateCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "validate",
        Short: "Validate configuration without starting",
        RunE: func(cmd *cobra.Command, args []string) error {
            _, err := loadConfig()
            if err != nil {
                fmt.Printf("Configuration validation failed: %v\n", err)
                return err
            }
            
            fmt.Println("Configuration is valid")
            return nil
        },
    }
}

func configExampleCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "example",
        Short: "Generate example configuration file",
        RunE: func(cmd *cobra.Command, args []string) error {
            config := DefaultConfig()
            output, err := yaml.Marshal(config)
            if err != nil {
                return err
            }
            
            fmt.Print(string(output))
            return nil
        },
    }
}
```

## 6. Error Handling Strategy

### 6.1 Configuration Error Types
```go
type ConfigError struct {
    Type    string
    Field   string
    Value   interface{}
    Message string
    Cause   error
}

func (e *ConfigError) Error() string {
    if e.Field != "" {
        return fmt.Sprintf("configuration error in %s: %s", e.Field, e.Message)
    }
    return fmt.Sprintf("configuration error: %s", e.Message)
}

// Specific error types
var (
    ErrConfigNotFound     = errors.New("configuration file not found")
    ErrInvalidYAML       = errors.New("invalid YAML syntax")
    ErrInvalidPath       = errors.New("invalid or inaccessible path")
    ErrPortInUse         = errors.New("port already in use")
    ErrDockerUnavailable = errors.New("Docker daemon unavailable")
)
```

### 6.2 User-Friendly Error Messages
```go
func formatConfigError(err error) string {
    switch {
    case errors.Is(err, ErrInvalidYAML):
        return `Configuration file contains invalid YAML syntax.
Please check for:
- Proper indentation (use spaces, not tabs)
- Matching quotes and brackets
- Valid YAML structure

Use 'dockswap config validate' to check syntax.`
        
    case errors.Is(err, ErrPortInUse):
        return `The configured API port is already in use.
Try:
- Change the port in configuration: api.port
- Set environment variable: DOCKSWAP_API_PORT=8888
- Use CLI flag: --api-port 8888`
        
    default:
        return err.Error()
    }
}
```

## 7. Testing Strategy

### 7.1 Configuration Testing Approach
- **Unit tests**: Individual validator components
- **Integration tests**: Full configuration loading scenarios
- **File system tests**: Directory creation and permissions
- **Network tests**: Port binding and availability
- **Environment tests**: Various deployment scenarios

### 7.2 Test Configuration Files
```yaml
# test/configs/minimal.yaml
system:
  data_dir: "/tmp/dockswap-test"

# test/configs/full.yaml  
# Complete configuration with all options

# test/configs/invalid.yaml
# Various invalid configurations for error testing
```