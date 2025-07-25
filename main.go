package main

import (
	"dockswap/internal/cli"
	"dockswap/internal/config"
	"dockswap/internal/logger"
	"dockswap/internal/state"
	"os"
	"strconv"
	"strings"
)

func main() {
	// Parse --config and --log-level from os.Args
	flags := cli.GlobalFlags{
		LogLevel: logger.LevelInfo, // Default to info level
	}
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--config" && i+1 < len(os.Args) {
			flags.Config = os.Args[i+1]
			i++
		} else if len(os.Args[i]) > 9 && os.Args[i][:9] == "--config=" {
			flags.Config = os.Args[i][9:]
		} else if os.Args[i] == "--log-level" && i+1 < len(os.Args) {
			if level, err := strconv.Atoi(os.Args[i+1]); err == nil && level >= 1 && level <= 3 {
				flags.LogLevel = level
			}
			i++
		} else if strings.HasPrefix(os.Args[i], "--log-level=") {
			levelStr := strings.TrimPrefix(os.Args[i], "--log-level=")
			if level, err := strconv.Atoi(levelStr); err == nil && level >= 1 && level <= 3 {
				flags.LogLevel = level
			}
		}
	}

	// Initialize logger with the parsed log level
	log := logger.New(flags.LogLevel)

	configDir, err := cli.FindConfigDir(flags, nil, nil, nil)
	if err != nil {
		log.Error("Config dir error: %v", err)
		os.Exit(1)
	}
	log.Info("Using config dir: %s", configDir)

	if err := config.ValidateAndPrepareConfigDir(configDir); err != nil {
		log.Error("Config validation error: %v", err)
		os.Exit(1)
	}

	dbPath := configDir + "/dockswap.db"
	db, err := state.OpenAndMigrate(dbPath)
	if err != nil {
		log.Error("Failed to open DB: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	c := cli.New(db, log)

	// Load app configurations
	if err := c.LoadConfigs(configDir); err != nil {
		log.Error("Failed to load configs: %v", err)
		os.Exit(1)
	}

	if err := c.Run(os.Args[1:]); err != nil {
		log.Error("Error: %v", err)
		os.Exit(1)
	}
}
