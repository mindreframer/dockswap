package state

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	db, err := OpenAndMigrate(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

func TestAppConfig_InsertAndQuery(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	id, err := InsertAppConfig(db, "web-api", "foo: bar", "sha1")
	if err != nil {
		t.Fatalf("insert app config: %v", err)
	}
	if id == 0 {
		t.Fatal("expected nonzero id")
	}

	cfg, err := GetLatestAppConfig(db, "web-api")
	if err != nil {
		t.Fatalf("get latest app config: %v", err)
	}
	if cfg.ConfigSHA != "sha1" || cfg.AppName != "web-api" {
		t.Errorf("unexpected config: %+v", cfg)
	}

	// Insert another config, test ordering
	_, err = InsertAppConfig(db, "web-api", "foo: baz", "sha2")
	if err != nil {
		t.Fatalf("insert 2nd app config: %v", err)
	}
	cfg, err = GetLatestAppConfig(db, "web-api")
	if err != nil {
		t.Fatalf("get latest app config: %v", err)
	}
	if cfg.ConfigSHA != "sha2" {
		t.Errorf("expected sha2, got %s", cfg.ConfigSHA)
	}

	history, err := GetAppConfigHistory(db, "web-api")
	if err != nil {
		t.Fatalf("get config history: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("expected 2 configs, got %d", len(history))
	}
	if history[0].ConfigSHA != "sha2" || history[1].ConfigSHA != "sha1" {
		t.Errorf("unexpected order: %+v", history)
	}
}

func TestDeployment_InsertAndHistory(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	cfgID, _ := InsertAppConfig(db, "web-api", "foo: bar", "sha1")
	id1, err := InsertDeployment(db, "web-api", cfgID, "img1", "success", "blue", nil)
	if err != nil {
		t.Fatalf("insert deployment: %v", err)
	}
	id2, err := InsertDeployment(db, "web-api", cfgID, "img2", "failed", "green", &id1)
	if err != nil {
		t.Fatalf("insert 2nd deployment: %v", err)
	}

	hist, err := GetDeploymentHistory(db, "web-api")
	if err != nil {
		t.Fatalf("get deployment history: %v", err)
	}
	if len(hist) != 2 {
		t.Errorf("expected 2 deployments, got %d", len(hist))
	}
	if hist[0].ID != id2 || hist[1].ID != id1 {
		t.Errorf("unexpected order: %+v", hist)
	}
}

func TestDeploymentEvent_InsertAndQuery(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	cfgID, _ := InsertAppConfig(db, "web-api", "foo: bar", "sha1")
	depID, _ := InsertDeployment(db, "web-api", cfgID, "img1", "success", "blue", nil)

	msg := "err msg"
	id, err := InsertDeploymentEvent(db, depID, "web-api", "container_started", `{"foo":1}`, &msg)
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if id == 0 {
		t.Fatal("expected nonzero id")
	}
	_, err = InsertDeploymentEvent(db, depID, "web-api", "health_check_passed", `{"foo":2}`, nil)
	if err != nil {
		t.Fatalf("insert 2nd event: %v", err)
	}

	events, err := GetDeploymentEvents(db, depID)
	if err != nil {
		t.Fatalf("get events: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
	if events[0].EventType != "container_started" || events[1].EventType != "health_check_passed" {
		t.Errorf("unexpected event order: %+v", events)
	}
	if !events[0].Error.Valid || events[0].Error.String != "err msg" {
		t.Errorf("expected error msg, got %+v", events[0].Error)
	}
}

func TestCurrentState_UpsertAndQuery(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	cfgID, _ := InsertAppConfig(db, "web-api", "foo: bar", "sha1")
	depID, _ := InsertDeployment(db, "web-api", cfgID, "img1", "success", "blue", nil)

	err := UpsertCurrentState(db, "web-api", depID, "blue", "img1", "stable")
	if err != nil {
		t.Fatalf("upsert current state: %v", err)
	}
	cs, err := GetCurrentState(db, "web-api")
	if err != nil {
		t.Fatalf("get current state: %v", err)
	}
	if cs.AppName != "web-api" || cs.Status != "stable" {
		t.Errorf("unexpected current state: %+v", cs)
	}

	// Upsert again (update)
	err = UpsertCurrentState(db, "web-api", depID, "green", "img2", "failed")
	if err != nil {
		t.Fatalf("upsert current state 2: %v", err)
	}
	cs, err = GetCurrentState(db, "web-api")
	if err != nil {
		t.Fatalf("get current state 2: %v", err)
	}
	if cs.ActiveColor != "green" || cs.Image != "img2" || cs.Status != "failed" {
		t.Errorf("unexpected updated state: %+v", cs)
	}

	// Insert another app
	cfgID2, _ := InsertAppConfig(db, "api2", "foo: bar", "sha2")
	depID2, _ := InsertDeployment(db, "api2", cfgID2, "img3", "success", "blue", nil)
	_ = UpsertCurrentState(db, "api2", depID2, "blue", "img3", "stable")

	all, err := GetAllCurrentStates(db)
	if err != nil {
		t.Fatalf("get all current states: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 current states, got %d", len(all))
	}
}
