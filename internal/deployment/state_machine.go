package deployment

import (
	"database/sql"
	"dockswap/internal/state"
	"fmt"
	"time"
)

type DeploymentState string

const (
	StateStable      DeploymentState = "stable"
	StateStarting    DeploymentState = "starting"
	StateHealthCheck DeploymentState = "health_check"
	StateSwitching   DeploymentState = "switching"
	StateDraining    DeploymentState = "draining"
	StateStopping    DeploymentState = "stopping"
	StateRollingBack DeploymentState = "rolling_back"
	StateFailed      DeploymentState = "failed"
)

type DeploymentEvent string

const (
	EventDeploy            DeploymentEvent = "deploy"
	EventContainerStarted  DeploymentEvent = "container_started"
	EventContainerFailed   DeploymentEvent = "container_failed"
	EventHealthCheckPassed DeploymentEvent = "health_check_passed"
	EventHealthCheckFailed DeploymentEvent = "health_check_failed"
	EventCaddyUpdated      DeploymentEvent = "caddy_updated"
	EventCaddyFailed       DeploymentEvent = "caddy_failed"
	EventDrainComplete     DeploymentEvent = "drain_complete"
	EventContainerStopped  DeploymentEvent = "container_stopped"
	EventStopFailed        DeploymentEvent = "stop_failed"
	EventRollbackComplete  DeploymentEvent = "rollback_complete"
	EventRollbackFailed    DeploymentEvent = "rollback_failed"
	EventManualRecovery    DeploymentEvent = "manual_recovery"
)

type ActionProvider interface {
	StartContainer(appName, color, image string) error
	CheckHealth(appName, color string) (bool, error)
	UpdateCaddy(appName, activeColor string) error
	DrainConnections(appName, color string, timeout time.Duration) error
	StopContainer(appName, color string) error
	RollbackCaddy(appName, activeColor string) error
}

type DeploymentStateMachine struct {
	state         DeploymentState
	appName       string
	newImage      string
	activeColor   string
	targetColor   string
	previousColor string
	actions       ActionProvider

	healthCheckTimeout time.Duration
	drainTimeout       time.Duration

	stateHistory []StateTransition

	db *sql.DB // NEW: DB handle for persistence

	deploymentID int64 // NEW: Track current deployment row
}

type StateTransition struct {
	FromState DeploymentState
	ToState   DeploymentState
	Event     DeploymentEvent
	Timestamp time.Time
	Error     error
}

// New creates a new state machine with DB persistence.
func New(appName, activeColor string, actions ActionProvider, db *sql.DB) *DeploymentStateMachine {
	return &DeploymentStateMachine{
		state:              StateStable,
		appName:            appName,
		activeColor:        activeColor,
		actions:            actions,
		healthCheckTimeout: 60 * time.Second,
		drainTimeout:       30 * time.Second,
		stateHistory:       make([]StateTransition, 0),
		db:                 db,
	}
}

func (sm *DeploymentStateMachine) GetState() DeploymentState {
	return sm.state
}

func (sm *DeploymentStateMachine) GetStateHistory() []StateTransition {
	return sm.stateHistory
}

func (sm *DeploymentStateMachine) SetHealthCheckTimeout(timeout time.Duration) {
	sm.healthCheckTimeout = timeout
}

func (sm *DeploymentStateMachine) SetDrainTimeout(timeout time.Duration) {
	sm.drainTimeout = timeout
}

func (sm *DeploymentStateMachine) ProcessEvent(event DeploymentEvent) error {
	oldState := sm.state
	var err error

	switch sm.state {
	case StateStable:
		err = sm.handleStableState(event)
	case StateStarting:
		err = sm.handleStartingState(event)
	case StateHealthCheck:
		err = sm.handleHealthCheckState(event)
	case StateSwitching:
		err = sm.handleSwitchingState(event)
	case StateDraining:
		err = sm.handleDrainingState(event)
	case StateStopping:
		err = sm.handleStoppingState(event)
	case StateRollingBack:
		err = sm.handleRollingBackState(event)
	case StateFailed:
		err = sm.handleFailedState(event)
	default:
		return fmt.Errorf("unknown state: %s", sm.state)
	}

	sm.recordTransition(oldState, sm.state, event, err)
	return err
}

