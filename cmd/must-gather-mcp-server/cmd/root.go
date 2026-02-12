package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	// Import toolsets to register them
	_ "github.com/openshift/must-gather-mcp-server/pkg/toolsets/cluster"
	_ "github.com/openshift/must-gather-mcp-server/pkg/toolsets/core"
	_ "github.com/openshift/must-gather-mcp-server/pkg/toolsets/diagnostics"
	_ "github.com/openshift/must-gather-mcp-server/pkg/toolsets/network"

	"github.com/openshift/must-gather-mcp-server/pkg/mcp"
	"github.com/openshift/must-gather-mcp-server/pkg/mustgather"
	"github.com/openshift/must-gather-mcp-server/pkg/toolsets"
	"github.com/openshift/must-gather-mcp-server/pkg/version"
)

var (
	mustGatherPath string
	showVersion    bool
	httpMode       bool
	httpAddr       string
)

var rootCmd = &cobra.Command{
	Use:   "must-gather-mcp-server",
	Short: "MCP server for analyzing OpenShift must-gather data",
	Long: `A Model Context Protocol (MCP) server that provides AI assistants
with the ability to analyze OpenShift must-gather data for troubleshooting
and diagnostics.`,
	RunE: run,
}

func init() {
	rootCmd.Flags().StringVar(&mustGatherPath, "must-gather-path", "", "Path to must-gather directory (required)")
	rootCmd.Flags().BoolVar(&showVersion, "version", false, "Show version information")
	rootCmd.Flags().BoolVar(&httpMode, "http", false, "Run in HTTP/SSE mode instead of STDIO")
	rootCmd.Flags().StringVar(&httpAddr, "http-addr", "localhost:8080", "HTTP server address (only used with --http)")
	rootCmd.MarkFlagRequired("must-gather-path")
}

func Execute() error {
	return rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) error {
	// Show version if requested
	if showVersion {
		fmt.Println(version.Info())
		return nil
	}

	// Verify must-gather path
	if mustGatherPath == "" {
		return fmt.Errorf("must-gather-path is required")
	}

	if _, err := os.Stat(mustGatherPath); os.IsNotExist(err) {
		return fmt.Errorf("must-gather path does not exist: %s", mustGatherPath)
	}

	// Create must-gather provider
	provider, err := mustgather.NewProvider(mustGatherPath)
	if err != nil {
		return fmt.Errorf("failed to create must-gather provider: %w", err)
	}

	// Get all registered toolsets
	allToolsets := toolsets.All()
	if len(allToolsets) == 0 {
		return fmt.Errorf("no toolsets registered")
	}

	fmt.Printf("Registered %d toolsets\n", len(allToolsets))

	// Create MCP server
	server, err := mcp.NewServer(provider, allToolsets)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	// Start server with appropriate transport
	ctx := cmd.Context()

	if httpMode {
		fmt.Printf("Starting must-gather MCP server in HTTP/SSE mode...\n")
		if err := server.ServeHTTP(ctx, httpAddr); err != nil {
			return fmt.Errorf("failed to start MCP server: %w", err)
		}
	} else {
		fmt.Printf("Starting must-gather MCP server in STDIO mode...\n")
		if err := server.ServeStdio(ctx); err != nil {
			return fmt.Errorf("failed to start MCP server: %w", err)
		}
	}

	return nil
}
