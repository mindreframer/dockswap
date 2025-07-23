package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"dockswap/internal/config"
	"dockswap/internal/state"
)

type CaddyManager struct {
	AdminURL     string
	ConfigPath   string
	TemplatePath string
	client       *http.Client
}

type AppTemplateData struct {
	Name       string
	Proxy      config.Proxy
	ActivePort int
	IsLast     bool
}

type TemplateData struct {
	Apps []AppTemplateData
}

const DefaultCaddyAdminURL = "http://localhost:2019"

func New(configPath, templatePath string) *CaddyManager {
	return &CaddyManager{
		AdminURL:     DefaultCaddyAdminURL,
		ConfigPath:   configPath,
		TemplatePath: templatePath,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (cm *CaddyManager) SetAdminURL(url string) {
	cm.AdminURL = url
}

func (cm *CaddyManager) ValidateCaddyRunning() error {
	resp, err := cm.client.Get(cm.AdminURL + "/")
	if err != nil {
		return fmt.Errorf("caddy admin API not accessible at %s: %w", cm.AdminURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("caddy admin API returned status %d", resp.StatusCode)
	}

	return nil
}

func (cm *CaddyManager) GenerateConfig(configs map[string]*config.AppConfig, states map[string]*state.AppState) error {
	templateContent, err := os.ReadFile(cm.TemplatePath)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %w", cm.TemplatePath, err)
	}

	tmpl, err := template.New("caddy").Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	templateData, err := cm.buildTemplateData(configs, states)
	if err != nil {
		return fmt.Errorf("failed to build template data: %w", err)
	}

	var configBuffer bytes.Buffer
	if err := tmpl.Execute(&configBuffer, templateData); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := cm.validateGeneratedConfig(configBuffer.Bytes()); err != nil {
		return fmt.Errorf("generated config is invalid JSON: %w", err)
	}

	configDir := filepath.Dir(cm.ConfigPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", configDir, err)
	}

	if err := os.WriteFile(cm.ConfigPath, configBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", cm.ConfigPath, err)
	}

	return nil
}

func (cm *CaddyManager) ReloadCaddy() error {
	configContent, err := os.ReadFile(cm.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", cm.ConfigPath, err)
	}

	req, err := http.NewRequest("POST", cm.AdminURL+"/load", bytes.NewReader(configContent))
	if err != nil {
		return fmt.Errorf("failed to create reload request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := cm.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send reload request to caddy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy reload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (cm *CaddyManager) UpdateAppRouting(appName string, configs map[string]*config.AppConfig, states map[string]*state.AppState) error {
	if err := cm.GenerateConfig(configs, states); err != nil {
		return fmt.Errorf("failed to generate config for app %s: %w", appName, err)
	}

	if err := cm.ReloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload caddy for app %s: %w", appName, err)
	}

	return nil
}

func (cm *CaddyManager) buildTemplateData(configs map[string]*config.AppConfig, states map[string]*state.AppState) (*TemplateData, error) {
	var apps []AppTemplateData

	for appName, appConfig := range configs {
		appState, exists := states[appName]
		if !exists {
			return nil, fmt.Errorf("no state found for app %s", appName)
		}

		activePort, err := cm.getActivePort(appConfig, appState)
		if err != nil {
			return nil, fmt.Errorf("failed to determine active port for app %s: %w", appName, err)
		}

		apps = append(apps, AppTemplateData{
			Name:       appName,
			Proxy:      appConfig.Proxy,
			ActivePort: activePort,
			IsLast:     false, // Will be set correctly below
		})
	}

	if len(apps) > 0 {
		apps[len(apps)-1].IsLast = true
	}

	return &TemplateData{Apps: apps}, nil
}

func (cm *CaddyManager) getActivePort(appConfig *config.AppConfig, appState *state.AppState) (int, error) {
	switch appState.ActiveColor {
	case "blue":
		return appConfig.Ports.Blue, nil
	case "green":
		return appConfig.Ports.Green, nil
	default:
		return 0, fmt.Errorf("invalid active color: %s", appState.ActiveColor)
	}
}

func (cm *CaddyManager) validateGeneratedConfig(configJSON []byte) error {
	var config map[string]interface{}
	return json.Unmarshal(configJSON, &config)
}

func (cm *CaddyManager) GetConfigPath() string {
	return cm.ConfigPath
}

func (cm *CaddyManager) GetTemplatePath() string {
	return cm.TemplatePath
}

func (cm *CaddyManager) HasTemplate() bool {
	_, err := os.Stat(cm.TemplatePath)
	return err == nil
}

func (cm *CaddyManager) CreateDefaultTemplate() error {
	defaultTemplate := `{
  "apps": {
    "http": {
      "servers": {
        {{range .Apps}}
        "{{.Name}}": {
          "listen": [":{{.Proxy.ListenPort}}"],
          {{if .Proxy.Host}}
          "routes": [
            {
              "match": [
                {
                  "host": ["{{.Proxy.Host}}"]
                }
              ],
              "handle": [
                {
                  "handler": "reverse_proxy",
                  "upstreams": [
                    {
                      "dial": "localhost:{{.ActivePort}}"
                    }
                  ]
                }
              ]
            }
          ]
          {{else}}
          "routes": [
            {
              "handle": [
                {
                  "handler": "reverse_proxy",
                  "upstreams": [
                    {
                      "dial": "localhost:{{.ActivePort}}"
                    }
                  ]
                }
              ]
            }
          ]
          {{end}}
        }{{if not .IsLast}},{{end}}
        {{end}}
      }
    }
  }
}`

	templateDir := filepath.Dir(cm.TemplatePath)
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return fmt.Errorf("failed to create template directory %s: %w", templateDir, err)
	}

	if err := os.WriteFile(cm.TemplatePath, []byte(defaultTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write default template %s: %w", cm.TemplatePath, err)
	}

	return nil
}
