package main

import (
	"dockswap/internal/cli"
	"dockswap/internal/state"
	"fmt"
	"os"
)

func main() {
	// Parse --config from os.Args
	flags := cli.GlobalFlags{}
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--config" && i+1 < len(os.Args) {
			flags.Config = os.Args[i+1]
			i++
		} else if len(os.Args[i]) > 9 && os.Args[i][:9] == "--config=" {
			flags.Config = os.Args[i][9:]
		}
	}

	configDir, err := cli.FindConfigDir(flags, nil, nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config dir error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[info] Using config dir: %s\n", configDir)

	dbPath := configDir + "/dockswap.db"
	db, err := state.OpenAndMigrate(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	c := cli.New(db)

	if err := c.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
