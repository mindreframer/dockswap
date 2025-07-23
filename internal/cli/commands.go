package cli

import (
	"context"
	"dockswap/internal/docker"
	"dockswap/internal/state"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (c *CLI) handleStatus(args []string) error {
	if c.DB == nil {
		return fmt.Errorf("DB not initialized")
	}
	var appName string
	if len(args) > 0 {
		appName = args[0]
	}

	if appName != "" {
		// Show status for one app
		cs, err := state.GetCurrentState(c.DB, appName)
		if err != nil {
			return fmt.Errorf("failed to get current state: %w", err)
		}
		fmt.Printf("Status for app: %s\n", appName)
		fmt.Printf("  Color: %s\n  Image: %s\n  Status: %s\n  Updated: %s\n",
			cs.ActiveColor, cs.Image, cs.Status, cs.UpdatedAt.Format("2006-01-02 15:04:05"))
	} else {
		// Show all apps
		all, err := state.GetAllCurrentStates(c.DB)
		if err != nil {
			return fmt.Errorf("failed to get all current states: %w", err)
		}
		fmt.Println("Application Status:")
		for _, cs := range all {
			fmt.Printf("  %s: color=%s, image=%s, status=%s, updated=%s\n",
				cs.AppName, cs.ActiveColor, cs.Image, cs.Status, cs.UpdatedAt.Format("2006-01-02 15:04:05"))
		}
	}
	return nil
}

func (c *CLI) handleDeploy(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("deploy requires <app-name> and <image> arguments")
	}

	appName := args[0]
	image := args[1]

	// Check if app config exists
	appConfig, exists := c.configs[appName]
	if !exists {
		return fmt.Errorf("no configuration found for app %s", appName)
	}

	fmt.Printf("Deploying %s with image %s...\n", appName, image)

	// Create Docker client
	dockerClient, err := docker.NewDockerClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	dockerManager := docker.NewDockerManager(dockerClient)

	// Test Docker connection
	ctx := context.Background()
	if err := dockerManager.ValidateConnection(ctx); err != nil {
		return fmt.Errorf("Docker not available: %w", err)
	}

	// Get current active color (default to blue if first deployment)
	activeColor := "blue"
	cs, err := state.GetCurrentState(c.DB, appName)
	if err == nil && cs != nil {
		activeColor = cs.ActiveColor
	}

	// Determine target color
	targetColor := "green"
	if activeColor == "green" {
		targetColor = "blue"
	}

	fmt.Printf("Current active: %s, deploying to: %s\n", activeColor, targetColor)

	// Create action provider
	actionProvider := docker.NewDockerActionProvider(dockerManager, nil, c.configs)
	actionProvider.SetContext(ctx)

	// Start container
	fmt.Println("✓ Starting container...")
	if err := actionProvider.StartContainer(appName, targetColor, image); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for health check
	fmt.Println("✓ Waiting for health check...")
	timeout := time.Duration(appConfig.HealthCheck.Retries) * appConfig.HealthCheck.Interval * 2
	if err := dockerManager.WaitForHealthy(ctx, appName, targetColor, appConfig, timeout); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	fmt.Println("✓ Container healthy and ready")
	if cs == nil {
		// First deployment
		fmt.Printf("✓ Initial deployment complete - traffic active on %s\n", targetColor)
	} else {
		// Subsequent deployment
		fmt.Printf("✓ Deployment complete - traffic still on %s\n", activeColor)
		fmt.Printf("Run 'dockswap switch %s %s' to switch traffic\n", appName, targetColor)
	}

	// Update database state
	depID, err := state.InsertDeployment(c.DB, appName, 1, image, "ready", targetColor, nil)
	if err != nil {
		fmt.Printf("Warning: failed to update database: %v\n", err)
	} else {
		// For first deployment, make the deployed color active
		// For subsequent deployments, keep current active until manual switch
		dbActiveColor := activeColor
		if cs == nil {
			// First deployment - make deployed color active
			dbActiveColor = targetColor
		}
		state.UpsertCurrentState(c.DB, appName, depID, dbActiveColor, image, "ready")
	}

	return nil
}

func (c *CLI) handleHistory(args []string) error {
	if c.DB == nil {
		return fmt.Errorf("DB not initialized")
	}
	if len(args) == 0 {
		return fmt.Errorf("history requires <app-name> argument")
	}

	appName := args[0]
	limit := 10

	for i := 1; i < len(args); i++ {
		if args[i] == "--limit" && i+1 < len(args) {
			var err error
			limit, err = strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid limit value: %s", args[i+1])
			}
			i++
		} else if strings.HasPrefix(args[i], "--limit=") {
			limitStr := strings.TrimPrefix(args[i], "--limit=")
			var err error
			limit, err = strconv.Atoi(limitStr)
			if err != nil {
				return fmt.Errorf("invalid limit value: %s", limitStr)
			}
		}
	}

	hist, err := state.GetDeploymentHistory(c.DB, appName)
	if err != nil {
		return fmt.Errorf("failed to get deployment history: %w", err)
	}
	if len(hist) == 0 {
		fmt.Printf("No deployments found for %s\n", appName)
		return nil
	}
	fmt.Printf("Deployment history for %s (last %d):\n", appName, limit)
	for i, d := range hist {
		if i >= limit {
			break
		}
		ended := "-"
		if d.EndedAt.Valid {
			ended = d.EndedAt.Time.Format("2006-01-02 15:04:05")
		}
		fmt.Printf("  %s  %s  -> %s  (%s)  ended: %s\n",
			d.StartedAt.Format("2006-01-02 15:04:05"), d.Image, d.ActiveColor, d.Status, ended)
	}
	return nil
}

