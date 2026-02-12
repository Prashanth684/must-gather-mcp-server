package network

import (
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"github.com/openshift/must-gather-mcp-server/pkg/toolsets"
)

func init() {
	toolsets.Register(&NetworkToolset{})
}

type NetworkToolset struct{}

func (t *NetworkToolset) Name() string {
	return "network"
}

func (t *NetworkToolset) Description() string {
	return "Tools for analyzing network configuration, connectivity, and performance"
}

func (t *NetworkToolset) GetTools() []api.ServerTool {
	tools := []api.ServerTool{}
	tools = append(tools, networkInfoTools()...)
	tools = append(tools, networkConnectivityTools()...)
	return tools
}
