package cluster

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func nodeTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "cluster_nodes_list",
				Description: "List all cluster nodes with their status, roles, and key information",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"role": {
							Type:        "string",
							Description: "Filter by role: all, master, worker (default: all)",
							Enum:        []interface{}{"all", "master", "worker"},
						},
					},
				},
			},
			Handler: clusterNodesList,
		},
		{
			Tool: api.Tool{
				Name:        "cluster_node_get",
				Description: "Get detailed information for a specific node including status, conditions, capacity, and system info",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"name": {
							Type:        "string",
							Description: "Node name",
						},
					},
					Required: []string{"name"},
				},
			},
			Handler: clusterNodeGet,
		},
	}
}

func clusterNodesList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	roleFilter := params.GetString("role", "all")

	gvk := parseGVK("v1", "Node")
	opts := api.ListOptions{}

	nodeList, err := params.MustGatherProvider.ListResources(params.Context, gvk, "", opts)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list nodes: %w", err)), nil
	}

	nodes := nodeList.Items
	if len(nodes) == 0 {
		return api.NewToolCallResult("No nodes found", nil), nil
	}

	// Sort by name
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].GetName() < nodes[j].GetName()
	})

	output := "Cluster Nodes\n"
	output += strings.Repeat("=", 80) + "\n\n"

	// Table header
	output += fmt.Sprintf("%-40s %-15s %-10s %-10s\n", "NAME", "ROLES", "STATUS", "VERSION")
	output += strings.Repeat("-", 80) + "\n"

	filteredCount := 0
	for i := range nodes {
		node := &nodes[i]
		name := node.GetName()
		labels := node.GetLabels()

		// Determine roles
		roles := getNodeRoles(labels)
		rolesStr := strings.Join(roles, ",")
		if rolesStr == "" {
			rolesStr = "<none>"
		}

		// Apply role filter
		if roleFilter != "all" {
			hasRole := false
			for _, role := range roles {
				if strings.EqualFold(role, roleFilter) {
					hasRole = true
					break
				}
			}
			if !hasRole {
				continue
			}
		}

		filteredCount++

		// Get status
		status := getNodeStatus(node)

		// Get kubelet version
		version, _ := getNestedString(node, "status", "nodeInfo", "kubeletVersion")

		output += fmt.Sprintf("%-40s %-15s %-10s %-10s\n",
			truncate(name, 40), truncate(rolesStr, 15), status, version)
	}

	output += fmt.Sprintf("\nTotal Nodes: %d", filteredCount)
	if roleFilter != "all" {
		output += fmt.Sprintf(" (filtered by role: %s)", roleFilter)
	}
	output += "\n"

	return api.NewToolCallResult(output, nil), nil
}

func clusterNodeGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	name := params.GetString("name", "")
	if name == "" {
		return api.NewToolCallResult("", fmt.Errorf("node name is required")), nil
	}

	gvk := parseGVK("v1", "Node")
	opts := api.ListOptions{}

	nodeList, err := params.MustGatherProvider.ListResources(params.Context, gvk, "", opts)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get node: %w", err)), nil
	}

	// Find the specific node by name
	var node *unstructured.Unstructured
	for i := range nodeList.Items {
		if nodeList.Items[i].GetName() == name {
			node = &nodeList.Items[i]
			break
		}
	}

	if node == nil {
		return api.NewToolCallResult("", fmt.Errorf("node '%s' not found", name)), nil
	}

	output := fmt.Sprintf("Node: %s\n", name)
	output += strings.Repeat("=", 80) + "\n\n"

	// Roles
	labels := node.GetLabels()
	roles := getNodeRoles(labels)
	output += fmt.Sprintf("Roles: %s\n", strings.Join(roles, ", "))

	// Status
	status := getNodeStatus(node)
	output += fmt.Sprintf("Status: %s\n\n", status)

	// Node Info
	output += "System Information:\n"
	output += strings.Repeat("-", 80) + "\n"

	if osImage, ok := getNestedString(node, "status", "nodeInfo", "osImage"); ok {
		output += fmt.Sprintf("OS Image: %s\n", osImage)
	}
	if kernelVersion, ok := getNestedString(node, "status", "nodeInfo", "kernelVersion"); ok {
		output += fmt.Sprintf("Kernel Version: %s\n", kernelVersion)
	}
	if containerRuntime, ok := getNestedString(node, "status", "nodeInfo", "containerRuntimeVersion"); ok {
		output += fmt.Sprintf("Container Runtime: %s\n", containerRuntime)
	}
	if kubeletVersion, ok := getNestedString(node, "status", "nodeInfo", "kubeletVersion"); ok {
		output += fmt.Sprintf("Kubelet Version: %s\n", kubeletVersion)
	}
	if kubeProxyVersion, ok := getNestedString(node, "status", "nodeInfo", "kubeProxyVersion"); ok {
		output += fmt.Sprintf("Kube-Proxy Version: %s\n", kubeProxyVersion)
	}
	if machineID, ok := getNestedString(node, "status", "nodeInfo", "machineID"); ok {
		output += fmt.Sprintf("Machine ID: %s\n", machineID)
	}
	if systemUUID, ok := getNestedString(node, "status", "nodeInfo", "systemUUID"); ok {
		output += fmt.Sprintf("System UUID: %s\n", systemUUID)
	}

	output += "\n"

	// Capacity and Allocatable
	output += "Resources:\n"
	output += strings.Repeat("-", 80) + "\n"

	if capacity, found, _ := unstructured.NestedMap(node.Object, "status", "capacity"); found {
		output += "Capacity:\n"
		for k, v := range capacity {
			output += fmt.Sprintf("  %s: %v\n", k, v)
		}
	}

	if allocatable, found, _ := unstructured.NestedMap(node.Object, "status", "allocatable"); found {
		output += "Allocatable:\n"
		for k, v := range allocatable {
			output += fmt.Sprintf("  %s: %v\n", k, v)
		}
	}

	output += "\n"

	// Addresses
	addresses, found, _ := unstructured.NestedSlice(node.Object, "status", "addresses")
	if found && len(addresses) > 0 {
		output += "Addresses:\n"
		output += strings.Repeat("-", 80) + "\n"

		for _, addr := range addresses {
			if addrMap, ok := addr.(map[string]interface{}); ok {
				addrType, _ := addrMap["type"].(string)
				address, _ := addrMap["address"].(string)
				output += fmt.Sprintf("  %s: %s\n", addrType, address)
			}
		}
		output += "\n"
	}

	// Conditions
	conditions, found, _ := unstructured.NestedSlice(node.Object, "status", "conditions")
	if found && len(conditions) > 0 {
		output += "Conditions:\n"
		output += strings.Repeat("-", 80) + "\n"

		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _ := condMap["type"].(string)
			status, _ := condMap["status"].(string)
			reason, _ := condMap["reason"].(string)
			message, _ := condMap["message"].(string)

			symbol := getStatusSymbol(status)
			output += fmt.Sprintf("%s %s: %s\n", symbol, condType, status)

			if reason != "" {
				output += fmt.Sprintf("  Reason: %s\n", reason)
			}

			if message != "" && len(message) < 100 {
				output += fmt.Sprintf("  Message: %s\n", message)
			}
		}
		output += "\n"
	}

	// Taints
	taints, found, _ := unstructured.NestedSlice(node.Object, "spec", "taints")
	if found && len(taints) > 0 {
		output += "Taints:\n"
		output += strings.Repeat("-", 80) + "\n"

		for _, taint := range taints {
			if taintMap, ok := taint.(map[string]interface{}); ok {
				key, _ := taintMap["key"].(string)
				value, _ := taintMap["value"].(string)
				effect, _ := taintMap["effect"].(string)

				if value != "" {
					output += fmt.Sprintf("  %s=%s:%s\n", key, value, effect)
				} else {
					output += fmt.Sprintf("  %s:%s\n", key, effect)
				}
			}
		}
		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func getNodeRoles(labels map[string]string) []string {
	roles := []string{}

	for key := range labels {
		if strings.HasPrefix(key, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(key, "node-role.kubernetes.io/")
			if role != "" {
				roles = append(roles, role)
			}
		}
	}

	sort.Strings(roles)
	return roles
}

func getNodeStatus(node *unstructured.Unstructured) string {
	conditions, found, _ := unstructured.NestedSlice(node.Object, "status", "conditions")
	if !found {
		return "Unknown"
	}

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}

		if condType, ok := condMap["type"].(string); ok && condType == "Ready" {
			if status, ok := condMap["status"].(string); ok {
				if status == "True" {
					return "Ready"
				} else {
					return "NotReady"
				}
			}
		}
	}

	return "Unknown"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
