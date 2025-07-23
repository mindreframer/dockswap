package deployment

import (
	"fmt"
	"testing"
	"time"
)

type MockActionProvider struct {
	startContainerError   error
	checkHealthResult     bool
	checkHealthError      error
	updateCaddyError      error
	drainConnectionsError error
	stopContainerError    error
	rollbackCaddyError    error

	// Call tracking
	startContainerCalls   []StartContainerCall
	checkHealthCalls      []CheckHealthCall
	updateCaddyCalls      []UpdateCaddyCall
	drainConnectionsCalls []DrainConnectionsCall
	stopContainerCalls    []StopContainerCall
	rollbackCaddyCalls    []RollbackCaddyCall
}

type StartContainerCall struct {
	AppName string
	Color   string
	Image   string
}

type CheckHealthCall struct {
	AppName string
	Color   string
}

type UpdateCaddyCall struct {
	AppName     string
	ActiveColor string
}

type DrainConnectionsCall struct {
	AppName string
	Color   string
	Timeout time.Duration
}

type StopContainerCall struct {
	AppName string
	Color   string
}

type RollbackCaddyCall struct {
	AppName     string
	ActiveColor string
}

func (m *MockActionProvider) StartContainer(appName, color, image string) error {
	m.startContainerCalls = append(m.startContainerCalls, StartContainerCall{
		AppName: appName,
		Color:   color,
		Image:   image,
	})
	return m.startContainerError
}

func (m *MockActionProvider) CheckHealth(appName, color string) (bool, error) {
	m.checkHealthCalls = append(m.checkHealthCalls, CheckHealthCall{
		AppName: appName,
		Color:   color,
	})
	return m.checkHealthResult, m.checkHealthError
}

func (m *MockActionProvider) UpdateCaddy(appName, activeColor string) error {
	m.updateCaddyCalls = append(m.updateCaddyCalls, UpdateCaddyCall{
		AppName:     appName,
		ActiveColor: activeColor,
	})
	return m.updateCaddyError
}

func (m *MockActionProvider) DrainConnections(appName, color string, timeout time.Duration) error {
	m.drainConnectionsCalls = append(m.drainConnectionsCalls, DrainConnectionsCall{
		AppName: appName,
		Color:   color,
		Timeout: timeout,
	})
	return m.drainConnectionsError
}

func (m *MockActionProvider) StopContainer(appName, color string) error {
	m.stopContainerCalls = append(m.stopContainerCalls, StopContainerCall{
		AppName: appName,
		Color:   color,
	})
	return m.stopContainerError
}

func (m *MockActionProvider) RollbackCaddy(appName, activeColor string) error {
	m.rollbackCaddyCalls = append(m.rollbackCaddyCalls, RollbackCaddyCall{
		AppName:     appName,
		ActiveColor: activeColor,
	})
	return m.rollbackCaddyError
}

func TestNew(t *testing.T) {
	actions := &MockActionProvider{}
	sm := New("test-app", "blue", actions, nil)

	if sm.GetState() != StateStable {
		t.Errorf("New() initial state = %v, want %v", sm.GetState(), StateStable)
	}
	if sm.GetActiveColor() != "blue" {
		t.Errorf("New() active color = %v, want %v", sm.GetActiveColor(), "blue")
	}
	if sm.appName != "test-app" {
		t.Errorf("New() app name = %v, want %v", sm.appName, "test-app")
	}
}

