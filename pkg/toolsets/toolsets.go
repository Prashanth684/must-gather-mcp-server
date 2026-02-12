package toolsets

import "github.com/openshift/must-gather-mcp-server/pkg/api"

// Registry holds all registered toolsets
var registry []api.Toolset

// Register registers a toolset
func Register(toolset api.Toolset) {
	registry = append(registry, toolset)
}

// All returns all registered toolsets
func All() []api.Toolset {
	return registry
}