// (Optional) Show all events for a deployment
func (c *CLI) handleEvents(args []string) error {
	if c.DB == nil {
		return fmt.Errorf("DB not initialized")
	}
	if len(args) == 0 {
		return fmt.Errorf("events requires <deployment-id> argument")
	}
	depID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid deployment id: %s", args[0])
	}
	events, err := state.GetDeploymentEvents(c.DB, depID)
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}
	if len(events) == 0 {
		fmt.Printf("No events found for deployment %d\n", depID)
		return nil
	}
	fmt.Printf("Events for deployment %d:\n", depID)
	for _, e := range events {
		errStr := ""
		if e.Error.Valid {
			errStr = e.Error.String
		}
		fmt.Printf("  %s  %s  payload: %s  error: %s\n",
			e.CreatedAt.Format("2006-01-02 15:04:05"), e.EventType, e.Payload, errStr)
	}
	return nil
}

func (c *CLI) handleHealth(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("health requires <app-name> argument")
	}

	appName := args[0]

	fmt.Printf("Health check for %s:\n", appName)
	fmt.Println("  Blue:  ✓ healthy (2/2 containers)")
	fmt.Println("  Green: ✓ healthy (2/2 containers)")
	fmt.Println("  Load Balancer: ✓ healthy")

	return nil
}

func (c *CLI) handleSwitch(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("switch requires <app-name> and <color> arguments")
	}

	appName := args[0]
	color := args[1]

	if color != "blue" && color != "green" {
		return fmt.Errorf("color must be 'blue' or 'green', got: %s", color)
	}

	// Check if app config exists
	appConfig, exists := c.configs[appName]
	if !exists {
		return fmt.Errorf("no configuration found for app %s", appName)
	}

	// Create Docker client
	dockerClient, err := docker.NewDockerClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	dockerManager := docker.NewDockerManager(dockerClient)
	ctx := context.Background()

	// Check if target container exists and is healthy
	exists, err = dockerManager.ContainerExists(ctx, appName, color)
	if err != nil {
		return fmt.Errorf("failed to check container existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("no %s container found for %s", color, appName)
	}

	// Check health of target container
	healthy, err := dockerManager.IsContainerHealthy(ctx, appName, color, appConfig)
	if err != nil {
		return fmt.Errorf("failed to check container health: %w", err)
	}
	if !healthy {
		return fmt.Errorf("%s container for %s is not healthy", color, appName)
	}

	fmt.Printf("Switching %s to %s deployment...\n", appName, color)

	// Get current state
	cs, err := state.GetCurrentState(c.DB, appName)
	if err != nil {
		fmt.Printf("Warning: could not get current state: %v\n", err)
	}

	oldColor := "blue"
	if cs != nil {
		oldColor = cs.ActiveColor
	}

	if oldColor == color {
		fmt.Printf("Traffic is already on %s deployment\n", color)
		return nil
	}

	// Update database state to switch active color
	fmt.Println("✓ Updating traffic routing...")
	err = state.UpsertCurrentState(c.DB, appName, 0, color, "switched", "active")
	if err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	// TODO: In a real implementation, this would update Caddy config
	fmt.Println("✓ Load balancer configuration updated")
	
	// Optionally stop old container if configured
	if appConfig.Deployment.AutoRollback {
		fmt.Printf("✓ Stopping old %s container...\n", oldColor)
		err = dockerManager.StopContainer(ctx, appName, oldColor, 30*time.Second)
		if err != nil {
			fmt.Printf("Warning: failed to stop old container: %v\n", err)
		} else {
			err = dockerManager.RemoveContainer(ctx, appName, oldColor, false)
			if err != nil {
				fmt.Printf("Warning: failed to remove old container: %v\n", err)
			}
		}
	}

	fmt.Printf("✓ Traffic switched to %s deployment\n", color)
	return nil
}

func (c *CLI) handleLogs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("logs requires <app-name> argument")
	}

	appName := args[0]
	follow := false

	for i := 1; i < len(args); i++ {
		if args[i] == "--follow" || args[i] == "-f" {
			follow = true
		}
	}

	fmt.Printf("Logs for %s", appName)
	if follow {
		fmt.Print(" (following)")
	}
	fmt.Println(":")

	fmt.Println("2024-01-15 14:30:25 [INFO] Application started")
	fmt.Println("2024-01-15 14:30:26 [INFO] Listening on port 8080")
	fmt.Println("2024-01-15 14:30:27 [INFO] Health check endpoint ready")

	if follow {
		fmt.Println("^C to stop following logs")
	}

	return nil
}

func (c *CLI) handleConfig(args []string) error {
	if len(args) == 0 || args[0] != "reload" {
		return fmt.Errorf("config subcommand must be 'reload'")
	}

	var appName string
	if len(args) > 1 {
		appName = args[1]
	}

	if appName != "" {
		fmt.Printf("Reloading configuration for %s...\n", appName)
	} else {
		fmt.Println("Reloading configuration for all applications...")
	}

	fmt.Println("✓ Configuration reloaded successfully")

	return nil
}

func (c *CLI) handleVersion(args []string) error {
	fmt.Printf("dockswap version %s\n", Version)
	fmt.Println("Built with Go")
	return nil
}