func TestSuccessfulDeployment(t *testing.T) {
	actions := &MockActionProvider{}
	sm := New("test-app", "blue", actions, nil)

	// Start deployment
	err := sm.Deploy("nginx:1.22")
	if err != nil {
		t.Fatalf("Deploy() failed: %v", err)
	}

	// Verify state transition and action call
	if sm.GetState() != StateStarting {
		t.Errorf("After Deploy() state = %v, want %v", sm.GetState(), StateStarting)
	}
	if len(actions.startContainerCalls) != 1 {
		t.Errorf("StartContainer called %d times, want 1", len(actions.startContainerCalls))
	}
	if actions.startContainerCalls[0].Color != "green" {
		t.Errorf("StartContainer color = %v, want green", actions.startContainerCalls[0].Color)
	}

	// Container started successfully
	err = sm.CompleteContainerOperation(true, false)
	if err != nil {
		t.Fatalf("CompleteContainerOperation() failed: %v", err)
	}
	if sm.GetState() != StateHealthCheck {
		t.Errorf("After container started state = %v, want %v", sm.GetState(), StateHealthCheck)
	}

	// Health check passed
	err = sm.CompleteHealthCheck(true)
	if err != nil {
		t.Fatalf("CompleteHealthCheck() failed: %v", err)
	}
	if sm.GetState() != StateSwitching {
		t.Errorf("After health check state = %v, want %v", sm.GetState(), StateSwitching)
	}
	if len(actions.updateCaddyCalls) != 1 {
		t.Errorf("UpdateCaddy called %d times, want 1", len(actions.updateCaddyCalls))
	}

	// Caddy updated successfully
	err = sm.CompleteCaddyUpdate(true)
	if err != nil {
		t.Fatalf("CompleteCaddyUpdate() failed: %v", err)
	}
	if sm.GetState() != StateDraining {
		t.Errorf("After Caddy update state = %v, want %v", sm.GetState(), StateDraining)
	}
	if len(actions.drainConnectionsCalls) != 1 {
		t.Errorf("DrainConnections called %d times, want 1", len(actions.drainConnectionsCalls))
	}

	// Drain completed
	err = sm.CompleteDrain()
	if err != nil {
		t.Fatalf("CompleteDrain() failed: %v", err)
	}
	if sm.GetState() != StateStopping {
		t.Errorf("After drain complete state = %v, want %v", sm.GetState(), StateStopping)
	}
	if len(actions.stopContainerCalls) != 1 {
		t.Errorf("StopContainer called %d times, want 1", len(actions.stopContainerCalls))
	}
	if actions.stopContainerCalls[0].Color != "blue" {
		t.Errorf("StopContainer color = %v, want blue", actions.stopContainerCalls[0].Color)
	}

	// Old container stopped
	err = sm.CompleteContainerOperation(true, false)
	if err != nil {
		t.Fatalf("CompleteContainerOperation() failed: %v", err)
	}
	if sm.GetState() != StateStable {
		t.Errorf("After container stopped state = %v, want %v", sm.GetState(), StateStable)
	}
	if sm.GetActiveColor() != "green" {
		t.Errorf("After deployment active color = %v, want green", sm.GetActiveColor())
	}
}

func TestContainerStartFailure(t *testing.T) {
	actions := &MockActionProvider{
		startContainerError: fmt.Errorf("container start failed"),
	}
	sm := New("test-app", "blue", actions, nil)

	// Start deployment
	err := sm.Deploy("nginx:1.22")
	if err == nil {
		t.Errorf("Deploy() should fail when container start fails")
	}
	if sm.GetState() != StateStarting {
		t.Errorf("After failed Deploy() state = %v, want %v", sm.GetState(), StateStarting)
	}

	// Container failed to start
	err = sm.CompleteContainerOperation(false, false)
	if err == nil {
		t.Errorf("CompleteContainerOperation() should return error for failure")
	}
	if sm.GetState() != StateFailed {
		t.Errorf("After container start failure state = %v, want %v", sm.GetState(), StateFailed)
	}
}

func TestHealthCheckFailureRollback(t *testing.T) {
	actions := &MockActionProvider{}
	sm := New("test-app", "blue", actions, nil)

	// Get to health check state
	sm.Deploy("nginx:1.22")
	sm.CompleteContainerOperation(true, false)

	// Health check failed
	err := sm.CompleteHealthCheck(false)
	if err != nil {
		t.Fatalf("CompleteHealthCheck() failed: %v", err)
	}
	if sm.GetState() != StateRollingBack {
		t.Errorf("After health check failure state = %v, want %v", sm.GetState(), StateRollingBack)
	}

	// Verify rollback action (stop new container)
	if len(actions.stopContainerCalls) != 1 {
		t.Errorf("StopContainer called %d times during rollback, want 1", len(actions.stopContainerCalls))
	}
	if actions.stopContainerCalls[0].Color != "green" {
		t.Errorf("Rollback StopContainer color = %v, want green", actions.stopContainerCalls[0].Color)
	}

	// Complete rollback
	err = sm.CompleteContainerOperation(true, true)
	if err != nil {
		t.Fatalf("CompleteContainerOperation() rollback failed: %v", err)
	}
	if sm.GetState() != StateStable {
		t.Errorf("After rollback state = %v, want %v", sm.GetState(), StateStable)
	}
	if sm.GetActiveColor() != "blue" {
		t.Errorf("After rollback active color = %v, want blue", sm.GetActiveColor())
	}
}

func TestCaddyUpdateFailureRollback(t *testing.T) {
	actions := &MockActionProvider{
		updateCaddyError: fmt.Errorf("caddy update failed"),
	}
	sm := New("test-app", "blue", actions, nil)

	// Get to switching state
	sm.Deploy("nginx:1.22")
	sm.CompleteContainerOperation(true, false)
	sm.CompleteHealthCheck(true)

	// Caddy update failed
	err := sm.CompleteCaddyUpdate(false)
	if err != nil {
		t.Fatalf("CompleteCaddyUpdate() failed: %v", err)
	}
	if sm.GetState() != StateRollingBack {
		t.Errorf("After Caddy failure state = %v, want %v", sm.GetState(), StateRollingBack)
	}
}

