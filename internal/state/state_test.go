package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateState(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		state   AppState
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid state",
			state: AppState{
				Name:           "test-app",
				CurrentImage:   "nginx:1.21",
				DesiredImage:   "nginx:1.21",
				ActiveColor:    "blue",
				Status:         "stable",
				LastDeployment: now,
				LastUpdated:    now,
			},
			wantErr: false,
		},
		{
			name: "missing app name",
			state: AppState{
				CurrentImage:   "nginx:1.21",
				DesiredImage:   "nginx:1.21",
				ActiveColor:    "blue",
				Status:         "stable",
				LastDeployment: now,
				LastUpdated:    now,
			},
			wantErr: true,
			errMsg:  "app name is required",
		},
		{
			name: "missing current image",
			state: AppState{
				Name:           "test-app",
				DesiredImage:   "nginx:1.21",
				ActiveColor:    "blue",
				Status:         "stable",
				LastDeployment: now,
				LastUpdated:    now,
			},
			wantErr: true,
			errMsg:  "current_image is required",
		},
		{
			name: "missing desired image",
			state: AppState{
				Name:           "test-app",
				CurrentImage:   "nginx:1.21",
				ActiveColor:    "blue",
				Status:         "stable",
				LastDeployment: now,
				LastUpdated:    now,
			},
			wantErr: true,
			errMsg:  "desired_image is required",
		},
		{
			name: "invalid active color",
			state: AppState{
				Name:           "test-app",
				CurrentImage:   "nginx:1.21",
				DesiredImage:   "nginx:1.21",
				ActiveColor:    "red",
				Status:         "stable",
				LastDeployment: now,
				LastUpdated:    now,
			},
			wantErr: true,
			errMsg:  "active_color must be 'blue' or 'green', got: red",
		},
		{
			name: "invalid status",
			state: AppState{
				Name:           "test-app",
				CurrentImage:   "nginx:1.21",
				DesiredImage:   "nginx:1.21",
				ActiveColor:    "blue",
				Status:         "invalid",
				LastDeployment: now,
				LastUpdated:    now,
			},
			wantErr: true,
			errMsg:  "invalid status: invalid",
		},
		{
			name: "zero last deployment",
			state: AppState{
				Name:         "test-app",
				CurrentImage: "nginx:1.21",
				DesiredImage: "nginx:1.21",
				ActiveColor:  "blue",
				Status:       "stable",
				LastUpdated:  now,
			},
			wantErr: true,
			errMsg:  "last_deployment is required",
		},
		{
			name: "zero last updated",
			state: AppState{
				Name:           "test-app",
				CurrentImage:   "nginx:1.21",
				DesiredImage:   "nginx:1.21",
				ActiveColor:    "blue",
				Status:         "stable",
				LastDeployment: now,
			},
			wantErr: true,
			errMsg:  "last_updated is required",
		},
		{
			name: "last updated before last deployment",
			state: AppState{
				Name:           "test-app",
				CurrentImage:   "nginx:1.21",
				DesiredImage:   "nginx:1.21",
				ActiveColor:    "blue",
				Status:         "stable",
				LastDeployment: now,
				LastUpdated:    now.Add(-time.Hour),
			},
			wantErr: true,
			errMsg:  "last_updated cannot be before last_deployment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateState(&tt.state)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateState() expected error but got none")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("validateState() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateState() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestLoadAppState(t *testing.T) {
	tempDir := t.TempDir()

	validYAML := `name: "test-app"
current_image: "nginx:1.21"
desired_image: "nginx:1.21"
active_color: "blue"
status: "stable"
last_deployment: "2025-07-23T10:30:00Z"
last_updated: "2025-07-23T10:35:00Z"`

	invalidYAML := `name: "test-app"
current_image: "nginx:1.21"
desired_image: "nginx:1.21"
active_color: "red"
status: "stable"
last_deployment: "2025-07-23T10:30:00Z"
last_updated: "2025-07-23T10:35:00Z"`

	malformedYAML := `name: "test-app"
current_image: nginx:1.21
invalid: yaml: structure`

	tests := []struct {
		name     string
		content  string
		wantErr  bool
		wantName string
	}{
		{
			name:     "valid state",
			content:  validYAML,
			wantErr:  false,
			wantName: "test-app",
		},
		{
			name:    "invalid state - bad color",
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
			stateFile := filepath.Join(tempDir, "test-state.yaml")
			err := os.WriteFile(stateFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test state file: %v", err)
			}

			state, err := LoadAppState(stateFile)
			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadAppState() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("LoadAppState() unexpected error = %v", err)
				}
				if state == nil {
					t.Errorf("LoadAppState() returned nil state")
				} else if state.Name != tt.wantName {
					t.Errorf("LoadAppState() name = %v, want %v", state.Name, tt.wantName)
				}
			}
		})
	}

	t.Run("file not found", func(t *testing.T) {
		_, err := LoadAppState("/nonexistent/path/state.yaml")
		if err == nil {
			t.Errorf("LoadAppState() expected error for nonexistent file")
		}
	})
}

