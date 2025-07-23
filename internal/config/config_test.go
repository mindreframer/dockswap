package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  AppConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: AppConfig{
				Name: "test-app",
				Docker: Docker{
					ExposePort: 8080,
				},
				Ports: Ports{
					Blue:  8081,
					Green: 8082,
				},
				HealthCheck: HealthCheck{
					Retries:          3,
					SuccessThreshold: 2,
					ExpectedStatus:   200,
				},
			},
			wantErr: false,
		},
		{
			name: "missing app name",
			config: AppConfig{
				Docker: Docker{
					ExposePort: 8080,
				},
				Ports: Ports{
					Blue:  8081,
					Green: 8082,
				},
				HealthCheck: HealthCheck{
					Retries:          3,
					SuccessThreshold: 2,
					ExpectedStatus:   200,
				},
			},
			wantErr: true,
			errMsg:  "app name is required",
		},
		{
			name: "invalid expose port",
			config: AppConfig{
				Name: "test-app",
				Docker: Docker{
					ExposePort: 0,
				},
				Ports: Ports{
					Blue:  8081,
					Green: 8082,
				},
				HealthCheck: HealthCheck{
					Retries:          3,
					SuccessThreshold: 2,
					ExpectedStatus:   200,
				},
			},
			wantErr: true,
			errMsg:  "docker.expose_port must be positive",
		},
		{
			name: "invalid blue port",
			config: AppConfig{
				Name: "test-app",
				Docker: Docker{
					ExposePort: 8080,
				},
				Ports: Ports{
					Blue:  0,
					Green: 8082,
				},
				HealthCheck: HealthCheck{
					Retries:          3,
					SuccessThreshold: 2,
					ExpectedStatus:   200,
				},
			},
			wantErr: true,
			errMsg:  "blue and green ports must be positive",
		},
		{
			name: "same blue and green ports",
			config: AppConfig{
				Name: "test-app",
				Docker: Docker{
					ExposePort: 8080,
				},
				Ports: Ports{
					Blue:  8081,
					Green: 8081,
				},
				HealthCheck: HealthCheck{
					Retries:          3,
					SuccessThreshold: 2,
					ExpectedStatus:   200,
				},
			},
			wantErr: true,
			errMsg:  "blue and green ports must be different",
		},
		{
			name: "negative retries",
			config: AppConfig{
				Name: "test-app",
				Docker: Docker{
					ExposePort: 8080,
				},
				Ports: Ports{
					Blue:  8081,
					Green: 8082,
				},
				HealthCheck: HealthCheck{
					Retries:          -1,
					SuccessThreshold: 2,
					ExpectedStatus:   200,
				},
			},
			wantErr: true,
			errMsg:  "health_check.retries must be non-negative",
		},
		{
			name: "invalid success threshold",
			config: AppConfig{
				Name: "test-app",
				Docker: Docker{
					ExposePort: 8080,
				},
				Ports: Ports{
					Blue:  8081,
					Green: 8082,
				},
				HealthCheck: HealthCheck{
					Retries:          3,
					SuccessThreshold: 0,
					ExpectedStatus:   200,
				},
			},
			wantErr: true,
			errMsg:  "health_check.success_threshold must be positive",
		},
		{
			name: "invalid HTTP status code",
			config: AppConfig{
				Name: "test-app",
				Docker: Docker{
					ExposePort: 8080,
				},
				Ports: Ports{
					Blue:  8081,
					Green: 8082,
				},
				HealthCheck: HealthCheck{
					Retries:          3,
					SuccessThreshold: 2,
					ExpectedStatus:   99,
				},
			},
			wantErr: true,
			errMsg:  "health_check.expected_status must be a valid HTTP status code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(&tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateConfig() expected error but got none")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("validateConfig() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateConfig() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestLoadAppConfig(t *testing.T) {
	tempDir := t.TempDir()

	validYAML := `name: test-app
description: "Test application"
docker:
  restart_policy: "unless-stopped"
  pull_policy: "always"
  memory_limit: "512m"
  cpu_limit: "0.5"
  environment:
    DATABASE_URL: "postgres://localhost/test"
    LOG_LEVEL: "info"
  volumes:
    - "/var/log:/app/logs"
  expose_port: 8080
  network: "test-network"
ports:
  blue: 8081
  green: 8082
health_check:
  endpoint: "/health"
  method: "GET"
  timeout: "5s"
  interval: "2s"
  retries: 3
  success_threshold: 2
  expected_status: 200
deployment:
  startup_delay: "10s"
  drain_timeout: "30s"
  stop_timeout: "15s"
  auto_rollback: true
proxy:
  listen_port: 80
  host: "test.example.com"
  path_prefix: "/"`

	invalidYAML := `name: test-app
docker:
  expose_port: 8080
ports:
  blue: 8081
  green: 8081
health_check:
  retries: 3
  success_threshold: 2
  expected_status: 200`

	malformedYAML := `name: test-app
docker:
  expose_port: "not-a-number"`

	tests := []struct {
		name     string
		content  string
		wantErr  bool
		wantName string
	}{
		{
			name:     "valid config",
			content:  validYAML,
			wantErr:  false,
			wantName: "test-app",
		},
		{
			name:    "invalid config - same ports",
			content: invalidYAML,
			wantErr: true,
		},
		{
			name:    "malformed YAML",
			content: malformedYAML,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := filepath.Join(tempDir, "test-config.yaml")
			err := os.WriteFile(configFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test config file: %v", err)
			}

			config, err := LoadAppConfig(configFile)
			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadAppConfig() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("LoadAppConfig() unexpected error = %v", err)
				}
				if config == nil {
					t.Errorf("LoadAppConfig() returned nil config")
				} else if config.Name != tt.wantName {
					t.Errorf("LoadAppConfig() name = %v, want %v", config.Name, tt.wantName)
				}
			}
		})
	}

	t.Run("file not found", func(t *testing.T) {
		_, err := LoadAppConfig("/nonexistent/path/config.yaml")
		if err == nil {
			t.Errorf("LoadAppConfig() expected error for nonexistent file")
		}
	})
}