func TestInvalidStateTransitions(t *testing.T) {
	actions := &MockActionProvider{}
	sm := New("test-app", "blue", actions, nil)

	tests := []struct {
		name         string
		currentState DeploymentState
		event        DeploymentEvent
		shouldError  bool
	}{
		{"Deploy from non-stable", StateStarting, EventDeploy, true},
		{"Invalid event in stable", StateStable, EventContainerStarted, true},
		{"Invalid event in starting", StateStarting, EventHealthCheckPassed, true},
		{"Invalid event in health check", StateHealthCheck, EventDrainComplete, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm.state = tt.currentState
			err := sm.ProcessEvent(tt.event)
			if tt.shouldError && err == nil {
				t.Errorf("ProcessEvent() should fail for invalid transition")
			} else if !tt.shouldError && err != nil {
				t.Errorf("ProcessEvent() should not fail: %v", err)
			}
		})
	}
}

func TestStateHistory(t *testing.T) {
	actions := &MockActionProvider{}
	sm := New("test-app", "blue", actions, nil)

	// Perform some state transitions
	sm.Deploy("nginx:1.22")
	sm.CompleteContainerOperation(true, false)

	history := sm.GetStateHistory()
	if len(history) != 2 {
		t.Errorf("State history length = %d, want 2", len(history))
	}

	// Check first transition
	if history[0].FromState != StateStable || history[0].ToState != StateStarting {
		t.Errorf("First transition = %v -> %v, want %v -> %v",
			history[0].FromState, history[0].ToState, StateStable, StateStarting)
	}
	if history[0].Event != EventDeploy {
		t.Errorf("First transition event = %v, want %v", history[0].Event, EventDeploy)
	}

	// Check second transition
	if history[1].FromState != StateStarting || history[1].ToState != StateHealthCheck {
		t.Errorf("Second transition = %v -> %v, want %v -> %v",
			history[1].FromState, history[1].ToState, StateStarting, StateHealthCheck)
	}
}

func TestManualRecovery(t *testing.T) {
	actions := &MockActionProvider{}
	sm := New("test-app", "blue", actions, nil)

	// Force into failed state
	sm.state = StateFailed

	err := sm.RecoverManually()
	if err != nil {
		t.Errorf("RecoverManually() failed: %v", err)
	}
	if sm.GetState() != StateStable {
		t.Errorf("After manual recovery state = %v, want %v", sm.GetState(), StateStable)
	}
}

func TestTimeoutConfiguration(t *testing.T) {
	actions := &MockActionProvider{}
	sm := New("test-app", "blue", actions, nil)

	healthTimeout := 120 * time.Second
	drainTimeout := 45 * time.Second

	sm.SetHealthCheckTimeout(healthTimeout)
	sm.SetDrainTimeout(drainTimeout)

	if sm.healthCheckTimeout != healthTimeout {
		t.Errorf("Health check timeout = %v, want %v", sm.healthCheckTimeout, healthTimeout)
	}
	if sm.drainTimeout != drainTimeout {
		t.Errorf("Drain timeout = %v, want %v", sm.drainTimeout, drainTimeout)
	}
}

func TestHelperMethods(t *testing.T) {
	actions := &MockActionProvider{}
	sm := New("test-app", "blue", actions, nil)

	// Test initial state
	if !sm.CanDeploy() {
		t.Errorf("CanDeploy() should be true in stable state")
	}
	if sm.IsInProgress() {
		t.Errorf("IsInProgress() should be false in stable state")
	}
	if sm.NeedsManualIntervention() {
		t.Errorf("NeedsManualIntervention() should be false in stable state")
	}

	// Start deployment
	sm.Deploy("nginx:1.22")

	if sm.CanDeploy() {
		t.Errorf("CanDeploy() should be false during deployment")
	}
	if !sm.IsInProgress() {
		t.Errorf("IsInProgress() should be true during deployment")
	}
	if sm.NeedsManualIntervention() {
		t.Errorf("NeedsManualIntervention() should be false during normal deployment")
	}

	// Force failed state
	sm.state = StateFailed

	if sm.CanDeploy() {
		t.Errorf("CanDeploy() should be false in failed state")
	}
	if sm.IsInProgress() {
		t.Errorf("IsInProgress() should be false in failed state")
	}
	if !sm.NeedsManualIntervention() {
		t.Errorf("NeedsManualIntervention() should be true in failed state")
	}
}

func TestGetInactiveColor(t *testing.T) {
	actions := &MockActionProvider{}

	blueActiveSm := New("test-app", "blue", actions, nil)
	if blueActiveSm.getInactiveColor() != "green" {
		t.Errorf("Inactive color for blue active = %v, want green", blueActiveSm.getInactiveColor())
	}

	greenActiveSm := New("test-app", "green", actions, nil)
	if greenActiveSm.getInactiveColor() != "blue" {
		t.Errorf("Inactive color for green active = %v, want blue", greenActiveSm.getInactiveColor())
	}
}
