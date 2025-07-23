package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dockswap/internal/state"
)

func TestGetSearchPaths(t *testing.T) {
	paths := getSearchPaths()

	if len(paths) < 2 {
		t.Errorf("getSearchPaths() should return at least 2 paths, got %d", len(paths))
	}

	expectedSuffix := "/etc/dockswap-cfg"
	if paths[len(paths)-1] != expectedSuffix {
		t.Errorf("getSearchPaths() last path should be %s, got %s", expectedSuffix, paths[len(paths)-1])
	}
}

func TestDirExists(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("existing directory", func(t *testing.T) {
		exists, err := dirExists(tempDir)
		if err != nil {
			t.Errorf("dirExists() unexpected error: %v", err)
		}
		if !exists {
			t.Errorf("dirExists() should return true for existing directory")
		}
	})

	t.Run("non-existing directory", func(t *testing.T) {
		exists, err := dirExists(filepath.Join(tempDir, "nonexistent"))
		if err != nil {
			t.Errorf("dirExists() unexpected error: %v", err)
		}
		if exists {
			t.Errorf("dirExists() should return false for non-existing directory")
		}
	})

	t.Run("file instead of directory", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "testfile")
		err := os.WriteFile(testFile, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		exists, err := dirExists(testFile)
		if err != nil {
			t.Errorf("dirExists() unexpected error: %v", err)
		}
		if exists {
			t.Errorf("dirExists() should return false for file")
		}
	})
}

func TestFileExists(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "testfile")

	t.Run("non-existing file", func(t *testing.T) {
		exists, err := fileExists(testFile)
		if err != nil {
			t.Errorf("fileExists() unexpected error: %v", err)
		}
		if exists {
			t.Errorf("fileExists() should return false for non-existing file")
		}
	})

	t.Run("existing file", func(t *testing.T) {
		err := os.WriteFile(testFile, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		exists, err := fileExists(testFile)
		if err != nil {
			t.Errorf("fileExists() unexpected error: %v", err)
		}
		if !exists {
			t.Errorf("fileExists() should return true for existing file")
		}
	})

	t.Run("directory instead of file", func(t *testing.T) {
		exists, err := fileExists(tempDir)
		if err != nil {
			t.Errorf("fileExists() unexpected error: %v", err)
		}
		if exists {
			t.Errorf("fileExists() should return false for directory")
		}
	})
}

func TestInitializeWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	workspaceRoot := filepath.Join(tempDir, "test-workspace")

	workspace, err := InitializeWorkspace(workspaceRoot)
	if err != nil {
		t.Fatalf("InitializeWorkspace() failed: %v", err)
	}
	defer workspace.Close()

	t.Run("directory structure created", func(t *testing.T) {
		dirs := []string{workspace.Root, workspace.AppsDir, workspace.StateDir}
		for _, dir := range dirs {
			if exists, err := dirExists(dir); err != nil {
				t.Errorf("Error checking directory %s: %v", dir, err)
			} else if !exists {
				t.Errorf("Directory %s was not created", dir)
			}
		}
	})

	t.Run("database file created", func(t *testing.T) {
		if exists, err := fileExists(workspace.DBPath); err != nil {
			t.Errorf("Error checking database file: %v", err)
		} else if !exists {
			t.Errorf("Database file was not created")
		}
	})

	t.Run("database connection works", func(t *testing.T) {
		if workspace.DB == nil {
			t.Errorf("Database connection is nil")
		} else if err := workspace.DB.Ping(); err != nil {
			t.Errorf("Database ping failed: %v", err)
		}
	})

	t.Run("database schema created", func(t *testing.T) {
		var count int
		err := workspace.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('deployments', 'app_configs')").Scan(&count)
		if err != nil {
			t.Errorf("Failed to query database schema: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected 2 tables, found %d", count)
		}
	})
}

