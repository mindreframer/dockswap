package docker

import (
	"context"
	"fmt"
	"time"

	"dockswap/internal/caddy"
	"dockswap/internal/config"
	"dockswap/internal/deployment"
)

// DockerActionProvider implements the deployment.ActionProvider interface
type DockerActionProvider struct {
	dockerManager *DockerManager
	caddyManager  caddy.CaddyManagerInterface
	configs       map[string]*config.AppConfig
	ctx           context.Context
}

func NewDockerActionProvider(dockerManager *DockerManager, caddyManager caddy.CaddyManagerInterface, configs map[string]*config.AppConfig) *DockerActionProvider {
	return &DockerActionProvider{
		dockerManager: dockerManager,
		caddyManager:  caddyManager,
		configs:       configs,
		ctx:           context.Background(),
	}
}

func (dap *DockerActionProvider) SetContext(ctx context.Context) {
	dap.ctx = ctx
}

func (dap *DockerActionProvider) StartContainer(appName, color, image string) error {
	appConfig, exists := dap.configs[appName]
	if !exists {
		return fmt.Errorf("no configuration found for app %s", appName)
	}

	// Ensure network exists if configured
	if appConfig.Docker.Network != "" {
		_, err := dap.dockerManager.EnsureNetwork(dap.ctx, appConfig.Docker.Network)
		if err != nil {
			return fmt.Errorf("failed to ensure network %s: %w", appConfig.Docker.Network, err)
		}
	}

	// Check if container already exists
	exists, err := dap.dockerManager.ContainerExists(dap.ctx, appName, color)
	if err != nil {
		return fmt.Errorf("failed to check container existence: %w", err)
	}

	if exists {
		// Remove existing container
		err = dap.dockerManager.RemoveContainer(dap.ctx, appName, color, true)
		if err != nil {
			return fmt.Errorf("failed to remove existing container: %w", err)
		}
	}

	// Create new container
	containerInfo, err := dap.dockerManager.CreateContainer(dap.ctx, appName, color, image, appConfig)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	err = dap.dockerManager.StartContainer(dap.ctx, containerInfo.ID)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Connect to network if configured
	if appConfig.Docker.Network != "" {
		err = dap.dockerManager.ConnectContainerToNetwork(dap.ctx, appConfig.Docker.Network, containerInfo.ID)
		if err != nil {
			return fmt.Errorf("failed to connect container to network: %w", err)
		}
	}

	return nil
}

func (dap *DockerActionProvider) CheckHealth(appName, color string) (bool, error) {
	appConfig, exists := dap.configs[appName]
	if !exists {
		return false, fmt.Errorf("no configuration found for app %s", appName)
	}

	return dap.dockerManager.IsContainerHealthy(dap.ctx, appName, color, appConfig)
}

func (dap *DockerActionProvider) UpdateCaddy(appName, activeColor string) error {
	if dap.caddyManager == nil {
		return fmt.Errorf("caddy manager not available")
	}

	// Update states to reflect new active color (this would normally be done by the caller)
	// For now, we'll regenerate config with current states
	return dap.caddyManager.ReloadCaddy()
}

func (dap *DockerActionProvider) DrainConnections(appName, color string, timeout time.Duration) error {
	// In a real implementation, this would:
	// 1. Check for active connections to the container
	// 2. Wait for connections to naturally close
	// 3. Force close remaining connections after timeout

	// For now, we'll simulate the drain timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-dap.ctx.Done():
		return dap.ctx.Err()
	case <-timer.C:
		// Drain timeout reached
		return nil
	}
}