func TestLoadAppConfigDurationParsing(t *testing.T) {
	tempDir := t.TempDir()

	yamlWithDurations := `name: duration-test
docker:
  expose_port: 8080
ports:
  blue: 8081
  green: 8082
health_check:
  timeout: "5s"
  interval: "2s"
  retries: 3
  success_threshold: 2
  expected_status: 200
deployment:
  startup_delay: "10s"
  drain_timeout: "30s"
  stop_timeout: "15s"
  auto_rollback: true`

	configFile := filepath.Join(tempDir, "duration-test.yaml")
	err := os.WriteFile(configFile, []byte(yamlWithDurations), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	config, err := LoadAppConfig(configFile)
	if err != nil {
		t.Fatalf("LoadAppConfig() unexpected error = %v", err)
	}

	expectedDurations := map[string]time.Duration{
		"health_timeout":  5 * time.Second,
		"health_interval": 2 * time.Second,
		"startup_delay":   10 * time.Second,
		"drain_timeout":   30 * time.Second,
		"stop_timeout":    15 * time.Second,
	}

	actualDurations := map[string]time.Duration{
		"health_timeout":  config.HealthCheck.Timeout,
		"health_interval": config.HealthCheck.Interval,
		"startup_delay":   config.Deployment.StartupDelay,
		"drain_timeout":   config.Deployment.DrainTimeout,
		"stop_timeout":    config.Deployment.StopTimeout,
	}

	for key, expected := range expectedDurations {
		if actual := actualDurations[key]; actual != expected {
			t.Errorf("Duration %s = %v, want %v", key, actual, expected)
		}
	}
}

func TestLoadAllConfigs(t *testing.T) {
	tempDir := t.TempDir()

	app1YAML := `name: app1
docker:
  expose_port: 8080
ports:
  blue: 8081
  green: 8082
health_check:
  retries: 3
  success_threshold: 2
  expected_status: 200`

	app2YAML := `name: app2
docker:
  expose_port: 9090
ports:
  blue: 9091
  green: 9092
health_check:
  retries: 3
  success_threshold: 2
  expected_status: 200`

	invalidYAML := `name: invalid-app
docker:
  expose_port: 8080
ports:
  blue: 8081
  green: 8081
health_check:
  retries: 3
  success_threshold: 2
  expected_status: 200`

	err := os.WriteFile(filepath.Join(tempDir, "app1.yaml"), []byte(app1YAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write app1 config: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "app2.yml"), []byte(app2YAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write app2 config: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "not-yaml.txt"), []byte("not yaml"), 0644)
	if err != nil {
		t.Fatalf("Failed to write non-yaml file: %v", err)
	}

	t.Run("load valid configs", func(t *testing.T) {
		configs, err := LoadAllConfigs(tempDir)
		if err != nil {
			t.Fatalf("LoadAllConfigs() unexpected error = %v", err)
		}

		if len(configs) != 2 {
			t.Errorf("LoadAllConfigs() loaded %d configs, want 2", len(configs))
		}

		if _, exists := configs["app1"]; !exists {
			t.Errorf("LoadAllConfigs() missing app1 config")
		}

		if _, exists := configs["app2"]; !exists {
			t.Errorf("LoadAllConfigs() missing app2 config")
		}
	})

	t.Run("invalid config in directory", func(t *testing.T) {
		err := os.WriteFile(filepath.Join(tempDir, "invalid.yaml"), []byte(invalidYAML), 0644)
		if err != nil {
			t.Fatalf("Failed to write invalid config: %v", err)
		}

		_, err = LoadAllConfigs(tempDir)
		if err == nil {
			t.Errorf("LoadAllConfigs() expected error for invalid config")
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		_, err := LoadAllConfigs("/nonexistent/directory")
		if err == nil {
			t.Errorf("LoadAllConfigs() expected error for nonexistent directory")
		}
	})
}
