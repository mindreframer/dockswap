package cli

import (
	"database/sql"
	"dockswap/internal/caddy"
	"dockswap/internal/config"
	"dockswap/internal/logger"
	"fmt"
	"strconv"
	"strings"
)

var (
	Version = "dev"
	commit  = "none"
	date    = "unknown"
)

type GlobalFlags struct {
	Config   string
	LogLevel int
}

type CLI struct {
	flags    GlobalFlags
	DB       *sql.DB // Add DB handle for inspection commands
	logger   logger.Logger
	configs  map[string]*config.AppConfig
	caddyMgr *caddy.CaddyManager
}

// New creates a CLI with a DB handle and logger.
func New(db *sql.DB, log logger.Logger) *CLI {
	return &CLI{
		DB:       db,
		logger:   log,
		configs:  make(map[string]*config.AppConfig),
		caddyMgr: nil, // Will be initialized when configs are loaded
	}
}

// LoadConfigs loads all app configurations from the specified directory
func (c *CLI) LoadConfigs(configDir string) error {
	appsDir := configDir + "/apps"
	configs, err := config.LoadAllConfigs(appsDir)
	if err != nil {
		return fmt.Errorf("failed to load app configs: %w", err)
	}
	c.configs = configs

	// Initialize Caddy manager if we have configs
	if len(configs) > 0 {
		caddyConfigPath := configDir + "/caddy/caddy.json"
		caddyTemplatePath := configDir + "/caddy/template.json"
		c.caddyMgr = caddy.New(caddyConfigPath, caddyTemplatePath)
	}

	return nil
}

func (c *CLI) parseGlobalFlags(args []string) ([]string, error) {
	var filteredArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if strings.HasPrefix(arg, "--config=") {
			c.flags.Config = strings.TrimPrefix(arg, "--config=")
		} else if arg == "--config" && i+1 < len(args) {
			i++
			c.flags.Config = args[i]
		} else if strings.HasPrefix(arg, "--log-level=") {
			levelStr := strings.TrimPrefix(arg, "--log-level=")
			level, err := strconv.Atoi(levelStr)
			if err != nil || level < 1 || level > 3 {
				return nil, fmt.Errorf("invalid log level: %s (must be 1, 2, or 3)", levelStr)
			}
			c.flags.LogLevel = level
		} else if arg == "--log-level" && i+1 < len(args) {
			i++
			level, err := strconv.Atoi(args[i])
			if err != nil || level < 1 || level > 3 {
				return nil, fmt.Errorf("invalid log level: %s (must be 1, 2, or 3)", args[i])
			}
			c.flags.LogLevel = level
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	return filteredArgs, nil
}

func (c *CLI) Run(args []string) error {
	if len(args) == 0 {
		c.printHelp()
		return nil
	}

	filteredArgs, err := c.parseGlobalFlags(args)
	if err != nil {
		return err
	}

	if len(filteredArgs) == 0 {
		c.printHelp()
		return nil
	}

	command := filteredArgs[0]
	commandArgs := filteredArgs[1:]

	switch command {
	case "status":
		return c.handleStatus(commandArgs)
	case "deploy":
		return c.handleDeploy(commandArgs)
	case "history":
		return c.handleHistory(commandArgs)
	case "events":
		return c.handleEvents(commandArgs)
	case "health":
		return c.handleHealth(commandArgs)
	case "switch":
		return c.handleSwitch(commandArgs)
	case "logs":
		return c.handleLogs(commandArgs)
	case "config":
		return c.handleConfig(commandArgs)
	case "caddy":
		return c.handleCaddy(commandArgs)
	case "dbg-cmd":
		return c.handleDbgCmd(commandArgs)
	case "version":
		return c.handleVersion(commandArgs)
	case "help", "-h", "--help":
		c.printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (c *CLI) printHelp() {
	fmt.Print(`dockswap - Docker container blue-green deployment tool

Usage:
  dockswap <command> [arguments] [flags]

Commands:
  status [app-name]               Show deployment status for all apps or specific app
  deploy <app-name> <image>       Deploy new image for application
  history <app-name> [--limit N]  Show deployment history for application
  health <app-name>               Check health status of application
  switch <app-name> <color>       Switch traffic to blue or green deployment
  logs <app-name> [--follow]      Show logs for application
  config reload [app-name]        Reload configuration for all apps or specific app
  caddy status                    Show Caddy proxy status
  caddy reload                    Reload Caddy configuration
  caddy config create             Create default Caddy template
  caddy config show               Show Caddy configuration paths
  dbg-cmd <app-name> [--color]    Show equivalent docker run command for debugging
  version                         Show version information
  help                           Show this help message

Global Flags:
  --config <path>                Configuration file path
  --log-level <level>            Log level (1=error, 2=info, 3=debug)

Examples:
  dockswap status                 # Show all app statuses
  dockswap deploy myapp nginx:1.21
  dockswap switch myapp blue
  dockswap logs myapp --follow
  dockswap caddy status           # Check Caddy proxy status
  dockswap caddy reload           # Reload Caddy configuration
  dockswap dbg-cmd myapp          # Show docker command for active container
  dockswap dbg-cmd myapp --color blue  # Show docker command for blue container
`)
}
