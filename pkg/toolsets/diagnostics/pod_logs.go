package diagnostics

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
)

func podLogsTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "pod_logs_get",
				Description: "Get logs for a specific pod container from must-gather. Returns current or previous logs.",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"namespace": {
							Type:        "string",
							Description: "Pod namespace",
						},
						"pod": {
							Type:        "string",
							Description: "Pod name",
						},
						"container": {
							Type:        "string",
							Description: "Container name (optional - will use first container if not specified)",
						},
						"previous": {
							Type:        "boolean",
							Description: "Get previous container logs (from previous crash/restart)",
						},
						"tail": {
							Type:        "integer",
							Description: "Number of lines from end of logs (0 or omit for all logs)",
						},
					},
					Required: []string{"namespace", "pod"},
				},
			},
			Handler: podLogsGet,
		},
		{
			Tool: api.Tool{
				Name:        "pod_containers_list",
				Description: "List all containers for a specific pod that have logs available",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"namespace": {
							Type:        "string",
							Description: "Pod namespace",
						},
						"pod": {
							Type:        "string",
							Description: "Pod name",
						},
					},
					Required: []string{"namespace", "pod"},
				},
			},
			Handler: podContainersList,
		},
	}
}

func podLogsGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	namespace := params.GetString("namespace", "")
	pod := params.GetString("pod", "")
	container := params.GetString("container", "")
	previous := params.GetBool("previous", false)
	tail := params.GetInt("tail", 0)

	if namespace == "" || pod == "" {
		return api.NewToolCallResult("", fmt.Errorf("namespace and pod are required")), nil
	}

	// If container not specified, try to get the first container
	if container == "" {
		containers, err := params.MustGatherProvider.ListPodContainers(namespace, pod)
		if err != nil {
			return api.NewToolCallResult("", fmt.Errorf("failed to list containers: %w", err)), nil
		}
		if len(containers) == 0 {
			return api.NewToolCallResult("", fmt.Errorf("no containers found for pod %s/%s", namespace, pod)), nil
		}
		container = containers[0]
	}

	// Determine log type
	logType := api.LogTypeCurrent
	if previous {
		logType = api.LogTypePrevious
	}

	opts := api.PodLogOptions{
		Namespace: namespace,
		Pod:       pod,
		Container: container,
		LogType:   logType,
		TailLines: tail,
	}

	logs, err := params.MustGatherProvider.GetPodLog(opts)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get pod logs: %w", err)), nil
	}

	// Format output
	output := fmt.Sprintf("Logs for pod %s/%s, container %s", namespace, pod, container)
	if previous {
		output += " (previous)"
	}
	if tail > 0 {
		output += fmt.Sprintf(" (last %d lines)", tail)
	}
	output += ":\n\n"
	output += logs

	return api.NewToolCallResult(output, nil), nil
}

func podContainersList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	namespace := params.GetString("namespace", "")
	pod := params.GetString("pod", "")

	if namespace == "" || pod == "" {
		return api.NewToolCallResult("", fmt.Errorf("namespace and pod are required")), nil
	}

	containers, err := params.MustGatherProvider.ListPodContainers(namespace, pod)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list containers: %w", err)), nil
	}

	if len(containers) == 0 {
		return api.NewToolCallResult(fmt.Sprintf("No containers with logs found for pod %s/%s", namespace, pod), nil), nil
	}

	output := fmt.Sprintf("Containers for pod %s/%s:\n\n", namespace, pod)
	for i, container := range containers {
		output += fmt.Sprintf("%d. %s\n", i+1, container)
	}

	return api.NewToolCallResult(output, nil), nil
}
