package state

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type AppState struct {
	Name           string    `yaml:"name"`
	CurrentImage   string    `yaml:"current_image"`
	DesiredImage   string    `yaml:"desired_image"`
	ActiveColor    string    `yaml:"active_color"`
	Status         string    `yaml:"status"`
	LastDeployment time.Time `yaml:"last_deployment"`
	LastUpdated    time.Time `yaml:"last_updated"`
}

type DeploymentStatus string

const (
	StatusStable      DeploymentStatus = "stable"
	StatusDeploying   DeploymentStatus = "deploying"
	StatusDraining    DeploymentStatus = "draining"
	StatusRollingBack DeploymentStatus = "rolling_back"
	StatusFailed      DeploymentStatus = "failed"
	StatusUnknown     DeploymentStatus = "unknown"
)

type Color string

const (
	ColorBlue  Color = "blue"
	ColorGreen Color = "green"
)

func LoadAppState(statePath string) (*AppState, error) {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file %s: %w", statePath, err)
	}

	var state AppState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse YAML state %s: %w", statePath, err)
	}

	if err := validateState(&state); err != nil {
		return nil, fmt.Errorf("state validation failed for %s: %w", statePath, err)
	}

	return &state, nil
}

func SaveAppState(statePath string, state *AppState) error {
	if err := validateState(state); err != nil {
		return fmt.Errorf("state validation failed: %w", err)
	}

	state.LastUpdated = time.Now().UTC()

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state to YAML: %w", err)
	}

	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory %s: %w", dir, err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file %s: %w", statePath, err)
	}

	return nil
}

func LoadAllStates(stateDir string) (map[string]*AppState, error) {
	states := make(map[string]*AppState)

	err := filepath.Walk(stateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml") {
			state, err := LoadAppState(path)
			if err != nil {
				return fmt.Errorf("failed to load state %s: %w", path, err)
			}
			states[state.Name] = state
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load states from %s: %w", stateDir, err)
	}

	return states, nil
}

func CreateInitialState(name, image string, activeColor Color) *AppState {
	now := time.Now().UTC()
	return &AppState{
		Name:           name,
		CurrentImage:   image,
		DesiredImage:   image,
		ActiveColor:    string(activeColor),
		Status:         string(StatusStable),
		LastDeployment: now,
		LastUpdated:    now,
	}
}

func (s *AppState) SetDeploying(desiredImage string) {
	s.DesiredImage = desiredImage
	s.Status = string(StatusDeploying)
	s.LastUpdated = time.Now().UTC()
}

func (s *AppState) SetDraining() {
	s.Status = string(StatusDraining)
	s.LastUpdated = time.Now().UTC()
}

func (s *AppState) CompleteDeployment(newActiveColor Color) {
	s.CurrentImage = s.DesiredImage
	s.ActiveColor = string(newActiveColor)
	s.Status = string(StatusStable)
	s.LastDeployment = time.Now().UTC()
	s.LastUpdated = s.LastDeployment
}

func (s *AppState) SetFailed() {
	s.Status = string(StatusFailed)
	s.LastUpdated = time.Now().UTC()
}

func (s *AppState) StartRollback() {
	s.Status = string(StatusRollingBack)
	s.LastUpdated = time.Now().UTC()
}

func (s *AppState) IsDeploymentInProgress() bool {
	status := DeploymentStatus(s.Status)
	return status == StatusDeploying || status == StatusDraining || status == StatusRollingBack
}

func (s *AppState) NeedsDeployment() bool {
	return s.CurrentImage != s.DesiredImage && !s.IsDeploymentInProgress()
}

func (s *AppState) GetInactiveColor() Color {
	if s.ActiveColor == string(ColorBlue) {
		return ColorGreen
	}
	return ColorBlue
}

func validateState(state *AppState) error {
	if state.Name == "" {
		return fmt.Errorf("app name is required")
	}

	if state.CurrentImage == "" {
		return fmt.Errorf("current_image is required")
	}

	if state.DesiredImage == "" {
		return fmt.Errorf("desired_image is required")
	}

	if state.ActiveColor != string(ColorBlue) && state.ActiveColor != string(ColorGreen) {
		return fmt.Errorf("active_color must be 'blue' or 'green', got: %s", state.ActiveColor)
	}

	validStatuses := map[string]bool{
		string(StatusStable):      true,
		string(StatusDeploying):   true,
		string(StatusDraining):    true,
		string(StatusRollingBack): true,
		string(StatusFailed):      true,
		string(StatusUnknown):     true,
	}

	if !validStatuses[state.Status] {
		return fmt.Errorf("invalid status: %s", state.Status)
	}

	if state.LastDeployment.IsZero() {
		return fmt.Errorf("last_deployment is required")
	}

	if state.LastUpdated.IsZero() {
		return fmt.Errorf("last_updated is required")
	}

	if state.LastUpdated.Before(state.LastDeployment) {
		return fmt.Errorf("last_updated cannot be before last_deployment")
	}

	return nil
}
