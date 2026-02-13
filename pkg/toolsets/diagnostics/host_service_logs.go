package diagnostics

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
)

func hostServiceLogsTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "host_service_logs_list",
				Description: "List all available host systemd service logs (kubelet, crio, NetworkManager, openvswitch, etc.)",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: hostServiceLogsList,
		},
		{
			Tool: api.Tool{
				Name:        "host_service_logs_get",
				Description: "Get systemd service logs from master nodes for debugging host-level issues (container runtime, kubelet, networking services)",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"service": {
							Type:        "string",
							Description: "Service name (e.g., kubelet, crio, NetworkManager, openvswitch, ovs-configuration, machine-config-daemon-host)",
						},
						"tail": {
							Type:        "integer",
							Description: "Number of lines from end (0 or omit for all logs)",
						},
					},
					Required: []string{"service"},
				},
			},
			Handler: hostServiceLogsGet,
		},
		{
			Tool: api.Tool{
				Name:        "host_service_logs_grep",
				Description: "Search across all host service logs for specific patterns (errors, warnings, specific events)",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"filter": {
							Type:        "string",
							Description: "String to search for in log lines",
						},
						"service": {
							Type:        "string",
							Description: "Optional: specific service to search (omit to search all services)",
						},
						"tail": {
							Type:        "integer",
							Description: "Maximum number of matching lines to return per service (0 for all matches)",
						},
						"caseInsensitive": {
							Type:        "boolean",
							Description: "Perform case-insensitive search (default: false)",
						},
					},
					Required: []string{"filter"},
				},
			},
			Handler: hostServiceLogsGrep,
		},
	}
}

func hostServiceLogsList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	services, err := params.MustGatherProvider.ListHostServiceLogs()
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list host service logs: %w", err)), nil
	}

	if len(services) == 0 {
		return api.NewToolCallResult("No host service logs found in must-gather", nil), nil
	}

	// Sort alphabetically
	sort.Strings(services)

	output := fmt.Sprintf("Found %d host systemd services with logs:\n\n", len(services))
	for i, service := range services {
		output += fmt.Sprintf("%d. %s\n", i+1, service)
	}

	output += "\nCommon services:\n"
	output += "  - kubelet: Kubernetes node agent logs\n"
	output += "  - crio: Container runtime logs\n"
	output += "  - NetworkManager: Host network management\n"
	output += "  - openvswitch: OVS daemon logs\n"
	output += "  - ovs-configuration: OVS configuration logs\n"
	output += "  - machine-config-daemon-host: OS configuration changes\n"

	return api.NewToolCallResult(output, nil), nil
}

func hostServiceLogsGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	service := params.GetString("service", "")
	tail := params.GetInt("tail", 0)

	if service == "" {
		return api.NewToolCallResult("", fmt.Errorf("service name is required")), nil
	}

	content, err := params.MustGatherProvider.GetHostServiceLog(service, tail)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get service log: %w", err)), nil
	}

	output := fmt.Sprintf("Host systemd service logs for: %s", service)
	if tail > 0 {
		output += fmt.Sprintf(" (last %d lines)", tail)
	}
	output += "\n"
	output += strings.Repeat("=", 80) + "\n\n"
	output += content

	return api.NewToolCallResult(output, nil), nil
}

func hostServiceLogsGrep(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	filter := params.GetString("filter", "")
	serviceName := params.GetString("service", "")
	tail := params.GetInt("tail", 0)
	caseInsensitive := params.GetBool("caseInsensitive", false)

	if filter == "" {
		return api.NewToolCallResult("", fmt.Errorf("filter string is required")), nil
	}

	// Get list of services to search
	var servicesToSearch []string
	if serviceName != "" {
		servicesToSearch = []string{serviceName}
	} else {
		services, err := params.MustGatherProvider.ListHostServiceLogs()
		if err != nil {
			return api.NewToolCallResult("", fmt.Errorf("failed to list services: %w", err)), nil
		}
		servicesToSearch = services
	}

	// Search across services
	output := fmt.Sprintf("Searching host service logs for: '%s'", filter)
	if caseInsensitive {
		output += " (case-insensitive)"
	}
	output += "\n"
	output += strings.Repeat("=", 80) + "\n\n"

	totalMatches := 0

	// Prepare filter for comparison
	searchFilter := filter
	if caseInsensitive {
		searchFilter = strings.ToLower(filter)
	}

	for _, service := range servicesToSearch {
		// Get the full log content
		content, err := params.MustGatherProvider.GetHostServiceLog(service, 0)
		if err != nil {
			continue // Skip services that can't be read
		}

		// Filter lines
		matchingLines := make([]string, 0)
		lines := strings.Split(content, "\n")

		for _, line := range lines {
			compareLine := line
			if caseInsensitive {
				compareLine = strings.ToLower(line)
			}

			if strings.Contains(compareLine, searchFilter) {
				matchingLines = append(matchingLines, line)

				// Apply tail limit if specified
				if tail > 0 && len(matchingLines) > tail {
					matchingLines = matchingLines[len(matchingLines)-tail:]
				}
			}
		}

		if len(matchingLines) > 0 {
			totalMatches += len(matchingLines)
			output += fmt.Sprintf("## %s (%d matches)\n\n", service, len(matchingLines))
			output += strings.Join(matchingLines, "\n")
			output += "\n\n"
		}
	}

	if totalMatches == 0 {
		output += "No matches found in any service logs.\n"
	} else {
		summary := fmt.Sprintf("Total: %d matches", totalMatches)
		if serviceName == "" {
			summary += fmt.Sprintf(" across %d services", len(servicesToSearch))
		}
		output = strings.Replace(output, strings.Repeat("=", 80)+"\n\n", strings.Repeat("=", 80)+"\n"+summary+"\n\n", 1)
	}

	return api.NewToolCallResult(output, nil), nil
}
