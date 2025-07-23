package main

import (
	"dockswap/internal/cli"
	"fmt"
	"os"
)

func main() {
	c := cli.New()
	
	if err := c.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}