func (sm *DeploymentStateMachine) Deploy(newImage string) error {
	if sm.state != StateStable {
		return fmt.Errorf("cannot deploy in state %s, must be in stable state", sm.state)
	}

	sm.newImage = newImage
	sm.targetColor = sm.getInactiveColor()
	sm.previousColor = sm.activeColor

	// --- DB: Insert config if new, then deployment ---
	if sm.db != nil {
		cfg, err := state.GetLatestAppConfig(sm.db, sm.appName)
		if err != nil || cfg == nil || cfg.ConfigYAML != newImage {
			// For now, treat newImage as config YAML (stub)
			_, err := state.InsertAppConfig(sm.db, sm.appName, newImage, "sha-stub")
			if err != nil {
				return fmt.Errorf("failed to insert app config: %w", err)
			}
		}
		cfg, _ = state.GetLatestAppConfig(sm.db, sm.appName)
		depID, err := state.InsertDeployment(sm.db, sm.appName, cfg.ID, newImage, "deploying", sm.targetColor, nil)
		if err != nil {
			return fmt.Errorf("failed to insert deployment: %w", err)
		}
		sm.deploymentID = depID
	}

	return sm.ProcessEvent(EventDeploy)
}

func (sm *DeploymentStateMachine) handleStableState(event DeploymentEvent) error {
	switch event {
	case EventDeploy:
		sm.state = StateStarting
		return sm.actions.StartContainer(sm.appName, sm.targetColor, sm.newImage)
	case EventManualRecovery:
		// Already stable, no-op
		return nil
	default:
		return fmt.Errorf("invalid event %s for state %s", event, sm.state)
	}
}

func (sm *DeploymentStateMachine) handleStartingState(event DeploymentEvent) error {
	switch event {
	case EventContainerStarted:
		sm.state = StateHealthCheck
		return nil
	case EventContainerFailed:
		sm.state = StateFailed
		return fmt.Errorf("container failed to start")
	default:
		return fmt.Errorf("invalid event %s for state %s", event, sm.state)
	}
}

func (sm *DeploymentStateMachine) handleHealthCheckState(event DeploymentEvent) error {
	switch event {
	case EventHealthCheckPassed:
		sm.state = StateSwitching
		return sm.actions.UpdateCaddy(sm.appName, sm.targetColor)
	case EventHealthCheckFailed:
		sm.state = StateRollingBack
		return sm.actions.StopContainer(sm.appName, sm.targetColor)
	default:
		return fmt.Errorf("invalid event %s for state %s", event, sm.state)
	}
}

func (sm *DeploymentStateMachine) handleSwitchingState(event DeploymentEvent) error {
	switch event {
	case EventCaddyUpdated:
		sm.state = StateDraining
		return sm.actions.DrainConnections(sm.appName, sm.previousColor, sm.drainTimeout)
	case EventCaddyFailed:
		sm.state = StateRollingBack
		return sm.actions.StopContainer(sm.appName, sm.targetColor)
	default:
		return fmt.Errorf("invalid event %s for state %s", event, sm.state)
	}
}

func (sm *DeploymentStateMachine) handleDrainingState(event DeploymentEvent) error {
	switch event {
	case EventDrainComplete:
		sm.state = StateStopping
		return sm.actions.StopContainer(sm.appName, sm.previousColor)
	default:
		return fmt.Errorf("invalid event %s for state %s", event, sm.state)
	}
}

func (sm *DeploymentStateMachine) handleStoppingState(event DeploymentEvent) error {
	switch event {
	case EventContainerStopped:
		sm.state = StateStable
		sm.activeColor = sm.targetColor
		return nil
	case EventStopFailed:
		sm.state = StateFailed
		return fmt.Errorf("failed to stop old container")
	default:
		return fmt.Errorf("invalid event %s for state %s", event, sm.state)
	}
}