func TestSaveAppState(t *testing.T) {
	tempDir := t.TempDir()

	now := time.Now().UTC()
	state := &AppState{
		Name:           "test-app",
		CurrentImage:   "nginx:1.21",
		DesiredImage:   "nginx:1.22",
		ActiveColor:    "blue",
		Status:         "deploying",
		LastDeployment: now,
		LastUpdated:    now,
	}

	t.Run("save valid state", func(t *testing.T) {
		stateFile := filepath.Join(tempDir, "save-test.yaml")
		err := SaveAppState(stateFile, state)
		if err != nil {
			t.Errorf("SaveAppState() unexpected error = %v", err)
		}

		loadedState, err := LoadAppState(stateFile)
		if err != nil {
			t.Fatalf("LoadAppState() after save failed: %v", err)
		}

		if loadedState.Name != state.Name {
			t.Errorf("Saved state name = %v, want %v", loadedState.Name, state.Name)
		}
		if loadedState.CurrentImage != state.CurrentImage {
			t.Errorf("Saved state current_image = %v, want %v", loadedState.CurrentImage, state.CurrentImage)
		}
		if loadedState.LastUpdated.Before(state.LastUpdated) {
			t.Errorf("SaveAppState() should update last_updated timestamp")
		}
	})

	t.Run("save invalid state", func(t *testing.T) {
		invalidState := &AppState{
			Name:         "test-app",
			CurrentImage: "nginx:1.21",
			ActiveColor:  "red",
		}

		stateFile := filepath.Join(tempDir, "invalid-test.yaml")
		err := SaveAppState(stateFile, invalidState)
		if err == nil {
			t.Errorf("SaveAppState() expected error for invalid state")
		}
	})

	t.Run("create directory", func(t *testing.T) {
		nestedDir := filepath.Join(tempDir, "nested", "path")
		stateFile := filepath.Join(nestedDir, "state.yaml")

		err := SaveAppState(stateFile, state)
		if err != nil {
			t.Errorf("SaveAppState() failed to create directory: %v", err)
		}

		if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
			t.Errorf("SaveAppState() did not create directory")
		}
	})
}

