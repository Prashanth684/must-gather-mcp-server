package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	// Import toolsets to register them
	_ "github.com/openshift/must-gather-mcp-server/pkg/toolsets/cluster"
	_ "github.com/openshift/must-gather-mcp-server/pkg/toolsets/core"
	_ "github.com/openshift/must-gather-mcp-server/pkg/toolsets/diagnostics"
	_ "github.com/openshift/must-gather-mcp-server/pkg/toolsets/monitoring"
	_ "github.com/openshift/must-gather-mcp-server/pkg/toolsets/network"

	"github.com/openshift/must-gather-mcp-server/pkg/config"
	httpserver "github.com/openshift/must-gather-mcp-server/pkg/http"
	"github.com/openshift/must-gather-mcp-server/pkg/mcp"
	"github.com/openshift/must-gather-mcp-server/pkg/mustgather"
	"github.com/openshift/must-gather-mcp-server/pkg/toolsets"
	"github.com/openshift/must-gather-mcp-server/pkg/version"
)

var (
	mustGatherPath   string
	showVersion      bool
	httpMode         bool
	httpAddr         string
	configPath       string
	dropInConfigDir  string
	port             string
	requireOAuth     bool
	oauthAudience    string
	authorizationURL string
	logLevel         int
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
	rootCmd.Flags().StringVar(&httpAddr, "http-addr", "localhost:8080", "HTTP server address (deprecated, use --port instead)")

	// Config file options
	rootCmd.Flags().StringVar(&configPath, "config", "", "Path to configuration file")
	rootCmd.Flags().StringVar(&dropInConfigDir, "config-dir", "", "Path to drop-in configuration directory")

	// Server options (can override config file)
	rootCmd.Flags().StringVar(&port, "port", "", "HTTP server port")
	rootCmd.Flags().IntVar(&logLevel, "log-level", -1, "Log verbosity level (0-9)")

	// OAuth options (can override config file)
	rootCmd.Flags().BoolVar(&requireOAuth, "require-oauth", false, "Require OAuth authentication")
	rootCmd.Flags().StringVar(&oauthAudience, "oauth-audience", "", "OAuth audience for token validation")
	rootCmd.Flags().StringVar(&authorizationURL, "authorization-url", "", "OIDC authorization server URL")

	// Don't mark must-gather-path as required so --version can work without it
	// We'll check for it manually in the run function
}

// Execute is the main entry point for the CLI
func Execute() error {
	return rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) error {
	// Show version if requested
	if showVersion {
		fmt.Println(version.Info())
		return nil
	}

	// Load configuration
	staticConfig, err := loadConfig(cmd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override must-gather path from config with flag if provided
	if mustGatherPath != "" {
		staticConfig.MustGatherPath = mustGatherPath
	}

	// Verify must-gather path
	if staticConfig.MustGatherPath == "" {
		return fmt.Errorf("must-gather-path is required")
	}

	if _, err := os.Stat(staticConfig.MustGatherPath); os.IsNotExist(err) {
		return fmt.Errorf("must-gather path does not exist: %s", staticConfig.MustGatherPath)
	}

	// Set up logging
	if staticConfig.LogLevel > 0 {
		klog.SetLogger(klog.Background().V(staticConfig.LogLevel))
	}

	// Print OAuth configuration if enabled (helpful for debugging)
	if staticConfig.RequireOAuth {
		fmt.Printf("OAuth Authentication: ENABLED\n")
		fmt.Printf("  Audience: %s\n", staticConfig.OAuthAudience)
		if staticConfig.AuthorizationURL != "" {
			fmt.Printf("  OIDC Provider: %s\n", staticConfig.AuthorizationURL)
		} else {
			fmt.Printf("  Mode: Offline validation only\n")
		}
		fmt.Printf("\n")
	}

	klog.V(1).Infof("Starting %s version %s", version.BinaryName, version.Version)
	klog.V(2).Infof("Must-gather path: %s", staticConfig.MustGatherPath)
	klog.V(2).Infof("OAuth required: %v", staticConfig.RequireOAuth)

	// Initialize telemetry if enabled
	shutdownTelemetry, err := staticConfig.Telemetry.InitTelemetry(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry: %w", err)
	}
	defer func() {
		if err := shutdownTelemetry(context.Background()); err != nil {
			klog.Warningf("Failed to shutdown telemetry: %v", err)
		}
	}()

	// Create must-gather provider
	provider, err := mustgather.NewProvider(staticConfig.MustGatherPath)
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

	if httpMode || staticConfig.Port != "" {
		// Initialize OIDC provider if configured
		var oidcProvider *oidc.Provider
		if staticConfig.RequireOAuth && staticConfig.AuthorizationURL != "" {
			klog.V(1).Infof("Initializing OIDC provider: %s", staticConfig.AuthorizationURL)
			provider, err := oidc.NewProvider(ctx, staticConfig.AuthorizationURL)
			if err != nil {
				return fmt.Errorf("failed to initialize OIDC provider: %w", err)
			}
			oidcProvider = provider
			klog.V(1).Info("OIDC provider initialized successfully")
		}

		// Use new HTTP server with authentication support
		fmt.Printf("Starting must-gather MCP server in HTTP/SSE mode on port %s...\n", staticConfig.Port)
		if err := httpserver.Serve(ctx, server, staticConfig, oidcProvider, http.DefaultClient); err != nil {
			return fmt.Errorf("failed to start HTTP server: %w", err)
		}
	} else {
		// STDIO mode
		fmt.Printf("Starting must-gather MCP server in STDIO mode...\n")
		if err := server.ServeStdio(ctx); err != nil {
			return fmt.Errorf("failed to start MCP server: %w", err)
		}
	}

	return nil
}

// loadConfig loads configuration from file and applies CLI flag overrides
func loadConfig(cmd *cobra.Command) (*config.StaticConfig, error) {
	var cfg *config.StaticConfig
	var err error

	if configPath != "" {
		// Load from config file
		cfg, err = config.Read(configPath, dropInConfigDir)
		if err != nil {
			return nil, err
		}
	} else {
		// Use defaults
		cfg = config.Default()
	}

	// Apply CLI flag overrides
	if port != "" {
		cfg.Port = port
	}
	if logLevel >= 0 {
		cfg.LogLevel = logLevel
	}
	// Check if requireOAuth flag was explicitly set
	// Note: We need to check if the flag was actually provided by the user
	if cmd.Flags().Changed("require-oauth") {
		cfg.RequireOAuth = requireOAuth
	}
	if oauthAudience != "" {
		cfg.OAuthAudience = oauthAudience
	}
	if authorizationURL != "" {
		cfg.AuthorizationURL = authorizationURL
	}
	if mustGatherPath != "" {
		cfg.MustGatherPath = mustGatherPath
	}

	// Handle deprecated http-addr flag
	if httpAddr != "" && port == "" && httpMode {
		// Extract port from address (e.g., "localhost:8080" -> "8080")
		// The new --port flag only accepts port numbers, not full addresses
		if strings.Contains(httpAddr, ":") {
			parts := strings.Split(httpAddr, ":")
			if len(parts) == 2 {
				cfg.Port = parts[1]
				klog.Warningf("--http-addr is deprecated and only supports binding to all interfaces. Use --port instead. Extracted port: %s", parts[1])
			}
		} else {
			// Just a port number was provided
			cfg.Port = httpAddr
		}
	}

	return cfg, nil
}
