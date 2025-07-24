package state

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const initialSchemaVersion int64 = 202507101010

// Migration represents a DB schema migration.
type Migration struct {
	Version int64
	Up      func(tx *sql.Tx) error
}

// migrations is the ordered list of schema migrations.
var migrations = []Migration{
	{
		Version: initialSchemaVersion,
		Up: func(tx *sql.Tx) error {
			_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS app_configs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				app_name TEXT NOT NULL,
				config_yaml TEXT NOT NULL,
				config_sha TEXT NOT NULL,
				created_at DATETIME NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_app_configs_app_name ON app_configs(app_name);
			CREATE INDEX IF NOT EXISTS idx_app_configs_config_sha ON app_configs(config_sha);

			CREATE TABLE IF NOT EXISTS deployments (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				app_name TEXT NOT NULL,
				config_id INTEGER NOT NULL,
				image TEXT NOT NULL,
				started_at DATETIME NOT NULL,
				ended_at DATETIME,
				status TEXT NOT NULL,
				active_color TEXT NOT NULL,
				rollback_of INTEGER,
				FOREIGN KEY(config_id) REFERENCES app_configs(id),
				FOREIGN KEY(rollback_of) REFERENCES deployments(id)
			);
			CREATE INDEX IF NOT EXISTS idx_deployments_app_name ON deployments(app_name);

			CREATE TABLE IF NOT EXISTS deployment_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				deployment_id INTEGER NOT NULL,
				app_name TEXT NOT NULL,
				event_type TEXT NOT NULL,
				payload TEXT,
				error TEXT,
				created_at DATETIME NOT NULL,
				FOREIGN KEY(deployment_id) REFERENCES deployments(id)
			);
			CREATE INDEX IF NOT EXISTS idx_deployment_events_deployment_id ON deployment_events(deployment_id);
			CREATE INDEX IF NOT EXISTS idx_deployment_events_app_name ON deployment_events(app_name);

			CREATE TABLE IF NOT EXISTS current_state (
				app_name TEXT PRIMARY KEY,
				deployment_id INTEGER NOT NULL,
				active_color TEXT NOT NULL,
				image TEXT NOT NULL,
				status TEXT NOT NULL,
				updated_at DATETIME NOT NULL,
				FOREIGN KEY(deployment_id) REFERENCES deployments(id)
			);

			CREATE TABLE IF NOT EXISTS schema_version (
				version INTEGER PRIMARY KEY,
				applied_at DATETIME NOT NULL
			);
			INSERT OR REPLACE INTO schema_version (version, applied_at) VALUES (?, ?);
			`, initialSchemaVersion, time.Now().UTC())
			return err
		},
	},
}

// OpenAndMigrate opens the SQLite DB at path and runs migrations as needed.
func OpenAndMigrate(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to begin migration tx: %w", err)
	}

	// Ensure schema_version table exists
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY, applied_at DATETIME NOT NULL);`)
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, fmt.Errorf("failed to ensure schema_version table: %w", err)
	}

	var currentVersion int64
	row := tx.QueryRow(`SELECT version FROM schema_version ORDER BY version DESC LIMIT 1;`)
	switch err := row.Scan(&currentVersion); err {
	case sql.ErrNoRows:
		currentVersion = 0
	case nil:
		// ok
	default:
		tx.Rollback()
		db.Close()
		return nil, fmt.Errorf("failed to query schema_version: %w", err)
	}

	for _, m := range migrations {
		if m.Version > currentVersion {
			if err := m.Up(tx); err != nil {
				tx.Rollback()
				db.Close()
				return nil, fmt.Errorf("migration to version %d failed: %w", m.Version, err)
			}
			currentVersion = m.Version
		}
	}

	if err := tx.Commit(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to commit migrations: %w", err)
	}

	return db, nil
}

// --- Entity Structs ---

type AppConfig struct {
	ID         int64
	AppName    string
	ConfigYAML string
	ConfigSHA  string
	CreatedAt  time.Time
}

type Deployment struct {
	ID          int64
	AppName     string
	ConfigID    int64
	Image       string
	StartedAt   time.Time
	EndedAt     sql.NullTime
	Status      string
	ActiveColor string
	RollbackOf  sql.NullInt64
}

type DeploymentEvent struct {
	ID           int64
	DeploymentID int64
	AppName      string
	EventType    string
	Payload      string
	Error        sql.NullString
	CreatedAt    time.Time
}

type CurrentState struct {
	AppName      string
	DeploymentID int64
	ActiveColor  string
	Image        string
	Status       string
	UpdatedAt    time.Time
}

// --- AppConfig Methods ---

func InsertAppConfig(db *sql.DB, appName, configYAML, configSHA string) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO app_configs (app_name, config_yaml, config_sha, created_at)
		VALUES (?, ?, ?, ?)
	`, appName, configYAML, configSHA, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetLatestAppConfig(db *sql.DB, appName string) (*AppConfig, error) {
	row := db.QueryRow(`
		SELECT id, app_name, config_yaml, config_sha, created_at
		FROM app_configs
		WHERE app_name = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, appName)
	var ac AppConfig
	if err := row.Scan(&ac.ID, &ac.AppName, &ac.ConfigYAML, &ac.ConfigSHA, &ac.CreatedAt); err != nil {
		return nil, err
	}
	return &ac, nil
}

