package main

import (
	"fmt"
	"os"

	"github.com/openshift/must-gather-mcp-server/cmd/must-gather-mcp-server/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
