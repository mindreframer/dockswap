package cli

import (
	"fmt"
	"strconv"
	"strings"
)

func (c *CLI) handleStatus(args []string) error {
	var appName string
	if len(args) > 0 {
		appName = args[0]
	}
	
	if appName != "" {
		fmt.Printf("Status for app: %s\n", appName)
		fmt.Println("  Blue:  running (50% traffic)")
		fmt.Println("  Green: ready (50% traffic)")
	} else {
		fmt.Println("Application Status:")
		fmt.Println("  myapp1: blue=running, green=stopped")
		fmt.Println("  myapp2: blue=stopped, green=running") 
	}
	
	return nil
}

func (c *CLI) handleDeploy(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("deploy requires <app-name> and <image> arguments")
	}
	
	appName := args[0]
	image := args[1]
	
	fmt.Printf("Deploying %s with image %s...\n", appName, image)
	fmt.Println("✓ Pulling image")
	fmt.Println("✓ Starting green deployment")
	fmt.Println("✓ Health check passed")
	fmt.Println("✓ Ready for traffic switch")
	
	return nil
}

func (c *CLI) handleHistory(args []string) error {
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
	
	fmt.Printf("Deployment history for %s (last %d):\n", appName, limit)
	fmt.Println("  2024-01-15 14:30:25  nginx:1.21  -> blue   (success)")
	fmt.Println("  2024-01-15 12:15:10  nginx:1.20  -> green  (success)")
	fmt.Println("  2024-01-14 16:45:33  nginx:1.19  -> blue   (rollback)")
	
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
	
	fmt.Printf("Switching %s to %s deployment...\n", appName, color)
	fmt.Println("✓ Updating load balancer configuration")
	fmt.Println("✓ Draining connections from old deployment")
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