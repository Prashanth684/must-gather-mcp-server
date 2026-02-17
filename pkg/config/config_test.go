package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	suite.Suite
	tempDir string
}

func (s *ConfigSuite) SetupTest() {
	var err error
	s.tempDir, err = os.MkdirTemp("", "config-test-*")
	s.Require().NoError(err)
}

func (s *ConfigSuite) TearDownTest() {
	if s.tempDir != "" {
		os.RemoveAll(s.tempDir)
	}
}

func (s *ConfigSuite) TestDefault() {
	s.Run("returns default configuration", func() {
		cfg := Default()
		s.NotNil(cfg)
		s.Equal("8080", cfg.Port)
		s.Equal(0, cfg.LogLevel)
		s.False(cfg.RequireOAuth)
		s.False(cfg.Stateless)
		s.Contains(cfg.OAuthScopes, "openid")
	})
}

func (s *ConfigSuite) TestReadToml() {
	s.Run("parses basic TOML config", func() {
		configData := []byte(`
port = "9000"
log_level = 2
require_oauth = true
oauth_audience = "test-audience"
`)
		cfg, err := ReadToml(configData)
		s.NoError(err)
		s.NotNil(cfg)
		s.Equal("9000", cfg.Port)
		s.Equal(2, cfg.LogLevel)
		s.True(cfg.RequireOAuth)
		s.Equal("test-audience", cfg.OAuthAudience)
	})

	s.Run("handles empty config with defaults", func() {
		configData := []byte("")
		cfg, err := ReadToml(configData)
		s.NoError(err)
		s.NotNil(cfg)
		s.Equal("8080", cfg.Port) // from defaults
	})

	s.Run("parses telemetry config", func() {
		configData := []byte(`
[telemetry]
enabled = true
endpoint = "http://localhost:4318"
service_name = "test-service"
`)
		cfg, err := ReadToml(configData)
		s.NoError(err)
		s.NotNil(cfg)
		s.True(cfg.Telemetry.Enabled)
		s.Equal("http://localhost:4318", cfg.Telemetry.Endpoint)
		s.Equal("test-service", cfg.Telemetry.ServiceName)
	})

	s.Run("parses OAuth scopes", func() {
		configData := []byte(`
oauth_scopes = ["openid", "profile", "custom:scope"]
`)
		cfg, err := ReadToml(configData)
		s.NoError(err)
		s.NotNil(cfg)
		s.Len(cfg.OAuthScopes, 3)
		s.Contains(cfg.OAuthScopes, "custom:scope")
	})

	s.Run("returns error for invalid TOML", func() {
		configData := []byte(`
port = "9000
invalid toml here
`)
		_, err := ReadToml(configData)
		s.Error(err)
	})
}

