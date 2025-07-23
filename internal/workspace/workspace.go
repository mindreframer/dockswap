package workspace

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"dockswap/internal/caddy"
	"dockswap/internal/config"
	"dockswap/internal/state"

	_ "github.com/mattn/go-sqlite3"
)

type Workspace struct {
	Root     string                       `json:"root"`
	AppsDir  string                       `json:"apps_dir"`
	StateDir string                       `json:"state_dir"`
	CaddyDir string                       `json:"caddy_dir"`
	DBPath   string                       `json:"db_path"`
	DB       *sql.DB                      `json:"-"`
	Configs  map[string]*config.AppConfig `json:"-"`
	States   map[string]*state.AppState   `json:"-"`
	CaddyMgr *caddy.CaddyManager          `json:"-"`
}

const (
	AppsSubdir  = "apps"
	StateSubdir = "state"
	CaddySubdir = "caddy"
	DBFilename  = "dockswap.db"
)

func DiscoverWorkspace() (*Workspace, error) {
	searchPaths := getSearchPaths()

	for _, path := range searchPaths {
		if exists, err := dirExists(path); err != nil {
			continue
		} else if exists {
			workspace, err := LoadWorkspace(path)
			if err != nil {
				continue
			}
			return workspace, nil
		}
	}

	return nil, fmt.Errorf("no valid dockswap workspace found in search paths: %v", searchPaths)
}

