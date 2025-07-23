package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Docker      Docker      `yaml:"docker"`
	Ports       Ports       `yaml:"ports"`
	HealthCheck HealthCheck `yaml:"health_check"`
	Deployment  Deployment  `yaml:"deployment"`
	Proxy       Proxy       `yaml:"proxy"`
}

type Docker struct {
	RestartPolicy string            `yaml:"restart_policy"`
	PullPolicy    string            `yaml:"pull_policy"`
	MemoryLimit   string            `yaml:"memory_limit"`
	CPULimit      string            `yaml:"cpu_limit"`
	Environment   map[string]string `yaml:"environment"`
	Volumes       []string          `yaml:"volumes"`
	ExposePort    int               `yaml:"expose_port"`
	Network       string            `yaml:"network"`
}

type Ports struct {
	Blue  int `yaml:"blue"`
	Green int `yaml:"green"`
}

type HealthCheck struct {
	Endpoint         string        `yaml:"endpoint"`
	Method           string        `yaml:"method"`
	Timeout          time.Duration `yaml:"timeout"`
	Interval         time.Duration `yaml:"interval"`
	Retries          int           `yaml:"retries"`
	SuccessThreshold int           `yaml:"success_threshold"`
	ExpectedStatus   int           `yaml:"expected_status"`
}

type Deployment struct {
	StartupDelay time.Duration `yaml:"startup_delay"`
	DrainTimeout time.Duration `yaml:"drain_timeout"`
	StopTimeout  time.Duration `yaml:"stop_timeout"`
	AutoRollback bool          `yaml:"auto_rollback"`
}

type Proxy struct {
	ListenPort int    `yaml:"listen_port"`
	Host       string `yaml:"host"`
	PathPrefix string `yaml:"path_prefix"`
}

func LoadAppConfig(configPath string) (*AppConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config %s: %w", configPath, err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed for %s: %w", configPath, err)
	}

	return &config, nil
}

func LoadAllConfigs(configDir string) (map[string]*AppConfig, error) {
	configs := make(map[string]*AppConfig)

	err := filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml") {
			config, err := LoadAppConfig(path)
			if err != nil {
				return fmt.Errorf("failed to load config %s: %w", path, err)
			}
			configs[config.Name] = config
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load configs from %s: %w", configDir, err)
	}

	return configs, nil
}

func validateConfig(config *AppConfig) error {
	if config.Name == "" {
		return fmt.Errorf("app name is required")
	}

	if config.Docker.ExposePort <= 0 {
		return fmt.Errorf("docker.expose_port must be positive")
	}

	if config.Ports.Blue <= 0 || config.Ports.Green <= 0 {
		return fmt.Errorf("blue and green ports must be positive")
	}

	if config.Ports.Blue == config.Ports.Green {
		return fmt.Errorf("blue and green ports must be different")
	}

	if config.HealthCheck.Retries < 0 {
		return fmt.Errorf("health_check.retries must be non-negative")
	}

	if config.HealthCheck.SuccessThreshold <= 0 {
		return fmt.Errorf("health_check.success_threshold must be positive")
	}

	if config.HealthCheck.ExpectedStatus < 100 || config.HealthCheck.ExpectedStatus >= 600 {
		return fmt.Errorf("health_check.expected_status must be a valid HTTP status code")
	}

	return nil
}
