package cluster

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func configTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "cluster_config_list",
				Description: "List all cluster configuration resources (Authentication, Console, FeatureGate, OAuth, Proxy, etc.) from config.openshift.io",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: clusterConfigList,
		},
		{
			Tool: api.Tool{
				Name:        "cluster_config_get",
				Description: "Get detailed configuration for a specific config.openshift.io resource type",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"kind": {
							Type:        "string",
							Description: "Configuration kind (e.g., Authentication, Console, FeatureGate, OAuth, Proxy, Network, DNS, Ingress)",
						},
						"name": {
							Type:        "string",
							Description: "Resource name (usually 'cluster' for singleton configs)",
						},
					},
					Required: []string{"kind"},
				},
			},
			Handler: clusterConfigGet,
		},
	}
}

func clusterConfigList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	output := "Cluster Configuration Resources:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	// List of common config.openshift.io resources
	configKinds := []string{
		"Authentication",
		"Build",
		"Console",
		"DNS",
		"FeatureGate",
		"Image",
		"Ingress",
		"Infrastructure",
		"Network",
		"OAuth",
		"OperatorHub",
		"Project",
		"Proxy",
		"Scheduler",
	}

	for _, kind := range configKinds {
		gvk := schema.GroupVersionKind{
			Group:   "config.openshift.io",
			Version: "v1",
			Kind:    kind,
		}

		list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, "", api.ListOptions{})
		if err != nil || len(list.Items) == 0 {
			continue
		}

		output += fmt.Sprintf("## %s\n", kind)

		for i := range list.Items {
			res := &list.Items[i]
			name := res.GetName()
			output += fmt.Sprintf("  - %s\n", name)
		}
		output += "\n"
	}

	output += "Use cluster_config_get to view detailed configuration for any resource.\n"

	return api.NewToolCallResult(output, nil), nil
}

func clusterConfigGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	kind := params.GetString("kind", "")
	name := params.GetString("name", "cluster")

	if kind == "" {
		return api.NewToolCallResult("", fmt.Errorf("kind is required")), nil
	}

	gvk := schema.GroupVersionKind{
		Group:   "config.openshift.io",
		Version: "v1",
		Kind:    kind,
	}

	resource, err := params.MustGatherProvider.GetResource(context.Background(), gvk, "", name)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get %s/%s: %w", kind, name, err)), nil
	}

	output := fmt.Sprintf("Configuration: %s/%s\n", kind, name)
	output += strings.Repeat("=", 80) + "\n\n"

	// Get spec
	spec, found, _ := unstructured.NestedMap(resource.Object, "spec")
	if found && spec != nil {
		output += "## Spec\n\n"
		output += formatConfig(spec, "  ")
		output += "\n"
	}

	// Get status
	status, found, _ := unstructured.NestedMap(resource.Object, "status")
	if found && status != nil {
		output += "## Status\n\n"
		output += formatConfig(status, "  ")
	}

	return api.NewToolCallResult(output, nil), nil
}

func formatConfig(data map[string]interface{}, indent string) string {
	output := ""

	// Sort keys for consistent output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := data[key]

		switch v := value.(type) {
		case map[string]interface{}:
			output += fmt.Sprintf("%s%s:\n", indent, key)
			output += formatConfig(v, indent+"  ")
		case []interface{}:
			if len(v) == 0 {
				output += fmt.Sprintf("%s%s: []\n", indent, key)
			} else {
				output += fmt.Sprintf("%s%s:\n", indent, key)
				for i, item := range v {
					if itemMap, ok := item.(map[string]interface{}); ok {
						output += fmt.Sprintf("%s  [%d]:\n", indent, i)
						output += formatConfig(itemMap, indent+"    ")
					} else {
						output += fmt.Sprintf("%s  - %v\n", indent, item)
					}
				}
			}
		case string:
			if v != "" {
				output += fmt.Sprintf("%s%s: %s\n", indent, key, v)
			}
		case bool:
			output += fmt.Sprintf("%s%s: %v\n", indent, key, v)
		case float64, int64, int:
			output += fmt.Sprintf("%s%s: %v\n", indent, key, v)
		default:
			if v != nil {
				output += fmt.Sprintf("%s%s: %v\n", indent, key, v)
			}
		}
	}

	return output
}
