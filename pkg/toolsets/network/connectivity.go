package network

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

func networkConnectivityTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "network_connectivity_check",
				Description: "Get pod network connectivity check results showing reachability between cluster components",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"status": {
							Type:        "string",
							Description: "Filter by status: all, failing, degraded (default: all)",
							Enum:        []interface{}{"all", "failing", "degraded"},
						},
					},
				},
			},
			Handler: networkConnectivityCheck,
		},
	}
}

func networkConnectivityCheck(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	statusFilter := params.GetString("status", "all")

	// Find container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	connectivityFile := filepath.Join(containerDir, "pod_network_connectivity_check", "podnetworkconnectivitychecks.yaml")

	// Check if file exists
	if _, err := os.Stat(connectivityFile); os.IsNotExist(err) {
		return api.NewToolCallResult("", fmt.Errorf("network connectivity check data not found")), nil
	}

	// Read and parse YAML
	data, err := os.ReadFile(connectivityFile)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to read connectivity check data: %w", err)), nil
	}

	// Parse as List
	var checkList unstructured.Unstructured
	if err := yaml.Unmarshal(data, &checkList.Object); err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to parse connectivity check data: %w", err)), nil
	}

	items, found, err := unstructured.NestedSlice(checkList.Object, "items")
	if err != nil || !found {
		return api.NewToolCallResult("", fmt.Errorf("no connectivity checks found")), nil
	}

	checks := make([]*unstructured.Unstructured, 0)
	for _, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			check := &unstructured.Unstructured{Object: itemMap}
			checks = append(checks, check)
		}
	}

	// Sort by name
	sort.Slice(checks, func(i, j int) bool {
		return checks[i].GetName() < checks[j].GetName()
	})

	output := "Pod Network Connectivity Checks\n"
	output += strings.Repeat("=", 80) + "\n\n"

	totalChecks := len(checks)
	failingChecks := 0
	degradedChecks := 0

	filteredChecks := make([]*unstructured.Unstructured, 0)

	for _, check := range checks {
		// Get condition
		reachable := getConnectivityCondition(check, "Reachable")

		if reachable == "False" {
			failingChecks++
		} else if reachable != "True" {
			degradedChecks++
		}

		// Apply filter
		if statusFilter != "all" {
			shouldInclude := false
			switch statusFilter {
			case "failing":
				shouldInclude = (reachable == "False")
			case "degraded":
				shouldInclude = (reachable != "True" && reachable != "False")
			}
			if !shouldInclude {
				continue
			}
		}

		filteredChecks = append(filteredChecks, check)
	}

	output += fmt.Sprintf("Total Checks: %d\n", totalChecks)
	output += fmt.Sprintf("Failing: %d\n", failingChecks)
	output += fmt.Sprintf("Degraded: %d\n\n", degradedChecks)

	if len(filteredChecks) == 0 {
		output += "No connectivity checks found matching filter.\n"
		return api.NewToolCallResult(output, nil), nil
	}

	output += fmt.Sprintf("Showing %d checks:\n", len(filteredChecks))
	output += strings.Repeat("-", 80) + "\n\n"

	for i, check := range filteredChecks {
		name := check.GetName()
		sourcePod, _, _ := unstructured.NestedString(check.Object, "spec", "sourcePod")
		targetEndpoint, _, _ := unstructured.NestedString(check.Object, "spec", "targetEndpoint")

		reachable := getConnectivityCondition(check, "Reachable")
		message := getConnectivityMessage(check, "Reachable")

		symbol := "✓"
		statusStr := "Reachable"
		if reachable == "False" {
			symbol = "✗"
			statusStr = "FAILING"
		} else if reachable != "True" {
			symbol = "?"
			statusStr = "UNKNOWN"
		}

		output += fmt.Sprintf("%d. %s %s\n", i+1, symbol, statusStr)
		output += fmt.Sprintf("   Name: %s\n", name)
		output += fmt.Sprintf("   Source: %s\n", sourcePod)
		output += fmt.Sprintf("   Target: %s\n", targetEndpoint)

		if message != "" && reachable == "False" {
			// Truncate long messages
			if len(message) > 150 {
				message = message[:150] + "..."
			}
			output += fmt.Sprintf("   Message: %s\n", message)
		}

		// Show recent failures if check is failing
		if reachable == "False" {
			failures, found, _ := unstructured.NestedSlice(check.Object, "status", "failures")
			if found && len(failures) > 0 {
				// Show last 3 failures
				count := len(failures)
				if count > 3 {
					count = 3
				}
				output += fmt.Sprintf("   Recent Failures (%d of %d):\n", count, len(failures))
				for j := 0; j < count; j++ {
					if failureMap, ok := failures[j].(map[string]interface{}); ok {
						timeStr, _ := failureMap["time"].(string)
						latency, _ := failureMap["latency"].(string)
						output += fmt.Sprintf("     - %s (latency: %s)\n", timeStr, latency)
					}
				}
			}
		}

		output += "\n"
	}

	if statusFilter != "all" {
		output += fmt.Sprintf("Filtered by status: %s\n", statusFilter)
	}

	return api.NewToolCallResult(output, nil), nil
}

func getConnectivityCondition(check *unstructured.Unstructured, conditionType string) string {
	conditions, found, _ := unstructured.NestedSlice(check.Object, "status", "conditions")
	if !found {
		return "Unknown"
	}

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}

		if cType, ok := condMap["type"].(string); ok && cType == conditionType {
			if status, ok := condMap["status"].(string); ok {
				return status
			}
		}
	}

	return "Unknown"
}

func getConnectivityMessage(check *unstructured.Unstructured, conditionType string) string {
	conditions, found, _ := unstructured.NestedSlice(check.Object, "status", "conditions")
	if !found {
		return ""
	}

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}

		if cType, ok := condMap["type"].(string); ok && cType == conditionType {
			if message, ok := condMap["message"].(string); ok {
				return message
			}
		}
	}

	return ""
}

// parseGVK parses apiVersion and kind into GroupVersionKind
func parseGVK(apiVersion, kind string) schema.GroupVersionKind {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		// Fallback for simple case
		return schema.GroupVersionKind{
			Group:   "",
			Version: apiVersion,
			Kind:    kind,
		}
	}
	return schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    kind,
	}
}
