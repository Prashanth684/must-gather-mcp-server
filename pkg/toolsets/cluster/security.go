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

func securityTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "security_scc_list",
				Description: "List SecurityContextConstraints (SCCs) showing which users/service accounts have privileged access",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"scc": {
							Type:        "string",
							Description: "Optional: specific SCC name to inspect",
						},
					},
				},
			},
			Handler: securitySCCList,
		},
		{
			Tool: api.Tool{
				Name:        "rbac_clusterroles_list",
				Description: "List ClusterRoles to understand cluster-wide permissions and identify overly permissive roles",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"role": {
							Type:        "string",
							Description: "Optional: specific role name to inspect (partial match)",
						},
						"aggregated": {
							Type:        "boolean",
							Description: "Show only aggregated roles (default: false)",
						},
					},
				},
			},
			Handler: rbacClusterRolesList,
		},
	}
}

func securitySCCList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	sccFilter := params.GetString("scc", "")

	gvk := schema.GroupVersionKind{
		Group:   "security.openshift.io",
		Version: "v1",
		Kind:    "SecurityContextConstraints",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, "", api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list SecurityContextConstraints: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No SecurityContextConstraints found", nil), nil
	}

	output := "SecurityContextConstraints:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	for i := range list.Items {
		scc := &list.Items[i]
		name := scc.GetName()

		if sccFilter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(sccFilter)) {
			continue
		}

		output += fmt.Sprintf("SCC: %s\n", name)
		output += strings.Repeat("-", 40) + "\n"

		// Get key security settings
		allowPrivilegedContainer, _, _ := unstructured.NestedBool(scc.Object, "allowPrivilegedContainer")
		allowHostDirVolumePlugin, _, _ := unstructured.NestedBool(scc.Object, "allowHostDirVolumePlugin")
		allowHostNetwork, _, _ := unstructured.NestedBool(scc.Object, "allowHostNetwork")
		allowHostPID, _, _ := unstructured.NestedBool(scc.Object, "allowHostPID")
		allowHostIPC, _, _ := unstructured.NestedBool(scc.Object, "allowHostIPC")
		allowHostPorts, _, _ := unstructured.NestedBool(scc.Object, "allowHostPorts")

		// Security level indicator
		securityLevel := "Restricted"
		if allowPrivilegedContainer || allowHostNetwork || allowHostPID || allowHostIPC {
			securityLevel = "Privileged ⚠"
		} else if allowHostDirVolumePlugin || allowHostPorts {
			securityLevel = "Permissive"
		}

		output += fmt.Sprintf("  Security Level: %s\n", securityLevel)
		output += fmt.Sprintf("  Privileged Containers: %v\n", allowPrivilegedContainer)
		output += fmt.Sprintf("  Host Network: %v\n", allowHostNetwork)
		output += fmt.Sprintf("  Host PID: %v\n", allowHostPID)
		output += fmt.Sprintf("  Host IPC: %v\n", allowHostIPC)

		// Users
		users, _, _ := unstructured.NestedStringSlice(scc.Object, "users")
		if len(users) > 0 {
			output += fmt.Sprintf("\n  Users (%d):\n", len(users))
			for _, user := range users {
				output += fmt.Sprintf("    - %s\n", user)
			}
		}

		// Groups
		groups, _, _ := unstructured.NestedStringSlice(scc.Object, "groups")
		if len(groups) > 0 {
			output += fmt.Sprintf("\n  Groups (%d):\n", len(groups))
			for _, group := range groups {
				output += fmt.Sprintf("    - %s\n", group)
			}
		}

		// RunAsUser strategy
		if runAsUser, ok, _ := unstructured.NestedMap(scc.Object, "runAsUser"); ok {
			if strategy, ok, _ := unstructured.NestedString(runAsUser, "type"); ok {
				output += fmt.Sprintf("\n  RunAsUser Strategy: %s\n", strategy)
			}
		}

		// SELinux strategy
		if seLinux, ok, _ := unstructured.NestedMap(scc.Object, "seLinuxContext"); ok {
			if strategy, ok, _ := unstructured.NestedString(seLinux, "type"); ok {
				output += fmt.Sprintf("  SELinux Strategy: %s\n", strategy)
			}
		}

		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func rbacClusterRolesList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	roleFilter := params.GetString("role", "")
	onlyAggregated := params.GetBool("aggregated", false)

	gvk := schema.GroupVersionKind{
		Group:   "rbac.authorization.k8s.io",
		Version: "v1",
		Kind:    "ClusterRole",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, "", api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list ClusterRoles: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No ClusterRoles found", nil), nil
	}

	// Sort by name
	items := list.Items
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetName() < items[j].GetName()
	})

	output := "ClusterRoles:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	roleCount := 0

	for i := range items {
		role := &items[i]
		name := role.GetName()

		if roleFilter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(roleFilter)) {
			continue
		}

		// Check if aggregated
		aggregationRule, isAggregated, _ := unstructured.NestedMap(role.Object, "aggregationRule")
		if onlyAggregated && !isAggregated {
			continue
		}

		roleCount++

		output += fmt.Sprintf("ClusterRole: %s\n", name)

		// Check if system role
		labels := role.GetLabels()
		if labels != nil {
			if rbacAgg, ok := labels["rbac.authorization.k8s.io/aggregate-to-admin"]; ok && rbacAgg == "true" {
				output += "  Aggregates to: admin\n"
			}
			if rbacAgg, ok := labels["rbac.authorization.k8s.io/aggregate-to-edit"]; ok && rbacAgg == "true" {
				output += "  Aggregates to: edit\n"
			}
			if rbacAgg, ok := labels["rbac.authorization.k8s.io/aggregate-to-view"]; ok && rbacAgg == "true" {
				output += "  Aggregates to: view\n"
			}
		}

		// If aggregated, show what it aggregates
		if isAggregated {
			output += "  Type: Aggregated Role\n"
			if clusterRoleSelectors, ok, _ := unstructured.NestedSlice(aggregationRule, "clusterRoleSelectors"); ok {
				output += fmt.Sprintf("  Aggregates %d role selector(s)\n", len(clusterRoleSelectors))
			}
		}

		// Get rules
		rules, _, _ := unstructured.NestedSlice(role.Object, "rules")
		if len(rules) > 0 {
			output += fmt.Sprintf("  Rules: %d\n", len(rules))

			// Check for dangerous permissions
			hasDangerousPerms := false
			dangerousPerms := []string{}

			for _, r := range rules {
				if ruleMap, ok := r.(map[string]interface{}); ok {
					verbs, _, _ := unstructured.NestedStringSlice(ruleMap, "verbs")
					resources, _, _ := unstructured.NestedStringSlice(ruleMap, "resources")

					// Check for wildcard permissions
					hasWildcardVerb := false
					for _, verb := range verbs {
						if verb == "*" {
							hasWildcardVerb = true
							break
						}
					}

					hasWildcardResource := false
					for _, res := range resources {
						if res == "*" {
							hasWildcardResource = true
							break
						}
					}

					if hasWildcardVerb && hasWildcardResource {
						dangerousPerms = append(dangerousPerms, "full wildcard access (*/*)")
						hasDangerousPerms = true
					}

					// Check for specific dangerous resources
					for _, res := range resources {
						for _, verb := range verbs {
							if res == "secrets" && (verb == "*" || verb == "get" || verb == "list") {
								if !contains(dangerousPerms, "secrets access") {
									dangerousPerms = append(dangerousPerms, "secrets access")
									hasDangerousPerms = true
								}
							}
							if res == "pods/exec" && (verb == "*" || verb == "create") {
								if !contains(dangerousPerms, "pod exec") {
									dangerousPerms = append(dangerousPerms, "pod exec")
									hasDangerousPerms = true
								}
							}
						}
					}
				}
			}

			if hasDangerousPerms {
				output += "  ⚠ Privileged Permissions: " + strings.Join(dangerousPerms, ", ") + "\n"
			}
		}

		output += "\n"
	}

	output = fmt.Sprintf("Found %d matching ClusterRoles (total: %d)\n\n", roleCount, len(list.Items)) + output

	return api.NewToolCallResult(output, nil), nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
