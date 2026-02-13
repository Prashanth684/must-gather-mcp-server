package monitoring

import (
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"github.com/openshift/must-gather-mcp-server/pkg/toolsets"
)

func init() {
	toolsets.Register(&MonitoringToolset{})
}

type MonitoringToolset struct{}

func (t *MonitoringToolset) Name() string {
	return "monitoring"
}

func (t *MonitoringToolset) Description() string {
	return "Tools for analyzing Prometheus and AlertManager monitoring data"
}

func (t *MonitoringToolset) GetTools() []api.ServerTool {
	tools := []api.ServerTool{}
	tools = append(tools, prometheusTools()...)
	tools = append(tools, alertTools()...)
	tools = append(tools, configTools()...)
	return tools
}
