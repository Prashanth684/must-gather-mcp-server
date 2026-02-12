package cluster

import (
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"github.com/openshift/must-gather-mcp-server/pkg/toolsets"
)

func init() {
	toolsets.Register(&ClusterToolset{})
}

type ClusterToolset struct{}

func (t *ClusterToolset) Name() string {
	return "cluster"
}

func (t *ClusterToolset) Description() string {
	return "Tools for analyzing cluster-level configuration and status"
}

func (t *ClusterToolset) GetTools() []api.ServerTool {
	tools := []api.ServerTool{}
	tools = append(tools, versionTools()...)
	tools = append(tools, infoTools()...)
	tools = append(tools, operatorTools()...)
	tools = append(tools, nodeTools()...)
	return tools
}
