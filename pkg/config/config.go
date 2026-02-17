package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"k8s.io/klog/v2"
)

const (
	DefaultDropInConfigDir = "conf.d"
)

// StaticConfig is the configuration for the must-gather MCP server.
type StaticConfig struct {
	// Server settings
	LogLevel       int    `toml:"log_level,omitzero"`
	Port           string `toml:"port,omitempty"`
	SSEBaseURL     string `toml:"sse_base_url,omitempty"`
	MustGatherPath string `toml:"must_gather_path,omitempty"`

	// Stateless configures the MCP server to operate in stateless mode.
	// When true, the server will not send notifications to clients.
	// This is useful for container deployments and load balancing.
	// Defaults to false (stateful mode with notifications enabled).
	Stateless bool `toml:"stateless,omitempty"`

	// Toolset configuration
	Toolsets      []string `toml:"toolsets,omitempty"`
	EnabledTools  []string `toml:"enabled_tools,omitempty"`
	DisabledTools []string `toml:"disabled_tools,omitempty"`

	// Authorization-related fields
	// RequireOAuth indicates whether the server requires OAuth for authentication.
	RequireOAuth bool `toml:"require_oauth,omitempty"`
	// OAuthAudience is the valid audience for the OAuth tokens, used for offline JWT claim validation.
	OAuthAudience string `toml:"oauth_audience,omitempty"`
	// AuthorizationURL is the URL of the OIDC authorization server.
	// It is used for token validation and discovery.
	AuthorizationURL string `toml:"authorization_url,omitempty"`
	// DisableDynamicClientRegistration indicates whether dynamic client registration is disabled.
	// If true, the .well-known endpoints will not expose the registration endpoint.
	DisableDynamicClientRegistration bool `toml:"disable_dynamic_client_registration,omitempty"`
	// OAuthScopes are the supported client scopes requested during the OAuth flow.
	OAuthScopes []string `toml:"oauth_scopes,omitempty"`
	// StsClientId is the OAuth client ID used for backend token exchange
	StsClientId string `toml:"sts_client_id,omitempty"`
	// StsClientSecret is the OAuth client secret used for backend token exchange
	StsClientSecret string `toml:"sts_client_secret,omitempty"`
	// StsAudience is the audience for the STS token exchange.
	StsAudience string `toml:"sts_audience,omitempty"`
	// StsScopes is the scopes for the STS token exchange.
	StsScopes            []string `toml:"sts_scopes,omitempty"`
	CertificateAuthority string   `toml:"certificate_authority,omitempty"`
	ServerURL            string   `toml:"server_url,omitempty"`

	// Server instructions to be provided by the MCP server to the MCP client
	ServerInstructions string `toml:"server_instructions,omitempty"`

	// Telemetry contains OpenTelemetry configuration options.
	// These can also be configured via OTEL_* environment variables.
	Telemetry TelemetryConfig `toml:"telemetry,omitempty"`

	// Internal: the config.toml directory, to help resolve relative file paths
	configDirPath string
}

type ReadConfigOpt func(cfg *StaticConfig)

// WithDirPath returns a ReadConfigOpt that sets the config directory path.
func WithDirPath(path string) ReadConfigOpt {
	return func(cfg *StaticConfig) {
		cfg.configDirPath = path
	}
}

// Read reads the toml file, applies drop-in configs from configDir (if provided),
// and returns the StaticConfig with any opts applied.
// Loading order: defaults → main config file → drop-in files (lexically sorted)
func Read(configPath, dropInConfigDir string) (*StaticConfig, error) {
	var configFiles []string
	var configDir string

	// Main config file
	if configPath != "" {
		klog.V(2).Infof("Loading main config from: %s", configPath)
		configFiles = append(configFiles, configPath)

		// get and save the absolute dir path to the config file
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve absolute path to config file: %w", err)
		}
		configDir = filepath.Dir(absPath)
	}

	// Drop-in config files
	if dropInConfigDir == "" {
		dropInConfigDir = DefaultDropInConfigDir
	}

	// Resolve drop-in config directory path (relative to config directory)
	// If dropInConfigDir is already absolute, use it as-is
	if !filepath.IsAbs(dropInConfigDir) && configDir != "" {
		dropInConfigDir = filepath.Join(configDir, dropInConfigDir)
	}

	if configDir == "" {
		// If no main config, use drop-in dir as the config dir
		if filepath.IsAbs(dropInConfigDir) {
			configDir = dropInConfigDir
		} else {
			// Relative drop-in dir without main config - resolve to CWD
			cwd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to get current directory: %w", err)
			}
			dropInConfigDir = filepath.Join(cwd, dropInConfigDir)
			configDir = dropInConfigDir
		}
	}

	dropInFiles, err := loadDropInConfigs(dropInConfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load drop-in configs from %s: %w", dropInConfigDir, err)
	}
	if len(dropInFiles) == 0 {
		klog.V(2).Infof("No drop-in config files found in: %s", dropInConfigDir)
	} else {
		klog.V(2).Infof("Loading %d drop-in config file(s) from: %s", len(dropInFiles), dropInConfigDir)
	}
	configFiles = append(configFiles, dropInFiles...)

	// Read and merge all config files
	configData, err := readAndMergeFiles(configFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to read and merge config files: %w", err)
	}

	return ReadToml(configData, WithDirPath(configDir))
}

