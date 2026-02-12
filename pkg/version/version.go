package version

// Version information populated by the build process
var (
	Version    = "dev"
	CommitHash = "unknown"
	BuildTime  = "unknown"
	BinaryName = "must-gather-mcp-server"
)

// Info returns formatted version information
func Info() string {
	return BinaryName + " " + Version + " (commit: " + CommitHash + ", built: " + BuildTime + ")"
}
