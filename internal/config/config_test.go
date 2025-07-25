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

func TestGetEnvironmentForColor(t *testing.T) {
	docker := Docker{
		Environment: map[string]string{
			"DATABASE_URL": "postgres://localhost/test",
			"LOG_LEVEL":    "info",
			"PORT":         "4000",
		},
		EnvironmentOverrides: map[string]map[string]string{
			"blue": {
				"PORT":      "8081",
				"LOG_LEVEL": "debug",
				"NEW_VAR":   "blue_value",
			},
			"green": {
				"PORT":    "8082",
				"NEW_VAR": "green_value",
			},
		},
	}

	tests := []struct {
		name     string
		color    string
		expected map[string]string
	}{
		{
			name:  "blue environment with overrides",
			color: "blue",
			expected: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
				"LOG_LEVEL":    "debug",      // overridden
				"PORT":         "8081",       // overridden
				"NEW_VAR":      "blue_value", // added
			},
		},
		{
			name:  "green environment with overrides",
			color: "green",
			expected: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
				"LOG_LEVEL":    "info",        // not overridden
				"PORT":         "8082",        // overridden
				"NEW_VAR":      "green_value", // added
			},
		},
		{
			name:  "non-existent color returns base environment",
			color: "purple",
			expected: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
				"LOG_LEVEL":    "info",
				"PORT":         "4000",
			},
		},
		{
			name:  "empty color returns base environment",
			color: "",
			expected: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
				"LOG_LEVEL":    "info",
				"PORT":         "4000",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := docker.GetEnvironmentForColor(tt.color)

			if len(result) != len(tt.expected) {
				t.Errorf("GetEnvironmentForColor() returned %d variables, expected %d", len(result), len(tt.expected))
			}

			for key, expectedValue := range tt.expected {
				if actualValue, exists := result[key]; !exists {
					t.Errorf("GetEnvironmentForColor() missing key %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("GetEnvironmentForColor() key %s = %v, expected %v", key, actualValue, expectedValue)
				}
			}
		})
	}
}

func TestGetEnvironmentForColorEmptyOverrides(t *testing.T) {
	docker := Docker{
		Environment: map[string]string{
			"DATABASE_URL": "postgres://localhost/test",
			"PORT":         "4000",
		},
		EnvironmentOverrides: map[string]map[string]string{
			"blue":  {}, // empty override
			"green": {}, // empty override
		},
	}

	result := docker.GetEnvironmentForColor("blue")
	expected := map[string]string{
		"DATABASE_URL": "postgres://localhost/test",
		"PORT":         "4000",
	}

	if len(result) != len(expected) {
		t.Errorf("GetEnvironmentForColor() with empty overrides returned %d variables, expected %d", len(result), len(expected))
	}

	for key, expectedValue := range expected {
		if actualValue := result[key]; actualValue != expectedValue {
			t.Errorf("GetEnvironmentForColor() key %s = %v, expected %v", key, actualValue, expectedValue)
		}
	}
}

func TestValidateConfigEnvironmentOverrides(t *testing.T) {
	tests := []struct {
		name    string
		config  AppConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid environment overrides",
			config: AppConfig{
				Name: "test-app",
				Docker: Docker{
					ExposePort: 8080,
					EnvironmentOverrides: map[string]map[string]string{
						"blue": {
							"PORT": "8081",
						},
						"green": {
							"PORT": "8082",
						},
					},
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
			name: "invalid color in environment overrides",
			config: AppConfig{
				Name: "test-app",
				Docker: Docker{
					ExposePort: 8080,
					EnvironmentOverrides: map[string]map[string]string{
						"blue": {
							"PORT": "8081",
						},
						"purple": { // invalid color
							"PORT": "8083",
						},
					},
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
			errMsg:  "environment_overrides: only 'blue' and 'green' colors are supported, got 'purple'",
		},
		{
			name: "empty environment overrides",
			config: AppConfig{
				Name: "test-app",
				Docker: Docker{
					ExposePort:           8080,
					EnvironmentOverrides: map[string]map[string]string{},
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
			name: "nil environment overrides",
			config: AppConfig{
				Name: "test-app",
				Docker: Docker{
					ExposePort:           8080,
					EnvironmentOverrides: nil,
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

func TestLoadAppConfigWithEnvironmentOverrides(t *testing.T) {
	tempDir := t.TempDir()

	yamlWithOverrides := `name: test-app
description: "Test application with environment overrides"
docker:
  restart_policy: "unless-stopped"
  memory_limit: "512m"
  cpu_limit: "0.5"
  environment:
    DATABASE_URL: "postgres://localhost/test"
    LOG_LEVEL: "info"
    PORT: "4000"
  environment_overrides:
    blue:
      PORT: "8081"
      LOG_LEVEL: "debug"
      BLUE_SPECIFIC: "blue_value"
    green:
      PORT: "8082"
      GREEN_SPECIFIC: "green_value"
  expose_port: 8080
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
  listen_port: 80`

	configFile := filepath.Join(tempDir, "test-overrides.yaml")
	err := os.WriteFile(configFile, []byte(yamlWithOverrides), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	config, err := LoadAppConfig(configFile)
	if err != nil {
		t.Fatalf("LoadAppConfig() unexpected error = %v", err)
	}

	if config == nil {
		t.Fatal("LoadAppConfig() returned nil config")
	}

	// Test base environment
	expectedBaseEnv := map[string]string{
		"DATABASE_URL": "postgres://localhost/test",
		"LOG_LEVEL":    "info",
		"PORT":         "4000",
	}

	for key, expected := range expectedBaseEnv {
		if actual := config.Docker.Environment[key]; actual != expected {
			t.Errorf("Base environment %s = %v, expected %v", key, actual, expected)
		}
	}

	// Test blue environment
	blueEnv := config.Docker.GetEnvironmentForColor("blue")
	expectedBlueEnv := map[string]string{
		"DATABASE_URL":  "postgres://localhost/test",
		"LOG_LEVEL":     "debug",      // overridden
		"PORT":          "8081",       // overridden
		"BLUE_SPECIFIC": "blue_value", // added
	}

	for key, expected := range expectedBlueEnv {
		if actual := blueEnv[key]; actual != expected {
			t.Errorf("Blue environment %s = %v, expected %v", key, actual, expected)
		}
	}

	// Test green environment
	greenEnv := config.Docker.GetEnvironmentForColor("green")
	expectedGreenEnv := map[string]string{
		"DATABASE_URL":   "postgres://localhost/test",
		"LOG_LEVEL":      "info",        // not overridden
		"PORT":           "8082",        // overridden
		"GREEN_SPECIFIC": "green_value", // added
	}

	for key, expected := range expectedGreenEnv {
		if actual := greenEnv[key]; actual != expected {
			t.Errorf("Green environment %s = %v, expected %v", key, actual, expected)
		}
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
