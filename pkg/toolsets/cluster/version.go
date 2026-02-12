package cluster

import (
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func versionTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "cluster_version_get",
				Description: "Get OpenShift cluster version information including current version, update status, and conditions",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: clusterVersionGet,
		},
	}
}

func clusterVersionGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	// Get ClusterVersion resource
	gvk := parseGVK("config.openshift.io/v1", "ClusterVersion")
	opts := api.ListOptions{}

	resources, err := params.MustGatherProvider.ListResources(params.Context, gvk, "", opts)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get cluster version: %w", err)), nil
	}

	if len(resources.Items) == 0 {
		return api.NewToolCallResult("", fmt.Errorf("cluster version not found")), nil
	}

	cv := &resources.Items[0]

	output := "OpenShift Cluster Version\n"
	output += strings.Repeat("=", 80) + "\n\n"

	// Cluster ID
	if clusterID, ok := getNestedString(cv, "spec", "clusterID"); ok {
		output += fmt.Sprintf("Cluster ID: %s\n", clusterID)
	}

	// Current version
	if version, ok := getNestedString(cv, "status", "desired", "version"); ok {
		output += fmt.Sprintf("Current Version: %s\n", version)
	}

	// Image
	if image, ok := getNestedString(cv, "status", "desired", "image"); ok {
		// Show shortened image reference
		if len(image) > 100 {
			output += fmt.Sprintf("Image: %s...\n", image[:100])
		} else {
			output += fmt.Sprintf("Image: %s\n", image)
		}
	}

	output += "\n"

	// Conditions
	conditions, found, err := unstructured.NestedSlice(cv.Object, "status", "conditions")
	if err == nil && found {
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

			symbol := getStatusSymbol(status)
			output += fmt.Sprintf("%s %s: %s\n", symbol, condType, status)

			if reason != "" && reason != "AsExpected" {
				output += fmt.Sprintf("  Reason: %s\n", reason)
			}

			if message != "" {
				// Truncate long messages
				if len(message) > 200 {
					message = message[:200] + "..."
				}
				output += fmt.Sprintf("  Message: %s\n", message)
			}
			output += "\n"
		}
	}

	// Capabilities
	enabledCaps, found, _ := unstructured.NestedStringSlice(cv.Object, "status", "capabilities", "enabledCapabilities")
	if found && len(enabledCaps) > 0 {
		output += fmt.Sprintf("Enabled Capabilities (%d):\n", len(enabledCaps))
		for _, cap := range enabledCaps {
			output += fmt.Sprintf("  - %s\n", cap)
		}
		output += "\n"
	}

	// History (show recent versions)
	history, found, _ := unstructured.NestedSlice(cv.Object, "status", "history")
	if found && len(history) > 0 {
		output += "Version History (most recent 3):\n"
		output += strings.Repeat("-", 80) + "\n"

		count := 0
		for _, h := range history {
			if count >= 3 {
				break
			}

			histMap, ok := h.(map[string]interface{})
			if !ok {
				continue
			}

			version, _ := histMap["version"].(string)
			state, _ := histMap["state"].(string)
			startedTime, _ := histMap["startedTime"].(string)
			completionTime, _ := histMap["completionTime"].(string)

			output += fmt.Sprintf("%d. Version: %s (State: %s)\n", count+1, version, state)
			output += fmt.Sprintf("   Started: %s\n", startedTime)
			if completionTime != "" {
				output += fmt.Sprintf("   Completed: %s\n", completionTime)
			}
			output += "\n"
			count++
		}
	}

	return api.NewToolCallResult(output, nil), nil
}

func getNestedString(obj *unstructured.Unstructured, fields ...string) (string, bool) {
	val, found, err := unstructured.NestedString(obj.Object, fields...)
	if err != nil || !found {
		return "", false
	}
	return val, true
}

func getStatusSymbol(status string) string {
	switch strings.ToLower(status) {
	case "true":
		return "✓"
	case "false":
		return "✗"
	default:
		return "?"
	}
}
