package caddy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dockswap/internal/config"
	"dockswap/internal/state"
)

func TestNew(t *testing.T) {
	configPath := "/test/config.json"
	templatePath := "/test/template.json"

	cm := New(configPath, templatePath)

	if cm.ConfigPath != configPath {
		t.Errorf("New() ConfigPath = %v, want %v", cm.ConfigPath, configPath)
	}
	if cm.TemplatePath != templatePath {
		t.Errorf("New() TemplatePath = %v, want %v", cm.TemplatePath, templatePath)
	}
	if cm.AdminURL != DefaultCaddyAdminURL {
		t.Errorf("New() AdminURL = %v, want %v", cm.AdminURL, DefaultCaddyAdminURL)
	}
}

func TestSetAdminURL(t *testing.T) {
	cm := New("/test/config.json", "/test/template.json")
	newURL := "http://localhost:3000"

	cm.SetAdminURL(newURL)

	if cm.AdminURL != newURL {
		t.Errorf("SetAdminURL() AdminURL = %v, want %v", cm.AdminURL, newURL)
	}
}

func TestValidateCaddyRunning(t *testing.T) {
	t.Run("caddy running", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cm := New("/test/config.json", "/test/template.json")
		cm.SetAdminURL(server.URL)

		err := cm.ValidateCaddyRunning()
		if err != nil {
			t.Errorf("ValidateCaddyRunning() should succeed when caddy is running: %v", err)
		}
	})

	t.Run("caddy not running", func(t *testing.T) {
		cm := New("/test/config.json", "/test/template.json")
		cm.SetAdminURL("http://localhost:99999")

		err := cm.ValidateCaddyRunning()
		if err == nil {
			t.Errorf("ValidateCaddyRunning() should fail when caddy is not running")
		}
	})

	t.Run("caddy returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		cm := New("/test/config.json", "/test/template.json")
		cm.SetAdminURL(server.URL)

		err := cm.ValidateCaddyRunning()
		if err == nil {
			t.Errorf("ValidateCaddyRunning() should fail when caddy returns error")
		}
	})
}

func TestGenerateConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	templatePath := filepath.Join(tempDir, "template.json")

	template := `{
  "apps": {
    "http": {
      "servers": {
        {{range .Apps}}
        "{{.Name}}": {
          "listen": [":{{.Proxy.ListenPort}}"],
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
        }{{if not .IsLast}},{{end}}
        {{end}}
      }
    }
  }
}`

	err := os.WriteFile(templatePath, []byte(template), 0644)
	if err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	cm := New(configPath, templatePath)

	configs := map[string]*config.AppConfig{
		"test-app": {
			Name: "test-app",
			Proxy: config.Proxy{
				ListenPort: 80,
				Host:       "test.example.com",
			},
			Ports: config.Ports{
				Blue:  8081,
				Green: 8082,
			},
		},
	}

	states := map[string]*state.AppState{
		"test-app": {
			Name:        "test-app",
			ActiveColor: "blue",
		},
	}

	t.Run("successful generation", func(t *testing.T) {
		err := cm.GenerateConfig(configs, states)
		if err != nil {
			t.Errorf("GenerateConfig() failed: %v", err)
		}

		configContent, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read generated config: %v", err)
		}

		var configJSON map[string]interface{}
		err = json.Unmarshal(configContent, &configJSON)
		if err != nil {
			t.Errorf("Generated config is not valid JSON: %v", err)
		}
	})

	t.Run("missing template", func(t *testing.T) {
		cm := New(configPath, "/nonexistent/template.json")
		err := cm.GenerateConfig(configs, states)
		if err == nil {
			t.Errorf("GenerateConfig() should fail with missing template")
		}
	})

	t.Run("invalid template", func(t *testing.T) {
		invalidTemplatePath := filepath.Join(tempDir, "invalid-template.json")
		err := os.WriteFile(invalidTemplatePath, []byte("{{invalid template"), 0644)
		if err != nil {
			t.Fatalf("Failed to write invalid template: %v", err)
		}

		cm := New(configPath, invalidTemplatePath)
		err = cm.GenerateConfig(configs, states)
		if err == nil {
			t.Errorf("GenerateConfig() should fail with invalid template")
		}
	})

	t.Run("missing state", func(t *testing.T) {
		emptyStates := map[string]*state.AppState{}
		err := cm.GenerateConfig(configs, emptyStates)
		if err == nil {
			t.Errorf("GenerateConfig() should fail when state is missing for app")
		}
	})
}

func TestReloadCaddy(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	validConfig := `{"apps": {"http": {"servers": {}}}}`
	err := os.WriteFile(configPath, []byte(validConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Run("successful reload", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" || r.URL.Path != "/load" {
				t.Errorf("Expected POST /load, got %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cm := New(configPath, "/test/template.json")
		cm.SetAdminURL(server.URL)

		err := cm.ReloadCaddy()
		if err != nil {
			t.Errorf("ReloadCaddy() failed: %v", err)
		}
	})

	t.Run("reload failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Bad configuration"))
		}))
		defer server.Close()

		cm := New(configPath, "/test/template.json")
		cm.SetAdminURL(server.URL)

		err := cm.ReloadCaddy()
		if err == nil {
			t.Errorf("ReloadCaddy() should fail when caddy returns error")
		}
	})

	t.Run("missing config file", func(t *testing.T) {
		cm := New("/nonexistent/config.json", "/test/template.json")
		err := cm.ReloadCaddy()
		if err == nil {
			t.Errorf("ReloadCaddy() should fail with missing config file")
		}
	})
}