func (s *ConfigSuite) TestRead() {
	s.Run("reads main config file", func() {
		configPath := filepath.Join(s.tempDir, "config.toml")
		configContent := `
port = "9000"
log_level = 2
must_gather_path = "/data/must-gather"
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		s.Require().NoError(err)

		cfg, err := Read(configPath, "")
		s.NoError(err)
		s.NotNil(cfg)
		s.Equal("9000", cfg.Port)
		s.Equal(2, cfg.LogLevel)
		s.Equal("/data/must-gather", cfg.MustGatherPath)
	})

	s.Run("merges drop-in configs", func() {
		// Create main config
		configPath := filepath.Join(s.tempDir, "config.toml")
		mainConfig := `
port = "8080"
log_level = 1
`
		err := os.WriteFile(configPath, []byte(mainConfig), 0644)
		s.Require().NoError(err)

		// Create drop-in directory
		dropInDir := filepath.Join(s.tempDir, "conf.d")
		err = os.MkdirAll(dropInDir, 0755)
		s.Require().NoError(err)

		// Create drop-in file that overrides port
		dropIn1 := filepath.Join(dropInDir, "01-oauth.toml")
		dropIn1Content := `
require_oauth = true
oauth_audience = "test"
`
		err = os.WriteFile(dropIn1, []byte(dropIn1Content), 0644)
		s.Require().NoError(err)

		// Create another drop-in file
		dropIn2 := filepath.Join(dropInDir, "50-override.toml")
		dropIn2Content := `
port = "9000"
`
		err = os.WriteFile(dropIn2, []byte(dropIn2Content), 0644)
		s.Require().NoError(err)

		cfg, err := Read(configPath, "conf.d")
		s.NoError(err)
		s.NotNil(cfg)
		s.Equal("9000", cfg.Port)          // overridden by drop-in
		s.Equal(1, cfg.LogLevel)           // from main
		s.True(cfg.RequireOAuth)           // from drop-in
		s.Equal("test", cfg.OAuthAudience) // from drop-in
	})

	s.Run("handles missing drop-in directory", func() {
		configPath := filepath.Join(s.tempDir, "config.toml")
		configContent := `port = "8080"`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		s.Require().NoError(err)

		cfg, err := Read(configPath, "nonexistent-dir")
		s.NoError(err) // should not error on missing drop-in dir
		s.NotNil(cfg)
		s.Equal("8080", cfg.Port)
	})

	s.Run("ignores dotfiles in drop-in dir", func() {
		testDir, err := os.MkdirTemp(s.tempDir, "dotfile-test-*")
		s.Require().NoError(err)
		dropInDir := filepath.Join(testDir, "conf.d")
		err = os.MkdirAll(dropInDir, 0755)
		s.Require().NoError(err)

		// Create a dotfile that should be ignored
		dotfile := filepath.Join(dropInDir, ".hidden.toml")
		err = os.WriteFile(dotfile, []byte(`port = "9999"`), 0644)
		s.Require().NoError(err)

		// Create normal file
		normalFile := filepath.Join(dropInDir, "config.toml")
		err = os.WriteFile(normalFile, []byte(`port = "8888"`), 0644)
		s.Require().NoError(err)

		cfg, err := Read("", dropInDir)
		s.NoError(err)
		s.Equal("8888", cfg.Port) // dotfile was ignored
	})

	s.Run("processes drop-in files in alphabetical order", func() {
		testDir, err := os.MkdirTemp(s.tempDir, "sort-test-*")
		s.Require().NoError(err)
		dropInDir := filepath.Join(testDir, "conf.d")
		err = os.MkdirAll(dropInDir, 0755)
		s.Require().NoError(err)

		// Create files that will be sorted
		files := []struct {
			name    string
			content string
		}{
			{"99-last.toml", `port = "9999"`},
			{"01-first.toml", `port = "1111"`},
			{"50-middle.toml", `port = "5555"`},
		}

		for _, f := range files {
			path := filepath.Join(dropInDir, f.name)
			err = os.WriteFile(path, []byte(f.content), 0644)
			s.Require().NoError(err)
		}

		cfg, err := Read("", dropInDir)
		s.NoError(err)
		s.Equal("9999", cfg.Port) // last file wins
	})
}

func (s *ConfigSuite) TestDeepMerge() {
	s.Run("merges nested maps", func() {
		dst := map[string]interface{}{
			"top": map[string]interface{}{
				"nested": map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
		}

		src := map[string]interface{}{
			"top": map[string]interface{}{
				"nested": map[string]interface{}{
					"key2": "overridden",
					"key3": "new",
				},
			},
		}

		deepMerge(dst, src)

		top := dst["top"].(map[string]interface{})
		nested := top["nested"].(map[string]interface{})

		s.Equal("value1", nested["key1"])
		s.Equal("overridden", nested["key2"])
		s.Equal("new", nested["key3"])
	})

	s.Run("overwrites non-map values", func() {
		dst := map[string]interface{}{
			"key1": "original",
			"key2": 42,
		}

		src := map[string]interface{}{
			"key1": "updated",
			"key2": 100,
			"key3": "new",
		}

		deepMerge(dst, src)

		s.Equal("updated", dst["key1"])
		s.Equal(100, dst["key2"])
		s.Equal("new", dst["key3"])
	})
}

func (s *ConfigSuite) TestGetters() {
	s.Run("GetMustGatherPath returns path", func() {
		cfg := &StaticConfig{MustGatherPath: "/test/path"}
		s.Equal("/test/path", cfg.GetMustGatherPath())
	})

	s.Run("IsRequireOAuth returns auth requirement", func() {
		cfg := &StaticConfig{RequireOAuth: true}
		s.True(cfg.IsRequireOAuth())
	})

	s.Run("GetStsClientId returns client ID", func() {
		cfg := &StaticConfig{StsClientId: "test-client"}
		s.Equal("test-client", cfg.GetStsClientId())
	})

	s.Run("GetStsClientSecret returns client secret", func() {
		cfg := &StaticConfig{StsClientSecret: "secret123"}
		s.Equal("secret123", cfg.GetStsClientSecret())
	})

	s.Run("GetStsAudience returns audience", func() {
		cfg := &StaticConfig{StsAudience: "api-server"}
		s.Equal("api-server", cfg.GetStsAudience())
	})

	s.Run("GetStsScopes returns scopes", func() {
		cfg := &StaticConfig{StsScopes: []string{"read", "write"}}
		scopes := cfg.GetStsScopes()
		s.Len(scopes, 2)
		s.Contains(scopes, "read")
		s.Contains(scopes, "write")
	})
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
