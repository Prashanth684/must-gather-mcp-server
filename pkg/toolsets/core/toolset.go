package core

import (
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"github.com/openshift/must-gather-mcp-server/pkg/toolsets"
)

// Toolset represents the core toolset
type Toolset struct{}

// Name returns the toolset name
func (t *Toolset) Name() string {
	return "core"
}

// GetTools returns all tools in this toolset
func (t *Toolset) GetTools() []api.ServerTool {
	tools := make([]api.ServerTool, 0)
	tools = append(tools, resourcesTools()...)
	tools = append(tools, namespacesTools()...)
	return tools
}

func init() {
	toolsets.Register(&Toolset{})
}