func TestValidateStructure(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("valid workspace structure", func(t *testing.T) {
		workspace, err := InitializeWorkspace(filepath.Join(tempDir, "valid"))
		if err != nil {
			t.Fatalf("InitializeWorkspace() failed: %v", err)
		}
		defer workspace.Close()

		err = workspace.ValidateStructure()
		if err != nil {
			t.Errorf("ValidateStructure() failed for valid workspace: %v", err)
		}
	})

	t.Run("missing apps directory", func(t *testing.T) {
		workspaceRoot := filepath.Join(tempDir, "missing-apps")
		workspace, err := InitializeWorkspace(workspaceRoot)
		if err != nil {
			t.Fatalf("InitializeWorkspace() failed: %v", err)
		}
		defer workspace.Close()

		err = os.RemoveAll(workspace.AppsDir)
		if err != nil {
			t.Fatalf("Failed to remove apps directory: %v", err)
		}

		err = workspace.ValidateStructure()
		if err == nil {
			t.Errorf("ValidateStructure() should fail when apps directory is missing")
		}
	})

	t.Run("missing database file", func(t *testing.T) {
		workspaceRoot := filepath.Join(tempDir, "missing-db")
		workspace, err := InitializeWorkspace(workspaceRoot)
		if err != nil {
			t.Fatalf("InitializeWorkspace() failed: %v", err)
		}
		defer workspace.Close()

		err = os.Remove(workspace.DBPath)
		if err != nil {
			t.Fatalf("Failed to remove database file: %v", err)
		}

		err = workspace.ValidateStructure()
		if err == nil {
			t.Errorf("ValidateStructure() should fail when database file is missing")
		}
	})
}

func TestLoadWorkspace(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("load valid workspace", func(t *testing.T) {
		workspaceRoot := filepath.Join(tempDir, "valid-load")

		originalWorkspace, err := InitializeWorkspace(workspaceRoot)
		if err != nil {
			t.Fatalf("InitializeWorkspace() failed: %v", err)
		}
		originalWorkspace.Close()

		workspace, err := LoadWorkspace(workspaceRoot)
		if err != nil {
			t.Fatalf("LoadWorkspace() failed: %v", err)
		}
		defer workspace.Close()

		if workspace.Root != workspaceRoot {
			t.Errorf("LoadWorkspace() root = %v, want %v", workspace.Root, workspaceRoot)
		}

		if workspace.DB == nil {
			t.Errorf("LoadWorkspace() database connection is nil")
		}
	})

	t.Run("load invalid workspace", func(t *testing.T) {
		invalidRoot := filepath.Join(tempDir, "nonexistent")

		_, err := LoadWorkspace(invalidRoot)
		if err == nil {
			t.Errorf("LoadWorkspace() should fail for invalid workspace")
		}
	})
}

