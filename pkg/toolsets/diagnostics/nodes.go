package diagnostics

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"github.com/openshift/must-gather-mcp-server/pkg/mustgather"
)

func nodeTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "nodes_list",
				Description: "List all nodes with diagnostic data available in must-gather",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: nodesList,
		},
		{
			Tool: api.Tool{
				Name:        "node_diagnostics_get",
				Description: "Get comprehensive diagnostic information for a specific node including kubelet logs, system info, CPU/IRQ affinities, and hardware details",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"node": {
							Type:        "string",
							Description: "Node name",
						},
						"include": {
							Type:        "string",
							Description: "Comma-separated list of diagnostics to include: kubelet,sysinfo,cpu,irq,pods,lscpu,lspci,dmesg,cmdline (default: all)",
						},
						"kubeletTail": {
							Type:        "integer",
							Description: "Number of lines from end of kubelet log (0 for all, default: 100)",
						},
					},
					Required: []string{"node"},
				},
			},
			Handler: nodeDiagnosticsGet,
		},
		{
			Tool: api.Tool{
				Name:        "node_kubelet_logs",
				Description: "Get kubelet logs for a specific node (decompressed from .gz file)",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"node": {
							Type:        "string",
							Description: "Node name",
						},
						"tail": {
							Type:        "integer",
							Description: "Number of lines from end (0 or omit for all logs)",
						},
					},
					Required: []string{"node"},
				},
			},
			Handler: nodeKubeletLogs,
		},
	}
}

func nodesList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	nodes, err := params.MustGatherProvider.ListNodes()
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list nodes: %w", err)), nil
	}

	if len(nodes) == 0 {
		return api.NewToolCallResult("No node diagnostic data found in must-gather", nil), nil
	}

	// Sort alphabetically
	sort.Strings(nodes)

	output := fmt.Sprintf("Found %d nodes with diagnostic data:\n\n", len(nodes))
	for i, node := range nodes {
		output += fmt.Sprintf("%d. %s\n", i+1, node)
	}

	return api.NewToolCallResult(output, nil), nil
}

func nodeDiagnosticsGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	node := params.GetString("node", "")
	include := params.GetString("include", "all")
	kubeletTail := params.GetInt("kubeletTail", 100)

	if node == "" {
		return api.NewToolCallResult("", fmt.Errorf("node is required")), nil
	}

	diag, err := params.MustGatherProvider.GetNodeDiagnostics(node)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get node diagnostics: %w", err)), nil
	}

	// Parse include list
	includeAll := include == "all"
	includeMap := make(map[string]bool)
	if !includeAll {
		for _, item := range strings.Split(include, ",") {
			includeMap[strings.TrimSpace(item)] = true
		}
	}

	shouldInclude := func(name string) bool {
		return includeAll || includeMap[name]
	}

	// Build output
	output := fmt.Sprintf("Node Diagnostics for %s\n", node)
	output += strings.Repeat("=", 80) + "\n\n"

	// Kubelet logs
	if shouldInclude("kubelet") && diag.KubeletLog != "" {
		output += "## Kubelet Logs"
		if kubeletTail > 0 {
			output += fmt.Sprintf(" (last %d lines)", kubeletTail)
			diag.KubeletLog = mustgather.TailLines(diag.KubeletLog, kubeletTail)
		}
		output += "\n\n"
		output += diag.KubeletLog + "\n\n"
	}

	// System info
	if shouldInclude("sysinfo") && diag.SysInfo != "" {
		output += "## System Info\n\n"
		output += diag.SysInfo + "\n\n"
	}

	// CPU info
	if shouldInclude("lscpu") && diag.Lscpu != "" {
		output += "## CPU Info (lscpu)\n\n"
		output += diag.Lscpu + "\n\n"
	}

	// CPU affinities
	if shouldInclude("cpu") && diag.CPUAffinities != "" {
		output += "## CPU Affinities\n\n"
		output += diag.CPUAffinities + "\n\n"
	}

	// IRQ affinities
	if shouldInclude("irq") && diag.IRQAffinities != "" {
		output += "## IRQ Affinities\n\n"
		output += diag.IRQAffinities + "\n\n"
	}

	// PCI devices
	if shouldInclude("lspci") && diag.Lspci != "" {
		output += "## PCI Devices (lspci)\n\n"
		output += diag.Lspci + "\n\n"
	}

	// Kernel messages
	if shouldInclude("dmesg") && diag.Dmesg != "" {
		output += "## Kernel Messages (dmesg)\n\n"
		output += diag.Dmesg + "\n\n"
	}

	// Boot parameters
	if shouldInclude("cmdline") && diag.ProcCmdline != "" {
		output += "## Kernel Boot Parameters\n\n"
		output += diag.ProcCmdline + "\n\n"
	}

	// Pod info
	if shouldInclude("pods") && diag.PodsInfo != "" {
		output += "## Pods Info\n\n"
		output += diag.PodsInfo + "\n\n"
	}

	// Pod resources
	if shouldInclude("pods") && diag.PodResources != "" {
		output += "## Pod Resources\n\n"
		output += diag.PodResources + "\n\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func nodeKubeletLogs(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	node := params.GetString("node", "")
	tail := params.GetInt("tail", 0)

	if node == "" {
		return api.NewToolCallResult("", fmt.Errorf("node is required")), nil
	}

	diag, err := params.MustGatherProvider.GetNodeDiagnostics(node)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get node diagnostics: %w", err)), nil
	}

	if diag.KubeletLog == "" {
		return api.NewToolCallResult("", fmt.Errorf("kubelet log not found for node %s", node)), nil
	}

	logs := diag.KubeletLog
	if tail > 0 {
		logs = mustgather.TailLines(logs, tail)
	}

	output := fmt.Sprintf("Kubelet logs for node %s", node)
	if tail > 0 {
		output += fmt.Sprintf(" (last %d lines)", tail)
	}
	output += ":\n\n"
	output += logs

	return api.NewToolCallResult(output, nil), nil
}
