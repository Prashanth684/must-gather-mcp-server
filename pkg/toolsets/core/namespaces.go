package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
)

func namespacesTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "namespaces_list",
				Description: "List all namespaces in the must-gather",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: namespacesList,
		},
	}
}

func namespacesList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	namespaces, err := params.MustGatherProvider.ListNamespaces(params.Context)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list namespaces: %w", err)), nil
	}

	// Sort alphabetically
	sort.Strings(namespaces)

	output := fmt.Sprintf("Found %d namespaces:\n\n", len(namespaces))
	output += strings.Join(namespaces, "\n")
	output += "\n"

	return api.NewToolCallResult(output, nil), nil
}
