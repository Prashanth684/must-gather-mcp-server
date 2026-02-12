package diagnostics

import (
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"github.com/openshift/must-gather-mcp-server/pkg/toolsets"
)

// Toolset represents the diagnostics toolset
type Toolset struct{}

// Name returns the toolset name
func (t *Toolset) Name() string {
	return "diagnostics"
}

// GetTools returns all tools in this toolset
func (t *Toolset) GetTools() []api.ServerTool {
	tools := make([]api.ServerTool, 0)
	tools = append(tools, podLogsTools()...)
	tools = append(tools, nodeTools()...)
	tools = append(tools, etcdTools()...)
	tools = append(tools, etcdExtendedTools()...)
	return tools
}

func init() {
	toolsets.Register(&Toolset{})
}
