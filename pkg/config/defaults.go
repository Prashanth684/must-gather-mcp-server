package config

// Default returns a StaticConfig with default values.
func Default() *StaticConfig {
	return &StaticConfig{
		Port:     "8080",
		LogLevel: 0,
		OAuthScopes: []string{
			"openid",
			"profile",
			"email",
		},
		Stateless: false,
		Telemetry: TelemetryConfig{
			Enabled: false,
		},
	}
}
