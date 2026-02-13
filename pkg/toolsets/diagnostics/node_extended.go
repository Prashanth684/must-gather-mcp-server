package diagnostics

import (
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
)

func nodeExtendedTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "node_hardware_info",
				Description: "Get hardware information for a node including CPU topology, PCI devices, and network interfaces",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"node": {
							Type:        "string",
							Description: "Node name",
						},
					},
					Required: []string{"node"},
				},
			},
			Handler: nodeHardwareInfo,
		},
		{
			Tool: api.Tool{
				Name:        "node_dmesg_errors",
				Description: "Parse kernel dmesg logs for errors, warnings, OOM kills, and hardware issues",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"node": {
							Type:        "string",
							Description: "Node name (optional - if omitted, scans all nodes)",
						},
						"severity": {
							Type:        "string",
							Description: "Filter by severity: all, error, warning, oom (default: all)",
							Enum:        []interface{}{"all", "error", "warning", "oom"},
						},
					},
				},
			},
			Handler: nodeDmesgErrors,
		},
		{
			Tool: api.Tool{
				Name:        "node_kernel_info",
				Description: "Get kernel command line parameters and boot configuration for a node",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"node": {
							Type:        "string",
							Description: "Node name",
						},
					},
					Required: []string{"node"},
				},
			},
			Handler: nodeKernelInfo,
		},
	}
}

func nodeHardwareInfo(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	node := params.GetString("node", "")
	if node == "" {
		return api.NewToolCallResult("", fmt.Errorf("node is required")), nil
	}

	diag, err := params.MustGatherProvider.GetNodeDiagnostics(node)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get node diagnostics: %w", err)), nil
	}

	output := fmt.Sprintf("Hardware Information for Node: %s\n", node)
	output += strings.Repeat("=", 80) + "\n\n"

	// CPU Info
	if diag.Lscpu != "" {
		output += "## CPU Information\n\n"
		output += diag.Lscpu + "\n\n"
	}

	// PCI Devices
	if diag.Lspci != "" {
		output += "## PCI Devices\n\n"
		output += diag.Lspci + "\n\n"
	}

	// Network configuration (from sysinfo if available)
	if diag.SysInfo != "" {
		// Try to extract network info from sysinfo
		if strings.Contains(diag.SysInfo, "ip addr") || strings.Contains(diag.SysInfo, "Network") {
			output += "## Network Information\n\n"
			output += "See sysinfo for detailed network configuration\n\n"
		}
	}

	return api.NewToolCallResult(output, nil), nil
}

func nodeDmesgErrors(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	nodeName := params.GetString("node", "")
	severityFilter := params.GetString("severity", "all")

	// Get list of nodes
	var nodes []string
	if nodeName != "" {
		nodes = []string{nodeName}
	} else {
		var err error
		nodes, err = params.MustGatherProvider.ListNodes()
		if err != nil {
			return api.NewToolCallResult("", fmt.Errorf("failed to list nodes: %w", err)), nil
		}
	}

	if len(nodes) == 0 {
		return api.NewToolCallResult("No nodes found", nil), nil
	}

	output := "Kernel Messages (dmesg) Analysis:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	totalErrors := 0
	totalWarnings := 0
	totalOOMs := 0

	for _, node := range nodes {
		diag, err := params.MustGatherProvider.GetNodeDiagnostics(node)
		if err != nil {
			continue
		}

		if diag.Dmesg == "" {
			continue
		}

		// Parse dmesg for issues
		issues := parseDmesgIssues(diag.Dmesg, severityFilter)

		if len(issues) == 0 {
			continue
		}

		output += fmt.Sprintf("Node: %s\n", node)
		output += strings.Repeat("-", 40) + "\n"

		// Count by severity
		errors := 0
		warnings := 0
		ooms := 0
		for _, issue := range issues {
			switch issue.Severity {
			case "error":
				errors++
			case "warning":
				warnings++
			case "oom":
				ooms++
			}
		}

		totalErrors += errors
		totalWarnings += warnings
		totalOOMs += ooms

		output += fmt.Sprintf("Found: %d errors, %d warnings, %d OOM kills\n\n", errors, warnings, ooms)

		for _, issue := range issues {
			symbol := "⚠"
			if issue.Severity == "error" || issue.Severity == "oom" {
				symbol = "✗"
			}
			output += fmt.Sprintf("%s [%s] %s\n", symbol, strings.ToUpper(issue.Severity), issue.Message)
		}

		output += "\n"
	}

	// Summary
	summary := fmt.Sprintf("Summary: %d errors, %d warnings, %d OOM kills across %d node(s)\n\n",
		totalErrors, totalWarnings, totalOOMs, len(nodes))
	output = summary + output

	return api.NewToolCallResult(output, nil), nil
}

func nodeKernelInfo(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	node := params.GetString("node", "")
	if node == "" {
		return api.NewToolCallResult("", fmt.Errorf("node is required")), nil
	}

	diag, err := params.MustGatherProvider.GetNodeDiagnostics(node)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get node diagnostics: %w", err)), nil
	}

	output := fmt.Sprintf("Kernel Configuration for Node: %s\n", node)
	output += strings.Repeat("=", 80) + "\n\n"

	if diag.ProcCmdline != "" {
		output += "## Kernel Boot Parameters\n\n"
		output += diag.ProcCmdline + "\n\n"

		// Parse interesting parameters
		params := strings.Fields(diag.ProcCmdline)
		output += "## Key Parameters\n\n"

		for _, param := range params {
			// Highlight important parameters
			if strings.HasPrefix(param, "console=") ||
				strings.HasPrefix(param, "root=") ||
				strings.HasPrefix(param, "cgroup") ||
				strings.HasPrefix(param, "systemd") ||
				strings.HasPrefix(param, "ostree") ||
				strings.HasPrefix(param, "ignition") {
				output += fmt.Sprintf("  - %s\n", param)
			}
		}
	} else {
		output += "Kernel command line not available\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

type dmesgIssue struct {
	Severity string
	Message  string
}

func parseDmesgIssues(dmesg string, severityFilter string) []dmesgIssue {
	issues := []dmesgIssue{}

	if dmesg == "" {
		return issues
	}

	lines := strings.Split(dmesg, "\n")

	for _, line := range lines {
		lowerLine := strings.ToLower(line)

		// Check for OOM kills
		if (severityFilter == "all" || severityFilter == "oom") &&
			(strings.Contains(lowerLine, "out of memory") ||
				strings.Contains(lowerLine, "oom") ||
				strings.Contains(lowerLine, "killed process")) {
			issues = append(issues, dmesgIssue{
				Severity: "oom",
				Message:  strings.TrimSpace(line),
			})
			continue
		}

		// Check for errors
		if (severityFilter == "all" || severityFilter == "error") &&
			(strings.Contains(lowerLine, " error") ||
				strings.Contains(lowerLine, " fail") ||
				strings.Contains(lowerLine, " critical") ||
				strings.Contains(lowerLine, " panic") ||
				strings.Contains(lowerLine, "i/o error") ||
				strings.Contains(lowerLine, "hardware error")) {
			issues = append(issues, dmesgIssue{
				Severity: "error",
				Message:  strings.TrimSpace(line),
			})
			continue
		}

		// Check for warnings
		if (severityFilter == "all" || severityFilter == "warning") &&
			(strings.Contains(lowerLine, " warn") ||
				strings.Contains(lowerLine, "temperature") ||
				strings.Contains(lowerLine, "thermal")) {
			issues = append(issues, dmesgIssue{
				Severity: "warning",
				Message:  strings.TrimSpace(line),
			})
			continue
		}
	}

	return issues
}
