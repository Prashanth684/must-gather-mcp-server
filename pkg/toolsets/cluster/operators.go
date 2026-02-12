package cluster

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func operatorTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "cluster_operators_list",
				Description: "List all OpenShift cluster operators with their status (Available, Degraded, Progressing)",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"status": {
							Type:        "string",
							Description: "Filter by status: all, degraded, progressing, unavailable (default: all)",
							Enum:        []interface{}{"all", "degraded", "progressing", "unavailable"},
						},
					},
				},
			},
			Handler: clusterOperatorsList,
		},
		{
			Tool: api.Tool{
				Name:        "cluster_operator_get",
				Description: "Get detailed information for a specific cluster operator including conditions, versions, and related objects",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"name": {
							Type:        "string",
							Description: "Cluster operator name (e.g., kube-apiserver, etcd, ingress)",
						},
					},
					Required: []string{"name"},
				},
			},
			Handler: clusterOperatorGet,
		},
	}
}

func clusterOperatorsList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	statusFilter := params.GetString("status", "all")

	gvk := parseGVK("config.openshift.io/v1", "ClusterOperator")
	opts := api.ListOptions{}

	operatorList, err := params.MustGatherProvider.ListResources(params.Context, gvk, "", opts)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list cluster operators: %w", err)), nil
	}

	operators := operatorList.Items
	if len(operators) == 0 {
		return api.NewToolCallResult("No cluster operators found", nil), nil
	}

	// Sort by name
	sort.Slice(operators, func(i, j int) bool {
		return operators[i].GetName() < operators[j].GetName()
	})

	output := "OpenShift Cluster Operators\n"
	output += strings.Repeat("=", 80) + "\n\n"
	output += fmt.Sprintf("Total Operators: %d\n\n", len(operators))

	// Table header
	output += fmt.Sprintf("%-35s %-12s %-12s %-12s\n", "NAME", "AVAILABLE", "PROGRESSING", "DEGRADED")
	output += strings.Repeat("-", 80) + "\n"

	filteredCount := 0
	for i := range operators {
		op := &operators[i]
		name := op.GetName()

		// Extract conditions
		available := getConditionStatus(op, "Available")
		progressing := getConditionStatus(op, "Progressing")
		degraded := getConditionStatus(op, "Degraded")

		// Apply filter
		if statusFilter != "all" {
			shouldInclude := false
			switch statusFilter {
			case "degraded":
				shouldInclude = (degraded == "True")
			case "progressing":
				shouldInclude = (progressing == "True")
			case "unavailable":
				shouldInclude = (available == "False")
			}
			if !shouldInclude {
				continue
			}
		}

		filteredCount++

		// Format status with symbols
		availSymbol := formatStatus(available)
		progSymbol := formatStatus(progressing)
		degSymbol := formatStatus(degraded)

		output += fmt.Sprintf("%-35s %-12s %-12s %-12s\n",
			name, availSymbol, progSymbol, degSymbol)
	}

	output += "\n"

	if statusFilter != "all" {
		output += fmt.Sprintf("\nShowing %d operators matching filter: %s\n", filteredCount, statusFilter)
	}

	return api.NewToolCallResult(output, nil), nil
}

func clusterOperatorGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	name := params.GetString("name", "")
	if name == "" {
		return api.NewToolCallResult("", fmt.Errorf("operator name is required")), nil
	}

	gvk := parseGVK("config.openshift.io/v1", "ClusterOperator")
	opts := api.ListOptions{}

	operatorList, err := params.MustGatherProvider.ListResources(params.Context, gvk, "", opts)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get cluster operator: %w", err)), nil
	}

	// Find the specific operator by name
	var op *unstructured.Unstructured
	for i := range operatorList.Items {
		if operatorList.Items[i].GetName() == name {
			op = &operatorList.Items[i]
			break
		}
	}

	if op == nil {
		return api.NewToolCallResult("", fmt.Errorf("cluster operator '%s' not found", name)), nil
	}

	output := fmt.Sprintf("Cluster Operator: %s\n", name)
	output += strings.Repeat("=", 80) + "\n\n"

	// Conditions
	conditions, found, _ := unstructured.NestedSlice(op.Object, "status", "conditions")
	if found && len(conditions) > 0 {
		output += "Status Conditions:\n"
		output += strings.Repeat("-", 80) + "\n"

		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _ := condMap["type"].(string)
			status, _ := condMap["status"].(string)
			message, _ := condMap["message"].(string)
			reason, _ := condMap["reason"].(string)
			lastTransition, _ := condMap["lastTransitionTime"].(string)

			symbol := getStatusSymbol(status)
			output += fmt.Sprintf("%s %s: %s\n", symbol, condType, status)

			if reason != "" {
				output += fmt.Sprintf("  Reason: %s\n", reason)
			}

			if lastTransition != "" {
				output += fmt.Sprintf("  Last Transition: %s\n", lastTransition)
			}

			if message != "" {
				// Format multi-line messages
				lines := strings.Split(message, "\n")
				output += "  Message:\n"
				for _, line := range lines {
					if strings.TrimSpace(line) != "" {
						output += fmt.Sprintf("    %s\n", line)
					}
				}
			}
			output += "\n"
		}
	}

	// Versions
	versions, found, _ := unstructured.NestedSlice(op.Object, "status", "versions")
	if found && len(versions) > 0 {
		output += "Versions:\n"
		output += strings.Repeat("-", 80) + "\n"

		for _, ver := range versions {
			verMap, ok := ver.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := verMap["name"].(string)
			version, _ := verMap["version"].(string)
			output += fmt.Sprintf("  %s: %s\n", name, version)
		}
		output += "\n"
	}

	// Related objects
	relatedObjs, found, _ := unstructured.NestedSlice(op.Object, "status", "relatedObjects")
	if found && len(relatedObjs) > 0 {
		output += "Related Objects:\n"
		output += strings.Repeat("-", 80) + "\n"

		for _, obj := range relatedObjs {
			objMap, ok := obj.(map[string]interface{})
			if !ok {
				continue
			}

			group, _ := objMap["group"].(string)
			resource, _ := objMap["resource"].(string)
			name, _ := objMap["name"].(string)
			namespace, _ := objMap["namespace"].(string)

			if group != "" {
				output += fmt.Sprintf("  - %s/%s", group, resource)
			} else {
				output += fmt.Sprintf("  - %s", resource)
			}

			if namespace != "" {
				output += fmt.Sprintf(" %s/%s\n", namespace, name)
			} else {
				output += fmt.Sprintf(" %s\n", name)
			}
		}
		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func getConditionStatus(obj *unstructured.Unstructured, conditionType string) string {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		return "Unknown"
	}

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}

		if condType, ok := condMap["type"].(string); ok && condType == conditionType {
			if status, ok := condMap["status"].(string); ok {
				return status
			}
		}
	}

	return "Unknown"
}

func formatStatus(status string) string {
	switch status {
	case "True":
		return "✓ True"
	case "False":
		return "✗ False"
	default:
		return "? Unknown"
	}
}
