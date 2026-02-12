package core

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func resourcesTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "resources_get",
				Description: "Get a specific Kubernetes resource from must-gather by kind, name, and optional namespace",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"kind":       {Type: "string", Description: "Resource kind (e.g., Pod, Deployment, Node, ClusterOperator)"},
						"name":       {Type: "string", Description: "Resource name"},
						"namespace":  {Type: "string", Description: "Namespace (optional for cluster-scoped resources)"},
						"apiVersion": {Type: "string", Description: "API version (e.g., v1, apps/v1). Defaults to v1 for core resources."},
					},
					Required: []string{"kind", "name"},
				},
			},
			Handler: resourcesGet,
		},
		{
			Tool: api.Tool{
				Name:        "resources_list",
				Description: "List Kubernetes resources from must-gather with optional filtering by namespace and labels",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"kind":          {Type: "string", Description: "Resource kind (e.g., Pod, Deployment, Node, ClusterOperator)"},
						"namespace":     {Type: "string", Description: "Namespace (empty for all namespaces or cluster-scoped resources)"},
						"apiVersion":    {Type: "string", Description: "API version (e.g., v1, apps/v1). Defaults to v1 for core resources."},
						"labelSelector": {Type: "string", Description: "Label selector (e.g., 'app=nginx' or 'app=nginx,tier=frontend')"},
						"fieldSelector": {Type: "string", Description: "Field selector (e.g., 'status.phase=Running')"},
						"limit":         {Type: "integer", Description: "Maximum number of results to return"},
					},
					Required: []string{"kind"},
				},
			},
			Handler: resourcesList,
		},
	}
}

func resourcesGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	kind := params.GetString("kind", "")
	name := params.GetString("name", "")
	namespace := params.GetString("namespace", "")
	apiVersion := params.GetString("apiVersion", "v1")

	if kind == "" || name == "" {
		return api.NewToolCallResult("", fmt.Errorf("kind and name are required")), nil
	}

	// Parse GVK
	gvk := parseGVK(apiVersion, kind)

	// Get resource
	resource, err := params.MustGatherProvider.GetResource(params.Context, gvk, namespace, name)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get resource: %w", err)), nil
	}

	// Format as YAML
	output, err := yaml.Marshal(resource.Object)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to marshal resource: %w", err)), nil
	}

	return api.NewToolCallResult(string(output), nil), nil
}

func resourcesList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	kind := params.GetString("kind", "")
	namespace := params.GetString("namespace", "")
	apiVersion := params.GetString("apiVersion", "v1")
	labelSelector := params.GetString("labelSelector", "")
	fieldSelector := params.GetString("fieldSelector", "")
	limit := params.GetInt("limit", 0)

	if kind == "" {
		return api.NewToolCallResult("", fmt.Errorf("kind is required")), nil
	}

	// Parse GVK
	gvk := parseGVK(apiVersion, kind)

	// List resources
	opts := api.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
		Limit:         limit,
	}

	resources, err := params.MustGatherProvider.ListResources(params.Context, gvk, namespace, opts)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list resources: %w", err)), nil
	}

	// Format output
	output := fmt.Sprintf("Found %d resources:\n\n", len(resources.Items))

	// If no resources found
	if len(resources.Items) == 0 {
		output += "No resources found matching the criteria.\n"
		return api.NewToolCallResult(output, nil), nil
	}

	// Show summary list
	for i, resource := range resources.Items {
		name := resource.GetName()
		ns := resource.GetNamespace()
		if ns != "" {
			output += fmt.Sprintf("%d. %s/%s\n", i+1, ns, name)
		} else {
			output += fmt.Sprintf("%d. %s\n", i+1, name)
		}
	}

	// If limit is small (5 or less), show full YAML
	if len(resources.Items) <= 5 {
		output += "\n--- Full Resource Details ---\n\n"
		for i, resource := range resources.Items {
			output += fmt.Sprintf("# Resource %d\n", i+1)
			yaml, err := yaml.Marshal(resource.Object)
			if err == nil {
				output += string(yaml)
				output += "\n---\n\n"
			}
		}
	} else {
		output += "\n(Showing summary only. Use resources_get to view individual resources)\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

// parseGVK parses apiVersion and kind into GroupVersionKind
func parseGVK(apiVersion, kind string) schema.GroupVersionKind {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		// If parsing fails, assume it's a version only (no group)
		gv = schema.GroupVersion{Version: apiVersion}
	}

	return schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    kind,
	}
}
