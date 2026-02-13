package diagnostics

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
)

func staticPodTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "static_pod_termination_logs",
				Description: "Get termination logs for static control plane pods (kube-apiserver, etcd, etc.) to debug crashes and restarts",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"pod": {
							Type:        "string",
							Description: "Static pod type (e.g., kube-apiserver, etcd, kube-controller-manager, kube-scheduler)",
						},
						"node": {
							Type:        "string",
							Description: "Node name (optional - if omitted, shows all available logs for the pod type)",
						},
					},
					Required: []string{"pod"},
				},
			},
			Handler: staticPodTerminationLogs,
		},
	}
}

func staticPodTerminationLogs(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	podType := params.GetString("pod", "")
	nodeName := params.GetString("node", "")

	if podType == "" {
		return api.NewToolCallResult("", fmt.Errorf("pod type is required")), nil
	}

	// Get available logs
	allLogs, err := params.MustGatherProvider.ListStaticPodTerminationLogs()
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list static pod logs: %w", err)), nil
	}

	if len(allLogs) == 0 {
		return api.NewToolCallResult("No static pod termination logs found in must-gather", nil), nil
	}

	// Check if pod type exists
	nodeLogs, exists := allLogs[podType]
	if !exists {
		available := []string{}
		for pod := range allLogs {
			available = append(available, pod)
		}
		sort.Strings(available)
		return api.NewToolCallResult(fmt.Sprintf("No termination logs found for pod type: %s\n\nAvailable pod types: %s",
			podType, strings.Join(available, ", ")), nil), nil
	}

	// If no specific node requested, list available nodes
	if nodeName == "" {
		output := fmt.Sprintf("Static pod termination logs for: %s\n", podType)
		output += strings.Repeat("=", 80) + "\n\n"
		output += fmt.Sprintf("Found logs for %d node(s):\n\n", len(nodeLogs))
		for i, node := range nodeLogs {
			output += fmt.Sprintf("%d. %s\n", i+1, node)
		}
		output += "\nUse the 'node' parameter to retrieve logs for a specific node.\n"
		return api.NewToolCallResult(output, nil), nil
	}

	// Get specific log
	content, err := params.MustGatherProvider.GetStaticPodTerminationLog(podType, nodeName)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get termination log: %w", err)), nil
	}

	output := fmt.Sprintf("Static Pod Termination Log\n")
	output += strings.Repeat("=", 80) + "\n"
	output += fmt.Sprintf("Pod Type: %s\n", podType)
	output += fmt.Sprintf("Node: %s\n", nodeName)
	output += strings.Repeat("=", 80) + "\n\n"
	output += content

	return api.NewToolCallResult(output, nil), nil
}