func TestLoadAllStates(t *testing.T) {
	tempDir := t.TempDir()

	state1YAML := `name: "app1"
current_image: "nginx:1.21"
desired_image: "nginx:1.21"
active_color: "blue"
status: "stable"
last_deployment: "2025-07-23T10:30:00Z"
last_updated: "2025-07-23T10:35:00Z"`

	state2YAML := `name: "app2"
current_image: "postgres:13"
desired_image: "postgres:14"
active_color: "green"
status: "deploying"
last_deployment: "2025-07-23T09:30:00Z"
last_updated: "2025-07-23T10:30:00Z"`

	err := os.WriteFile(filepath.Join(tempDir, "app1.yaml"), []byte(state1YAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write app1 state: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "app2.yml"), []byte(state2YAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write app2 state: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "not-yaml.txt"), []byte("not yaml"), 0644)
	if err != nil {
		t.Fatalf("Failed to write non-yaml file: %v", err)
	}

	t.Run("load valid states", func(t *testing.T) {
		states, err := LoadAllStates(tempDir)
		if err != nil {
			t.Fatalf("LoadAllStates() unexpected error = %v", err)
		}

		if len(states) != 2 {
			t.Errorf("LoadAllStates() loaded %d states, want 2", len(states))
		}

		if _, exists := states["app1"]; !exists {
			t.Errorf("LoadAllStates() missing app1 state")
		}

		if _, exists := states["app2"]; !exists {
			t.Errorf("LoadAllStates() missing app2 state")
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		_, err := LoadAllStates("/nonexistent/directory")
		if err == nil {
			t.Errorf("LoadAllStates() expected error for nonexistent directory")
		}
	})
}

func TestCreateInitialState(t *testing.T) {
	name := "test-app"
	image := "nginx:1.21"
	color := ColorBlue

	state := CreateInitialState(name, image, color)

	if state.Name != name {
		t.Errorf("CreateInitialState() name = %v, want %v", state.Name, name)
	}
	if state.CurrentImage != image {
		t.Errorf("CreateInitialState() current_image = %v, want %v", state.CurrentImage, image)
	}
	if state.DesiredImage != image {
		t.Errorf("CreateInitialState() desired_image = %v, want %v", state.DesiredImage, image)
	}
	if state.ActiveColor != string(color) {
		t.Errorf("CreateInitialState() active_color = %v, want %v", state.ActiveColor, string(color))
	}
	if state.Status != string(StatusStable) {
		t.Errorf("CreateInitialState() status = %v, want %v", state.Status, string(StatusStable))
	}
	if state.LastDeployment.IsZero() {
		t.Errorf("CreateInitialState() last_deployment should not be zero")
	}
	if state.LastUpdated.IsZero() {
		t.Errorf("CreateInitialState() last_updated should not be zero")
	}
}

func TestAppStateStateMethods(t *testing.T) {
	now := time.Now().UTC()
	state := &AppState{
		Name:           "test-app",
		CurrentImage:   "nginx:1.21",
		DesiredImage:   "nginx:1.21",
		ActiveColor:    "blue",
		Status:         "stable",
		LastDeployment: now,
		LastUpdated:    now,
	}

	t.Run("SetDeploying", func(t *testing.T) {
		newImage := "nginx:1.22"
		state.SetDeploying(newImage)

		if state.DesiredImage != newImage {
			t.Errorf("SetDeploying() desired_image = %v, want %v", state.DesiredImage, newImage)
		}
		if state.Status != string(StatusDeploying) {
			t.Errorf("SetDeploying() status = %v, want %v", state.Status, string(StatusDeploying))
		}
		if !state.LastUpdated.After(now) {
			t.Errorf("SetDeploying() should update last_updated timestamp")
		}
	})

	t.Run("CompleteDeployment", func(t *testing.T) {
		beforeComplete := state.LastUpdated
		state.CompleteDeployment(ColorGreen)

		if state.CurrentImage != state.DesiredImage {
			t.Errorf("CompleteDeployment() current_image = %v, want %v", state.CurrentImage, state.DesiredImage)
		}
		if state.ActiveColor != string(ColorGreen) {
			t.Errorf("CompleteDeployment() active_color = %v, want %v", state.ActiveColor, string(ColorGreen))
		}
		if state.Status != string(StatusStable) {
			t.Errorf("CompleteDeployment() status = %v, want %v", state.Status, string(StatusStable))
		}
		if !state.LastDeployment.After(beforeComplete) {
			t.Errorf("CompleteDeployment() should update last_deployment timestamp")
		}
	})

	t.Run("IsDeploymentInProgress", func(t *testing.T) {
		state.Status = string(StatusStable)
		if state.IsDeploymentInProgress() {
			t.Errorf("IsDeploymentInProgress() should be false for stable status")
		}

		state.Status = string(StatusDeploying)
		if !state.IsDeploymentInProgress() {
			t.Errorf("IsDeploymentInProgress() should be true for deploying status")
		}
	})

	t.Run("NeedsDeployment", func(t *testing.T) {
		state.Status = string(StatusStable)
		state.CurrentImage = "nginx:1.21"
		state.DesiredImage = "nginx:1.21"
		if state.NeedsDeployment() {
			t.Errorf("NeedsDeployment() should be false when images match")
		}

		state.DesiredImage = "nginx:1.22"
		if !state.NeedsDeployment() {
			t.Errorf("NeedsDeployment() should be true when images differ")
		}

		state.Status = string(StatusDeploying)
		if state.NeedsDeployment() {
			t.Errorf("NeedsDeployment() should be false when deployment is in progress")
		}
	})

	t.Run("GetInactiveColor", func(t *testing.T) {
		state.ActiveColor = string(ColorBlue)
		if state.GetInactiveColor() != ColorGreen {
			t.Errorf("GetInactiveColor() = %v, want %v", state.GetInactiveColor(), ColorGreen)
		}

		state.ActiveColor = string(ColorGreen)
		if state.GetInactiveColor() != ColorBlue {
			t.Errorf("GetInactiveColor() = %v, want %v", state.GetInactiveColor(), ColorBlue)
		}
	})
}
