package config

import (
	"os"
	"path/filepath"
	"testing"
)

const validYAML = `
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
  timeout: 5s
  retries: 3
  success_threshold: 1
  expected_status: 200
`

const invalidYAML = `name: [unclosed`

func TestValidateAndPrepareConfigDir_AllFoldersCreated(t *testing.T) {
	dir := t.TempDir()
	apps := filepath.Join(dir, "apps")
	os.MkdirAll(apps, 0755)
	os.WriteFile(filepath.Join(apps, "app1.yaml"), []byte(validYAML), 0644)

	err := ValidateAndPrepareConfigDir(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	for _, sub := range []string{"apps", "state", "caddy"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); err != nil {
			t.Errorf("expected %s to exist", sub)
		}
	}
}

func TestValidateAndPrepareConfigDir_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	apps := filepath.Join(dir, "apps")
	os.MkdirAll(apps, 0755)
	os.WriteFile(filepath.Join(apps, "bad.yaml"), []byte(invalidYAML), 0644)

	err := ValidateAndPrepareConfigDir(dir)
	if err == nil || err.Error() == "" {
		t.Fatalf("expected error for invalid YAML, got %v", err)
	}
}

func TestValidateAndPrepareConfigDir_MissingRequiredFields(t *testing.T) {
	dir := t.TempDir()
	apps := filepath.Join(dir, "apps")
	os.MkdirAll(apps, 0755)
	os.WriteFile(filepath.Join(apps, "bad.yaml"), []byte(`name: ""`), 0644)

	err := ValidateAndPrepareConfigDir(dir)
	if err == nil || err.Error() == "" {
		t.Fatalf("expected error for missing fields, got %v", err)
	}
}

func TestValidateAndPrepareConfigDir_NoValidConfigs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "apps"), 0755)
	// No YAML files
	err := ValidateAndPrepareConfigDir(dir)
	if err != nil {
		t.Fatalf("expected no error for empty apps/, got %v", err)
	}
}