// loadDropInConfigs loads and merges config files from a drop-in directory.
// Files are processed in lexical (alphabetical) order.
// Only files with .toml extension are processed; dotfiles are ignored.
func loadDropInConfigs(dropInConfigDir string) ([]string, error) {
	// Check if directory exists
	info, err := os.Stat(dropInConfigDir)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(2).Infof("Drop-in config directory does not exist, skipping: %s", dropInConfigDir)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat drop-in directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("drop-in config path is not a directory: %s", dropInConfigDir)
	}

	// Get all .toml files in the directory
	return getSortedConfigFiles(dropInConfigDir)
}

// getSortedConfigFiles returns a sorted list of .toml files in the specified directory.
// Dotfiles (starting with '.') and non-.toml files are ignored.
// Files are sorted lexically (alphabetically) by filename.
func getSortedConfigFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip dotfiles
		if strings.HasPrefix(name, ".") {
			klog.V(4).Infof("Skipping dotfile: %s", name)
			continue
		}

		// Only process .toml files
		if !strings.HasSuffix(name, ".toml") {
			klog.V(4).Infof("Skipping non-.toml file: %s", name)
			continue
		}

		files = append(files, filepath.Join(dir, name))
	}

	// Sort lexically
	sort.Strings(files)

	return files, nil
}

// readAndMergeFiles reads and merges multiple TOML config files into a single byte slice.
// Files are merged in the order provided, with later files overriding earlier ones.
func readAndMergeFiles(files []string) ([]byte, error) {
	rawConfig := map[string]interface{}{}
	// Merge each file in order using deep merge
	for _, file := range files {
		klog.V(3).Infof("  - Merging config: %s", filepath.Base(file))
		configData, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read config %s: %w", file, err)
		}

		dropInConfig := make(map[string]interface{})
		if _, err = toml.NewDecoder(bytes.NewReader(configData)).Decode(&dropInConfig); err != nil {
			return nil, fmt.Errorf("failed to decode config %s: %w", file, err)
		}

		deepMerge(rawConfig, dropInConfig)
	}

	bufferedConfig := new(bytes.Buffer)
	if err := toml.NewEncoder(bufferedConfig).Encode(rawConfig); err != nil {
		return nil, fmt.Errorf("failed to encode merged config: %w", err)
	}
	return bufferedConfig.Bytes(), nil
}

// deepMerge recursively merges src into dst.
// For nested maps, it merges recursively. For other types, src overwrites dst.
func deepMerge(dst, src map[string]interface{}) {
	for key, srcVal := range src {
		if dstVal, exists := dst[key]; exists {
			// Both have this key - check if both are maps for recursive merge
			srcMap, srcIsMap := srcVal.(map[string]interface{})
			dstMap, dstIsMap := dstVal.(map[string]interface{})
			if srcIsMap && dstIsMap {
				deepMerge(dstMap, srcMap)
				continue
			}
		}
		// Either key doesn't exist in dst, or values aren't both maps - overwrite
		dst[key] = srcVal
	}
}

// ReadToml reads the toml data and returns the StaticConfig with any opts applied.
func ReadToml(configData []byte, opts ...ReadConfigOpt) (*StaticConfig, error) {
	config := Default()
	_, err := toml.NewDecoder(bytes.NewReader(configData)).Decode(config)
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(config)
	}

	return config, nil
}

// GetMustGatherPath returns the path to the must-gather directory
func (c *StaticConfig) GetMustGatherPath() string {
	return c.MustGatherPath
}

// IsRequireOAuth returns whether OAuth is required
func (c *StaticConfig) IsRequireOAuth() bool {
	return c.RequireOAuth
}

// GetStsClientId returns the STS client ID
func (c *StaticConfig) GetStsClientId() string {
	return c.StsClientId
}

// GetStsClientSecret returns the STS client secret
func (c *StaticConfig) GetStsClientSecret() string {
	return c.StsClientSecret
}

// GetStsAudience returns the STS audience
func (c *StaticConfig) GetStsAudience() string {
	return c.StsAudience
}

// GetStsScopes returns the STS scopes
func (c *StaticConfig) GetStsScopes() []string {
	return c.StsScopes
}

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const configDirPathKey contextKey = "configDirPath"

// withConfigDirPath returns a context with the config directory path
func withConfigDirPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, configDirPathKey, path)
}

// ConfigDirPath returns the config directory path from the context
func ConfigDirPath(ctx context.Context) string {
	if path, ok := ctx.Value(configDirPathKey).(string); ok {
		return path
	}
	return ""
}