func InitializeWorkspace(rootPath string) (*Workspace, error) {
	workspace := &Workspace{
		Root:     rootPath,
		AppsDir:  filepath.Join(rootPath, AppsSubdir),
		StateDir: filepath.Join(rootPath, StateSubdir),
		CaddyDir: filepath.Join(rootPath, CaddySubdir),
		DBPath:   filepath.Join(rootPath, DBFilename),
		Configs:  make(map[string]*config.AppConfig),
		States:   make(map[string]*state.AppState),
	}

	if err := workspace.createDirectoryStructure(); err != nil {
		return nil, fmt.Errorf("failed to create directory structure: %w", err)
	}

	if err := workspace.initializeDatabase(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := workspace.initializeCaddy(); err != nil {
		return nil, fmt.Errorf("failed to initialize caddy: %w", err)
	}

	return workspace, nil
}

func LoadWorkspace(rootPath string) (*Workspace, error) {
	workspace := &Workspace{
		Root:     rootPath,
		AppsDir:  filepath.Join(rootPath, AppsSubdir),
		StateDir: filepath.Join(rootPath, StateSubdir),
		CaddyDir: filepath.Join(rootPath, CaddySubdir),
		DBPath:   filepath.Join(rootPath, DBFilename),
		Configs:  make(map[string]*config.AppConfig),
		States:   make(map[string]*state.AppState),
	}

	if err := workspace.ValidateStructure(); err != nil {
		return nil, fmt.Errorf("workspace structure validation failed: %w", err)
	}

	if err := workspace.openDatabase(); err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := workspace.initializeCaddy(); err != nil {
		return nil, fmt.Errorf("failed to initialize caddy: %w", err)
	}

	if err := workspace.RefreshWorkspace(); err != nil {
		return nil, fmt.Errorf("failed to load workspace data: %w", err)
	}

	return workspace, nil
}

func (w *Workspace) ValidateStructure() error {
	if exists, err := dirExists(w.Root); err != nil {
		return fmt.Errorf("cannot access root directory %s: %w", w.Root, err)
	} else if !exists {
		return fmt.Errorf("root directory does not exist: %s", w.Root)
	}

	if exists, err := dirExists(w.AppsDir); err != nil {
		return fmt.Errorf("cannot access apps directory %s: %w", w.AppsDir, err)
	} else if !exists {
		return fmt.Errorf("apps directory does not exist: %s", w.AppsDir)
	}

	if exists, err := dirExists(w.StateDir); err != nil {
		return fmt.Errorf("cannot access state directory %s: %w", w.StateDir, err)
	} else if !exists {
		return fmt.Errorf("state directory does not exist: %s", w.StateDir)
	}

	if exists, err := fileExists(w.DBPath); err != nil {
		return fmt.Errorf("cannot access database file %s: %w", w.DBPath, err)
	} else if !exists {
		return fmt.Errorf("database file does not exist: %s", w.DBPath)
	}

	return nil
}

func (w *Workspace) RefreshWorkspace() error {
	configs, err := config.LoadAllConfigs(w.AppsDir)
	if err != nil {
		return fmt.Errorf("failed to load app configs: %w", err)
	}
	w.Configs = configs

	states, err := state.LoadAllStates(w.StateDir)
	if err != nil {
		return fmt.Errorf("failed to load app states: %w", err)
	}
	w.States = states

	if err := w.ValidateConfigs(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	return nil
}

func (w *Workspace) ValidateConfigs() error {
	for appName, appConfig := range w.Configs {
		if appName != appConfig.Name {
			return fmt.Errorf("config file name mismatch: file suggests '%s' but config.name is '%s'", appName, appConfig.Name)
		}

		if appState, exists := w.States[appName]; exists {
			if appState.Name != appConfig.Name {
				return fmt.Errorf("state/config name mismatch for app '%s'", appName)
			}
		}

		if err := w.validatePortConflicts(appName, appConfig); err != nil {
			return err
		}
	}

	return nil
}

func (w *Workspace) GetConfig(appName string) (*config.AppConfig, bool) {
	cfg, exists := w.Configs[appName]
	return cfg, exists
}

func (w *Workspace) GetState(appName string) (*state.AppState, bool) {
	st, exists := w.States[appName]
	return st, exists
}

func (w *Workspace) SaveState(appName string, appState *state.AppState) error {
	statePath := filepath.Join(w.StateDir, appName+".yaml")
	if err := state.SaveAppState(statePath, appState); err != nil {
		return fmt.Errorf("failed to save state for app '%s': %w", appName, err)
	}

	w.States[appName] = appState
	return nil
}

func (w *Workspace) ListApps() []string {
	var apps []string
	for appName := range w.Configs {
		apps = append(apps, appName)
	}
	return apps
}

func (w *Workspace) Close() error {
	if w.DB != nil {
		return w.DB.Close()
	}
	return nil
}

func (w *Workspace) createDirectoryStructure() error {
	dirs := []string{w.Root, w.AppsDir, w.StateDir, w.CaddyDir}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (w *Workspace) initializeDatabase() error {
	db, err := sql.Open("sqlite3", w.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	w.DB = db

	if err := w.createDatabaseSchema(); err != nil {
		return fmt.Errorf("failed to create database schema: %w", err)
	}

	return nil
}

func (w *Workspace) openDatabase() error {
	db, err := sql.Open("sqlite3", w.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	w.DB = db

	if err := db.Ping(); err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	return nil
}

func (w *Workspace) createDatabaseSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS deployments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_name TEXT NOT NULL,
		image TEXT NOT NULL,
		color TEXT NOT NULL,
		status TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		completed_at DATETIME,
		error_message TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_deployments_app_name ON deployments(app_name);
	CREATE INDEX IF NOT EXISTS idx_deployments_started_at ON deployments(started_at);

	CREATE TABLE IF NOT EXISTS app_configs (
		app_name TEXT PRIMARY KEY,
		config_hash TEXT NOT NULL,
		last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := w.DB.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}

func (w *Workspace) validatePortConflicts(appName string, appConfig *config.AppConfig) error {
	usedPorts := make(map[int]string)

	checkPort := func(port int, portType string) error {
		if existingApp, exists := usedPorts[port]; exists {
			return fmt.Errorf("port conflict: app '%s' %s port %d conflicts with app '%s'", appName, portType, port, existingApp)
		}
		usedPorts[port] = appName
		return nil
	}

	if err := checkPort(appConfig.Docker.ExposePort, "expose"); err != nil {
		return err
	}
	if err := checkPort(appConfig.Ports.Blue, "blue"); err != nil {
		return err
	}
	if err := checkPort(appConfig.Ports.Green, "green"); err != nil {
		return err
	}
	if err := checkPort(appConfig.Proxy.ListenPort, "proxy"); err != nil {
		return err
	}

	for otherAppName, otherConfig := range w.Configs {
		if otherAppName == appName {
			continue
		}

		otherPorts := []struct {
			port     int
			portType string
		}{
			{otherConfig.Docker.ExposePort, "expose"},
			{otherConfig.Ports.Blue, "blue"},
			{otherConfig.Ports.Green, "green"},
			{otherConfig.Proxy.ListenPort, "proxy"},
		}

		for _, otherPort := range otherPorts {
			if err := checkPort(otherPort.port, otherPort.portType); err != nil {
				return err
			}
		}
	}

	return nil
}

func getSearchPaths() []string {
	var paths []string

	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, "dockswap-cfg"))
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".dockswap-cfg"))
	}

	paths = append(paths, "/etc/dockswap-cfg")

	return paths
}

func dirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}

func (w *Workspace) initializeCaddy() error {
	configPath := filepath.Join(w.CaddyDir, "config.json")
	templatePath := filepath.Join(w.CaddyDir, "template.json")

	w.CaddyMgr = caddy.New(configPath, templatePath)

	if !w.CaddyMgr.HasTemplate() {
		if err := w.CaddyMgr.CreateDefaultTemplate(); err != nil {
			return fmt.Errorf("failed to create default caddy template: %w", err)
		}
	}

	return nil
}

func (w *Workspace) UpdateCaddyConfig() error {
	if w.CaddyMgr == nil {
		return fmt.Errorf("caddy manager not initialized")
	}

	if err := w.CaddyMgr.GenerateConfig(w.Configs, w.States); err != nil {
		return fmt.Errorf("failed to generate caddy config: %w", err)
	}

	if err := w.CaddyMgr.ReloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload caddy: %w", err)
	}

	return nil
}

func (w *Workspace) ValidateCaddy() error {
	if w.CaddyMgr == nil {
		return fmt.Errorf("caddy manager not initialized")
	}

	return w.CaddyMgr.ValidateCaddyRunning()
}

func (w *Workspace) GetCaddyManager() *caddy.CaddyManager {
	return w.CaddyMgr
}