func TestWorkspaceWithConfigs(t *testing.T) {
	tempDir := t.TempDir()
	workspaceRoot := filepath.Join(tempDir, "config-test")

	workspace, err := InitializeWorkspace(workspaceRoot)
	if err != nil {
		t.Fatalf("InitializeWorkspace() failed: %v", err)
	}
	defer workspace.Close()

	appConfigYAML := `name: test-app
description: "Test application"
docker:
  expose_port: 8080
  environment:
    DATABASE_URL: "postgres://localhost/test"
ports:
  blue: 8081
  green: 8082
health_check:
  endpoint: "/health"
  retries: 3
  success_threshold: 2
  expected_status: 200
deployment:
  drain_timeout: "30s"
  auto_rollback: true
proxy:
  listen_port: 80
  host: "test.example.com"`

	configFile := filepath.Join(workspace.AppsDir, "test-app.yaml")
	err = os.WriteFile(configFile, []byte(appConfigYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	appStateYAML := `name: "test-app"
current_image: "nginx:1.21"
desired_image: "nginx:1.21"
active_color: "blue"
status: "stable"
last_deployment: "2025-07-23T10:30:00Z"
last_updated: "2025-07-23T10:35:00Z"`

	stateFile := filepath.Join(workspace.StateDir, "test-app.yaml")
	err = os.WriteFile(stateFile, []byte(appStateYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write state file: %v", err)
	}

	err = workspace.RefreshWorkspace()
	if err != nil {
		t.Fatalf("RefreshWorkspace() failed: %v", err)
	}

	t.Run("config loaded correctly", func(t *testing.T) {
		cfg, exists := workspace.GetConfig("test-app")
		if !exists {
			t.Errorf("GetConfig() config not found")
		}
		if cfg.Name != "test-app" {
			t.Errorf("GetConfig() name = %v, want %v", cfg.Name, "test-app")
		}
	})

	t.Run("state loaded correctly", func(t *testing.T) {
		st, exists := workspace.GetState("test-app")
		if !exists {
			t.Errorf("GetState() state not found")
		}
		if st.Name != "test-app" {
			t.Errorf("GetState() name = %v, want %v", st.Name, "test-app")
		}
	})

	t.Run("list apps", func(t *testing.T) {
		apps := workspace.ListApps()
		if len(apps) != 1 {
			t.Errorf("ListApps() returned %d apps, want 1", len(apps))
		}
		if apps[0] != "test-app" {
			t.Errorf("ListApps() = %v, want ['test-app']", apps)
		}
	})

	t.Run("save state", func(t *testing.T) {
		now := time.Now().UTC()
		newState := &state.AppState{
			Name:           "test-app",
			CurrentImage:   "nginx:1.22",
			DesiredImage:   "nginx:1.22",
			ActiveColor:    "green",
			Status:         "stable",
			LastDeployment: now,
			LastUpdated:    now,
		}

		err := workspace.SaveState("test-app", newState)
		if err != nil {
			t.Errorf("SaveState() failed: %v", err)
		}

		savedState, exists := workspace.GetState("test-app")
		if !exists {
			t.Errorf("SaveState() state not found after save")
		}
		if savedState.CurrentImage != "nginx:1.22" {
			t.Errorf("SaveState() current_image = %v, want %v", savedState.CurrentImage, "nginx:1.22")
		}
	})
}

func TestValidateConfigs(t *testing.T) {
	tempDir := t.TempDir()
	workspaceRoot := filepath.Join(tempDir, "validate-test")

	workspace, err := InitializeWorkspace(workspaceRoot)
	if err != nil {
		t.Fatalf("InitializeWorkspace() failed: %v", err)
	}
	defer workspace.Close()

	t.Run("port conflicts", func(t *testing.T) {
		app1YAML := `name: app1
docker:
  expose_port: 8080
ports:
  blue: 8081
  green: 8082
health_check:
  retries: 3
  success_threshold: 2
  expected_status: 200
proxy:
  listen_port: 80`

		app2YAML := `name: app2
docker:
  expose_port: 8080
ports:
  blue: 8083
  green: 8084
health_check:
  retries: 3
  success_threshold: 2
  expected_status: 200
proxy:
  listen_port: 81`

		err = os.WriteFile(filepath.Join(workspace.AppsDir, "app1.yaml"), []byte(app1YAML), 0644)
		if err != nil {
			t.Fatalf("Failed to write app1 config: %v", err)
		}

		err = os.WriteFile(filepath.Join(workspace.AppsDir, "app2.yaml"), []byte(app2YAML), 0644)
		if err != nil {
			t.Fatalf("Failed to write app2 config: %v", err)
		}

		err = workspace.RefreshWorkspace()
		if err == nil {
			t.Errorf("RefreshWorkspace() should fail due to port conflicts")
		}
	})
}

func TestDiscoverWorkspace(t *testing.T) {
	tempDir := t.TempDir()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalWd)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	t.Run("no workspace found", func(t *testing.T) {
		_, err := DiscoverWorkspace()
		if err == nil {
			t.Errorf("DiscoverWorkspace() should fail when no workspace exists")
		}
	})

	t.Run("workspace found in current directory", func(t *testing.T) {
		workspaceRoot := filepath.Join(tempDir, "dockswap-cfg")
		_, err := InitializeWorkspace(workspaceRoot)
		if err != nil {
			t.Fatalf("InitializeWorkspace() failed: %v", err)
		}

		workspace, err := DiscoverWorkspace()
		if err != nil {
			t.Errorf("DiscoverWorkspace() failed: %v", err)
		}
		if workspace != nil {
			defer workspace.Close()

			expectedRoot, err := filepath.EvalSymlinks(workspaceRoot)
			if err != nil {
				expectedRoot = workspaceRoot
			}
			actualRoot, err := filepath.EvalSymlinks(workspace.Root)
			if err != nil {
				actualRoot = workspace.Root
			}

			if actualRoot != expectedRoot {
				t.Errorf("DiscoverWorkspace() root = %v, want %v", actualRoot, expectedRoot)
			}
		}
	})
}
