package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ValidateAndPrepareConfigDir ensures required folders exist and all app configs are valid.
func ValidateAndPrepareConfigDir(configDir string) error {
	required := []string{"apps", "state", "caddy"}
	for _, sub := range required {
		dir := filepath.Join(configDir, sub)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create/check %s: %w", dir, err)
		}
	}

	appsDir := filepath.Join(configDir, "apps")
	files, err := os.ReadDir(appsDir)
	if err != nil {
		return fmt.Errorf("failed to read apps dir: %w", err)
	}
	hasYAML := false
	for _, f := range files {
		if !f.IsDir() && (filepath.Ext(f.Name()) == ".yaml" || filepath.Ext(f.Name()) == ".yml") {
			hasYAML = true
			break
		}
	}
	if hasYAML {
		configs, err := LoadAllConfigs(appsDir)
		if err != nil {
			return fmt.Errorf("app config validation failed: %w", err)
		}
		_ = configs // not used, just validate
	}

	return nil
}