func (sm *DeploymentStateMachine) handleRollingBackState(event DeploymentEvent) error {
	switch event {
	case EventRollbackComplete:
		sm.state = StateStable
		return nil
	case EventRollbackFailed:
		sm.state = StateFailed
		return fmt.Errorf("rollback failed")
	default:
		return fmt.Errorf("invalid event %s for state %s", event, sm.state)
	}
}

func (sm *DeploymentStateMachine) handleFailedState(event DeploymentEvent) error {
	switch event {
	case EventManualRecovery:
		sm.state = StateStable
		return nil
	default:
		return fmt.Errorf("invalid event %s for state %s", event, sm.state)
	}
}

func (sm *DeploymentStateMachine) CheckHealth() error {
	if sm.state != StateHealthCheck {
		return fmt.Errorf("cannot check health in state %s", sm.state)
	}

	healthy, err := sm.actions.CheckHealth(sm.appName, sm.targetColor)
	if err != nil {
		return sm.ProcessEvent(EventHealthCheckFailed)
	}

	if healthy {
		return sm.ProcessEvent(EventHealthCheckPassed)
	}

	return nil // Still checking
}

func (sm *DeploymentStateMachine) CompleteHealthCheck(passed bool) error {
	if passed {
		return sm.ProcessEvent(EventHealthCheckPassed)
	} else {
		return sm.ProcessEvent(EventHealthCheckFailed)
	}
}

func (sm *DeploymentStateMachine) CompleteDrain() error {
	return sm.ProcessEvent(EventDrainComplete)
}

func (sm *DeploymentStateMachine) CompleteContainerOperation(success bool, isRollback bool) error {
	if isRollback {
		if success {
			return sm.ProcessEvent(EventRollbackComplete)
		} else {
			return sm.ProcessEvent(EventRollbackFailed)
		}
	}

	switch sm.state {
	case StateStarting:
		if success {
			return sm.ProcessEvent(EventContainerStarted)
		} else {
			return sm.ProcessEvent(EventContainerFailed)
		}
	case StateStopping:
		if success {
			return sm.ProcessEvent(EventContainerStopped)
		} else {
			return sm.ProcessEvent(EventStopFailed)
		}
	default:
		return fmt.Errorf("unexpected container operation completion in state %s", sm.state)
	}
}

func (sm *DeploymentStateMachine) CompleteCaddyUpdate(success bool) error {
	if success {
		return sm.ProcessEvent(EventCaddyUpdated)
	} else {
		return sm.ProcessEvent(EventCaddyFailed)
	}
}

func (sm *DeploymentStateMachine) RecoverManually() error {
	return sm.ProcessEvent(EventManualRecovery)
}

func (sm *DeploymentStateMachine) getInactiveColor() string {
	if sm.activeColor == "blue" {
		return "green"
	}
	return "blue"
}

func (sm *DeploymentStateMachine) recordTransition(fromState, toState DeploymentState, event DeploymentEvent, err error) {
	transition := StateTransition{
		FromState: fromState,
		ToState:   toState,
		Event:     event,
		Timestamp: time.Now(),
		Error:     err,
	}
	sm.stateHistory = append(sm.stateHistory, transition)

	// --- DB: Persist event and update current state ---
	if sm.db != nil && sm.deploymentID != 0 {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		_, _ = state.InsertDeploymentEvent(sm.db, sm.deploymentID, sm.appName, string(event), "{}", &errMsg)
		_ = state.UpsertCurrentState(sm.db, sm.appName, sm.deploymentID, sm.activeColor, sm.newImage, string(toState))
	}
}

func (sm *DeploymentStateMachine) GetActiveColor() string {
	return sm.activeColor
}

func (sm *DeploymentStateMachine) GetTargetColor() string {
	return sm.targetColor
}

func (sm *DeploymentStateMachine) GetNewImage() string {
	return sm.newImage
}

func (sm *DeploymentStateMachine) IsInProgress() bool {
	return sm.state != StateStable && sm.state != StateFailed
}

func (sm *DeploymentStateMachine) CanDeploy() bool {
	return sm.state == StateStable
}

func (sm *DeploymentStateMachine) NeedsManualIntervention() bool {
	return sm.state == StateFailed
}