func TestBuildTemplateData(t *testing.T) {
	cm := New("/test/config.json", "/test/template.json")

	configs := map[string]*config.AppConfig{
		"app1": {
			Name: "app1",
			Proxy: config.Proxy{
				ListenPort: 80,
				Host:       "app1.example.com",
			},
			Ports: config.Ports{
				Blue:  8081,
				Green: 8082,
			},
		},
		"app2": {
			Name: "app2",
			Proxy: config.Proxy{
				ListenPort: 81,
				Host:       "app2.example.com",
			},
			Ports: config.Ports{
				Blue:  8083,
				Green: 8084,
			},
		},
	}

	states := map[string]*state.AppState{
		"app1": {
			Name:        "app1",
			ActiveColor: "blue",
		},
		"app2": {
			Name:        "app2",
			ActiveColor: "green",
		},
	}

	templateData, err := cm.buildTemplateData(configs, states)
	if err != nil {
		t.Fatalf("buildTemplateData() failed: %v", err)
	}

	if len(templateData.Apps) != 2 {
		t.Errorf("buildTemplateData() returned %d apps, want 2", len(templateData.Apps))
	}

	app1Found := false
	app2Found := false
	lastFound := false

	for _, app := range templateData.Apps {
		if app.Name == "app1" {
			app1Found = true
			if app.ActivePort != 8081 {
				t.Errorf("app1 ActivePort = %d, want 8081", app.ActivePort)
			}
		}
		if app.Name == "app2" {
			app2Found = true
			if app.ActivePort != 8084 {
				t.Errorf("app2 ActivePort = %d, want 8084", app.ActivePort)
			}
		}
		if app.IsLast {
			if lastFound {
				t.Errorf("Multiple apps marked as IsLast")
			}
			lastFound = true
		}
	}

	if !app1Found {
		t.Errorf("app1 not found in template data")
	}
	if !app2Found {
		t.Errorf("app2 not found in template data")
	}
	if !lastFound {
		t.Errorf("No app marked as IsLast")
	}
}

func TestGetActivePort(t *testing.T) {
	cm := New("/test/config.json", "/test/template.json")

	appConfig := &config.AppConfig{
		Ports: config.Ports{
			Blue:  8081,
			Green: 8082,
		},
	}

	t.Run("blue active", func(t *testing.T) {
		appState := &state.AppState{
			ActiveColor: "blue",
		}

		port, err := cm.getActivePort(appConfig, appState)
		if err != nil {
			t.Errorf("getActivePort() failed: %v", err)
		}
		if port != 8081 {
			t.Errorf("getActivePort() = %d, want 8081", port)
		}
	})

	t.Run("green active", func(t *testing.T) {
		appState := &state.AppState{
			ActiveColor: "green",
		}

		port, err := cm.getActivePort(appConfig, appState)
		if err != nil {
			t.Errorf("getActivePort() failed: %v", err)
		}
		if port != 8082 {
			t.Errorf("getActivePort() = %d, want 8082", port)
		}
	})

	t.Run("invalid color", func(t *testing.T) {
		appState := &state.AppState{
			ActiveColor: "red",
		}

		_, err := cm.getActivePort(appConfig, appState)
		if err == nil {
			t.Errorf("getActivePort() should fail with invalid color")
		}
	})
}

func TestHasTemplate(t *testing.T) {
	tempDir := t.TempDir()
	existingTemplate := filepath.Join(tempDir, "existing.json")
	nonExistentTemplate := filepath.Join(tempDir, "nonexistent.json")

	err := os.WriteFile(existingTemplate, []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test template: %v", err)
	}

	t.Run("template exists", func(t *testing.T) {
		cm := New("/test/config.json", existingTemplate)
		if !cm.HasTemplate() {
			t.Errorf("HasTemplate() should return true for existing template")
		}
	})

	t.Run("template does not exist", func(t *testing.T) {
		cm := New("/test/config.json", nonExistentTemplate)
		if cm.HasTemplate() {
			t.Errorf("HasTemplate() should return false for nonexistent template")
		}
	})
}

func TestCreateDefaultTemplate(t *testing.T) {
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "subdir", "template.json")
	configPath := filepath.Join(tempDir, "config.json")

	cm := New(configPath, templatePath)

	err := cm.CreateDefaultTemplate()
	if err != nil {
		t.Errorf("CreateDefaultTemplate() failed: %v", err)
	}

	if !cm.HasTemplate() {
		t.Errorf("CreateDefaultTemplate() should create template file")
	}

	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("Failed to read created template: %v", err)
	}

	if len(content) == 0 {
		t.Errorf("CreateDefaultTemplate() created empty template")
	}

	// Template contains Go template syntax, so we test if it can be used to generate valid JSON
	testConfigs := map[string]*config.AppConfig{
		"test-app": {
			Name: "test-app",
			Proxy: config.Proxy{
				ListenPort: 80,
				Host:       "test.example.com",
			},
			Ports: config.Ports{
				Blue:  8081,
				Green: 8082,
			},
		},
	}

	testStates := map[string]*state.AppState{
		"test-app": {
			Name:        "test-app",
			ActiveColor: "blue",
		},
	}

	err = cm.GenerateConfig(testConfigs, testStates)
	if err != nil {
		t.Errorf("CreateDefaultTemplate() template cannot generate valid config: %v", err)
	}
}