func (dap *DockerActionProvider) StopContainer(appName, color string) error {
	appConfig, exists := dap.configs[appName]
	if !exists {
		return fmt.Errorf("no configuration found for app %s", appName)
	}

	// Stop container with configured timeout
	stopTimeout := appConfig.Deployment.StopTimeout
	if stopTimeout == 0 {
		stopTimeout = 15 * time.Second // Default timeout
	}

	err := dap.dockerManager.StopContainer(dap.ctx, appName, color, stopTimeout)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Remove stopped container
	err = dap.dockerManager.RemoveContainer(dap.ctx, appName, color, false)
	if err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

func (dap *DockerActionProvider) RollbackCaddy(appName, activeColor string) error {
	if dap.caddyManager == nil {
		return fmt.Errorf("caddy manager not available")
	}

	// Rollback caddy config (regenerate with previous state)
	return dap.caddyManager.ReloadCaddy()
}

// DeploymentOrchestrator orchestrates the entire deployment process
type DeploymentOrchestrator struct {
	dockerManager *DockerManager
	caddyManager  *caddy.CaddyManager
	configs       map[string]*config.AppConfig
	states        map[string]*deployment.DeploymentStateMachine
}

func NewDeploymentOrchestrator(dockerManager *DockerManager, caddyManager *caddy.CaddyManager, configs map[string]*config.AppConfig) *DeploymentOrchestrator {
	return &DeploymentOrchestrator{
		dockerManager: dockerManager,
		caddyManager:  caddyManager,
		configs:       configs,
		states:        make(map[string]*deployment.DeploymentStateMachine),
	}
}

func (do *DeploymentOrchestrator) InitializeApp(appName, activeColor string) error {
	appConfig, exists := do.configs[appName]
	if !exists {
		return fmt.Errorf("no configuration found for app %s", appName)
	}

	// Create action provider for this app
	actionProvider := NewDockerActionProvider(do.dockerManager, do.caddyManager, do.configs)

	// Create state machine
	stateMachine := deployment.New(appName, activeColor, actionProvider, nil)

	// Configure timeouts from app config
	stateMachine.SetHealthCheckTimeout(time.Duration(appConfig.HealthCheck.Retries) * appConfig.HealthCheck.Interval)
	stateMachine.SetDrainTimeout(appConfig.Deployment.DrainTimeout)

	do.states[appName] = stateMachine
	return nil
}

func (do *DeploymentOrchestrator) Deploy(appName, newImage string) error {
	stateMachine, exists := do.states[appName]
	if !exists {
		return fmt.Errorf("app %s not initialized", appName)
	}

	if !stateMachine.CanDeploy() {
		return fmt.Errorf("app %s is not in a deployable state: %s", appName, stateMachine.GetState())
	}

	// Start deployment
	err := stateMachine.Deploy(newImage)
	if err != nil {
		return fmt.Errorf("failed to start deployment: %w", err)
	}

	// Run deployment loop
	return do.runDeploymentLoop(appName)
}

func (do *DeploymentOrchestrator) runDeploymentLoop(appName string) error {
	stateMachine := do.states[appName]
	appConfig := do.configs[appName]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("deployment timeout for app %s", appName)
		case <-ticker.C:
			state := stateMachine.GetState()

			switch state {
			case deployment.StateHealthCheck:
				// Check health and complete if ready
				healthy, err := do.dockerManager.IsContainerHealthy(ctx, appName, stateMachine.GetTargetColor(), appConfig)
				if err != nil {
					stateMachine.CompleteHealthCheck(false)
					continue
				}

				if healthy {
					err = stateMachine.CompleteHealthCheck(true)
					if err != nil {
						return fmt.Errorf("failed to complete health check: %w", err)
					}
				}

			case deployment.StateDraining:
				// Wait for drain timeout, then complete
				// In a real implementation, you'd check for active connections
				time.Sleep(appConfig.Deployment.DrainTimeout)
				err := stateMachine.CompleteDrain()
				if err != nil {
					return fmt.Errorf("failed to complete drain: %w", err)
				}

			case deployment.StateStable:
				// Deployment completed successfully
				return nil

			case deployment.StateFailed:
				// Deployment failed
				return fmt.Errorf("deployment failed for app %s", appName)
			}
		}
	}
}

func (do *DeploymentOrchestrator) GetAppState(appName string) deployment.DeploymentState {
	stateMachine, exists := do.states[appName]
	if !exists {
		return deployment.StateFailed
	}
	return stateMachine.GetState()
}

func (do *DeploymentOrchestrator) GetAppHistory(appName string) []deployment.StateTransition {
	stateMachine, exists := do.states[appName]
	if !exists {
		return nil
	}
	return stateMachine.GetStateHistory()
}

func (do *DeploymentOrchestrator) RecoverApp(appName string) error {
	stateMachine, exists := do.states[appName]
	if !exists {
		return fmt.Errorf("app %s not initialized", appName)
	}

	return stateMachine.RecoverManually()
}