func GetAppConfigHistory(db *sql.DB, appName string) ([]AppConfig, error) {
	rows, err := db.Query(`
		SELECT id, app_name, config_yaml, config_sha, created_at
		FROM app_configs
		WHERE app_name = ?
		ORDER BY created_at DESC
	`, appName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []AppConfig
	for rows.Next() {
		var ac AppConfig
		if err := rows.Scan(&ac.ID, &ac.AppName, &ac.ConfigYAML, &ac.ConfigSHA, &ac.CreatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, ac)
	}
	return configs, nil
}

// --- Deployment Methods ---

func InsertDeployment(db *sql.DB, appName string, configID int64, image, status, activeColor string, rollbackOf *int64) (int64, error) {
	var rollbackOfVal sql.NullInt64
	if rollbackOf != nil {
		rollbackOfVal = sql.NullInt64{Int64: *rollbackOf, Valid: true}
	}
	res, err := db.Exec(`
		INSERT INTO deployments (app_name, config_id, image, started_at, status, active_color, rollback_of)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, appName, configID, image, time.Now().UTC(), status, activeColor, rollbackOfVal)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetDeploymentHistory(db *sql.DB, appName string) ([]Deployment, error) {
	rows, err := db.Query(`
		SELECT id, app_name, config_id, image, started_at, ended_at, status, active_color, rollback_of
		FROM deployments
		WHERE app_name = ?
		ORDER BY started_at DESC
	`, appName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deployments []Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.AppName, &d.ConfigID, &d.Image, &d.StartedAt, &d.EndedAt, &d.Status, &d.ActiveColor, &d.RollbackOf); err != nil {
			return nil, err
		}
		deployments = append(deployments, d)
	}
	return deployments, nil
}

// --- DeploymentEvent Methods ---

func InsertDeploymentEvent(db *sql.DB, deploymentID int64, appName, eventType, payload string, errMsg *string) (int64, error) {
	var errVal sql.NullString
	if errMsg != nil {
		errVal = sql.NullString{String: *errMsg, Valid: true}
	}
	res, err := db.Exec(`
		INSERT INTO deployment_events (deployment_id, app_name, event_type, payload, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, deploymentID, appName, eventType, payload, errVal, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetDeploymentEvents(db *sql.DB, deploymentID int64) ([]DeploymentEvent, error) {
	rows, err := db.Query(`
		SELECT id, deployment_id, app_name, event_type, payload, error, created_at
		FROM deployment_events
		WHERE deployment_id = ?
		ORDER BY created_at ASC
	`, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []DeploymentEvent
	for rows.Next() {
		var e DeploymentEvent
		if err := rows.Scan(&e.ID, &e.DeploymentID, &e.AppName, &e.EventType, &e.Payload, &e.Error, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

// --- CurrentState Methods ---

func UpsertCurrentState(db *sql.DB, appName string, deploymentID int64, activeColor, image, status string) error {
	_, err := db.Exec(`
		INSERT INTO current_state (app_name, deployment_id, active_color, image, status, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(app_name) DO UPDATE SET
			deployment_id=excluded.deployment_id,
			active_color=excluded.active_color,
			image=excluded.image,
			status=excluded.status,
			updated_at=excluded.updated_at
	`, appName, deploymentID, activeColor, image, status, time.Now().UTC())
	return err
}

func GetCurrentState(db *sql.DB, appName string) (*CurrentState, error) {
	row := db.QueryRow(`
		SELECT app_name, deployment_id, active_color, image, status, updated_at
		FROM current_state
		WHERE app_name = ?
	`, appName)
	var cs CurrentState
	if err := row.Scan(&cs.AppName, &cs.DeploymentID, &cs.ActiveColor, &cs.Image, &cs.Status, &cs.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			// Try to auto-initialize from latest deployment
			deployments, derr := GetDeploymentHistory(db, appName)
			if derr == nil && len(deployments) > 0 {
				latest := deployments[0]
				_ = UpsertCurrentState(db, appName, latest.ID, latest.ActiveColor, latest.Image, latest.Status)
			} else {
				// Fallback: use defaults
				_ = UpsertCurrentState(db, appName, 0, "blue", "", "unknown")
			}
			// Re-query
			row2 := db.QueryRow(`
				SELECT app_name, deployment_id, active_color, image, status, updated_at
				FROM current_state
				WHERE app_name = ?
			`, appName)
			if err2 := row2.Scan(&cs.AppName, &cs.DeploymentID, &cs.ActiveColor, &cs.Image, &cs.Status, &cs.UpdatedAt); err2 != nil {
				return nil, err2
			}
			return &cs, nil
		}
		return nil, err
	}
	return &cs, nil
}

func GetAllCurrentStates(db *sql.DB) ([]CurrentState, error) {
	rows, err := db.Query(`
		SELECT app_name, deployment_id, active_color, image, status, updated_at
		FROM current_state
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var states []CurrentState
	for rows.Next() {
		var cs CurrentState
		if err := rows.Scan(&cs.AppName, &cs.DeploymentID, &cs.ActiveColor, &cs.Image, &cs.Status, &cs.UpdatedAt); err != nil {
			return nil, err
		}
		states = append(states, cs)
	}
	return states, nil
}
