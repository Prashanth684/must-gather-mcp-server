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

func machineConfigTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "machineconfig_list",
				Description: "List all MachineConfigs which define OS and systemd configuration for nodes",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"limit": {
							Type:        "integer",
							Description: "Maximum number of configs to return (default: 0 for all)",
						},
					},
				},
			},
			Handler: machineConfigList,
		},
		{
			Tool: api.Tool{
				Name:        "machineconfig_get",
				Description: "Get detailed information about a specific MachineConfig including ignition configuration",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"name": {
							Type:        "string",
							Description: "MachineConfig name",
						},
					},
					Required: []string{"name"},
				},
			},
			Handler: machineConfigGet,
		},
		{
			Tool: api.Tool{
				Name:        "machineconfigpool_status",
				Description: "Get MachineConfigPool status showing which node pools are degraded or updating",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"pool": {
							Type:        "string",
							Description: "Optional: specific pool name (e.g., master, worker)",
						},
					},
				},
			},
			Handler: machineConfigPoolStatus,
		},
		{
			Tool: api.Tool{
				Name:        "machineconfignode_status",
				Description: "Get per-node machine config application status to identify nodes with config issues",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"node": {
							Type:        "string",
							Description: "Optional: specific node name",
						},
						"degraded": {
							Type:        "boolean",
							Description: "Filter to show only degraded nodes (default: false)",
						},
					},
				},
			},
			Handler: machineConfigNodeStatus,
		},
	}
}

func machineConfigList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	limit := params.GetInt("limit", 0)

	gvk := schema.GroupVersionKind{
		Group:   "machineconfiguration.openshift.io",
		Version: "v1",
		Kind:    "MachineConfig",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, "", api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list MachineConfigs: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No MachineConfigs found", nil), nil
	}

	// Sort by name
	items := list.Items
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetName() < items[j].GetName()
	})

	// Apply limit if specified
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	output := fmt.Sprintf("MachineConfigs (showing %d of %d):\n\n", len(items), len(list.Items))
	output += fmt.Sprintf("%-50s %-10s %-15s\n", "NAME", "GENERATION", "ROLE")
	output += strings.Repeat("-", 80) + "\n"

	for _, mc := range items {
		name := mc.GetName()
		generation := mc.GetGeneration()

		// Get role from labels
		role := "-"
		labels := mc.GetLabels()
		if labels != nil {
			if r, ok := labels["machineconfiguration.openshift.io/role"]; ok {
				role = r
			}
		}

		output += fmt.Sprintf("%-50s %-10d %-15s\n", name, generation, role)
	}

	return api.NewToolCallResult(output, nil), nil
}

func machineConfigGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	name := params.GetString("name", "")
	if name == "" {
		return api.NewToolCallResult("", fmt.Errorf("name is required")), nil
	}

	gvk := schema.GroupVersionKind{
		Group:   "machineconfiguration.openshift.io",
		Version: "v1",
		Kind:    "MachineConfig",
	}

	mc, err := params.MustGatherProvider.GetResource(context.Background(), gvk, "", name)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get MachineConfig: %w", err)), nil
	}

	output := fmt.Sprintf("MachineConfig: %s\n", name)
	output += strings.Repeat("=", 80) + "\n\n"

	// Basic metadata
	output += fmt.Sprintf("Generation: %d\n", mc.GetGeneration())
	output += fmt.Sprintf("Created: %s\n\n", mc.GetCreationTimestamp())

	// Labels
	labels := mc.GetLabels()
	if len(labels) > 0 {
		output += "Labels:\n"
		for k, v := range labels {
			output += fmt.Sprintf("  %s: %s\n", k, v)
		}
		output += "\n"
	}

	// Get spec fields
	spec, found, err := unstructured.NestedMap(mc.Object, "spec")
	if err == nil && found {
		// FIPS
		if fips, ok, _ := unstructured.NestedBool(spec, "fips"); ok {
			output += fmt.Sprintf("FIPS: %v\n", fips)
		}

		// Kernel Type
		if kernelType, ok, _ := unstructured.NestedString(spec, "kernelType"); ok && kernelType != "" {
			output += fmt.Sprintf("Kernel Type: %s\n", kernelType)
		}

		// OSImageURL
		if osImageURL, ok, _ := unstructured.NestedString(spec, "osImageURL"); ok && osImageURL != "" {
			output += fmt.Sprintf("OS Image URL: %s\n", osImageURL)
		}

		output += "\n"

		// Config (ignition)
		if config, ok, _ := unstructured.NestedMap(spec, "config"); ok && len(config) > 0 {
			output += "## Ignition Config\n\n"

			// Storage
			if storage, ok, _ := unstructured.NestedMap(config, "storage"); ok {
				// Files
				if files, ok, _ := unstructured.NestedSlice(storage, "files"); ok && len(files) > 0 {
					output += fmt.Sprintf("Files (%d):\n", len(files))
					for _, f := range files {
						if fileMap, ok := f.(map[string]interface{}); ok {
							if path, ok, _ := unstructured.NestedString(fileMap, "path"); ok {
								output += fmt.Sprintf("  - %s\n", path)
							}
						}
					}
					output += "\n"
				}

				// Filesystems
				if filesystems, ok, _ := unstructured.NestedSlice(storage, "filesystems"); ok && len(filesystems) > 0 {
					output += fmt.Sprintf("Filesystems (%d)\n", len(filesystems))
				}

				// Directories
				if directories, ok, _ := unstructured.NestedSlice(storage, "directories"); ok && len(directories) > 0 {
					output += fmt.Sprintf("Directories (%d)\n", len(directories))
				}

				// Links
				if links, ok, _ := unstructured.NestedSlice(storage, "links"); ok && len(links) > 0 {
					output += fmt.Sprintf("Symlinks (%d)\n", len(links))
				}
			}

			// Systemd units
			if systemd, ok, _ := unstructured.NestedMap(config, "systemd"); ok {
				if units, ok, _ := unstructured.NestedSlice(systemd, "units"); ok && len(units) > 0 {
					output += fmt.Sprintf("\nSystemd Units (%d):\n", len(units))
					for _, u := range units {
						if unitMap, ok := u.(map[string]interface{}); ok {
							if name, ok, _ := unstructured.NestedString(unitMap, "name"); ok {
								enabled := false
								if e, ok, _ := unstructured.NestedBool(unitMap, "enabled"); ok {
									enabled = e
								}
								status := "disabled"
								if enabled {
									status = "enabled"
								}
								output += fmt.Sprintf("  - %s (%s)\n", name, status)
							}
						}
					}
					output += "\n"
				}
			}
		}
	}

	return api.NewToolCallResult(output, nil), nil
}

func machineConfigPoolStatus(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	poolName := params.GetString("pool", "")

	gvk := schema.GroupVersionKind{
		Group:   "machineconfiguration.openshift.io",
		Version: "v1",
		Kind:    "MachineConfigPool",
	}

	var pools []unstructured.Unstructured

	if poolName != "" {
		// Get specific pool
		pool, err := params.MustGatherProvider.GetResource(context.Background(), gvk, "", poolName)
		if err != nil {
			return api.NewToolCallResult("", fmt.Errorf("failed to get MachineConfigPool: %w", err)), nil
		}
		pools = []unstructured.Unstructured{*pool}
	} else {
		// List all pools
		list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, "", api.ListOptions{})
		if err != nil {
			return api.NewToolCallResult("", fmt.Errorf("failed to list MachineConfigPools: %w", err)), nil
		}
		pools = list.Items
	}

	if len(pools) == 0 {
		return api.NewToolCallResult("No MachineConfigPools found", nil), nil
	}

	output := "MachineConfigPool Status:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	for i := range pools {
		pool := &pools[i]
		name := pool.GetName()
		output += fmt.Sprintf("Pool: %s\n", name)
		output += strings.Repeat("-", 40) + "\n"

		// Get status
		status, found, err := unstructured.NestedMap(pool.Object, "status")
		if !found || err != nil {
			output += "  Status: Unknown\n\n"
			continue
		}

		// Machine counts
		machineCount, _, _ := unstructured.NestedInt64(status, "machineCount")
		readyMachineCount, _, _ := unstructured.NestedInt64(status, "readyMachineCount")
		updatedMachineCount, _, _ := unstructured.NestedInt64(status, "updatedMachineCount")
		unavailableMachineCount, _, _ := unstructured.NestedInt64(status, "unavailableMachineCount")
		degradedMachineCount, _, _ := unstructured.NestedInt64(status, "degradedMachineCount")

		output += fmt.Sprintf("  Total Machines: %d\n", machineCount)
		output += fmt.Sprintf("  Ready: %d\n", readyMachineCount)
		output += fmt.Sprintf("  Updated: %d\n", updatedMachineCount)
		output += fmt.Sprintf("  Unavailable: %d\n", unavailableMachineCount)
		output += fmt.Sprintf("  Degraded: %d\n", degradedMachineCount)

		// Configuration
		if currentConfig, ok, _ := unstructured.NestedString(status, "configuration", "name"); ok {
			output += fmt.Sprintf("  Current Config: %s\n", currentConfig)
		}

		// Conditions
		conditions, _, _ := unstructured.NestedSlice(status, "conditions")
		if len(conditions) > 0 {
			output += "\n  Conditions:\n"
			for _, c := range conditions {
				if condMap, ok := c.(map[string]interface{}); ok {
					condType, _, _ := unstructured.NestedString(condMap, "type")
					condStatus, _, _ := unstructured.NestedString(condMap, "status")
					condReason, _, _ := unstructured.NestedString(condMap, "reason")
					condMessage, _, _ := unstructured.NestedString(condMap, "message")

					symbol := "✓"
					if condStatus != "True" {
						symbol = "✗"
					}
					output += fmt.Sprintf("    %s %s: %s", symbol, condType, condStatus)
					if condReason != "" {
						output += fmt.Sprintf(" (%s)", condReason)
					}
					output += "\n"
					if condMessage != "" && condStatus != "True" {
						output += fmt.Sprintf("       %s\n", condMessage)
					}
				}
			}
		}

		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func machineConfigNodeStatus(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	nodeName := params.GetString("node", "")
	onlyDegraded := params.GetBool("degraded", false)

	gvk := schema.GroupVersionKind{
		Group:   "machineconfiguration.openshift.io",
		Version: "v1",
		Kind:    "MachineConfigNode",
	}

	var nodes []unstructured.Unstructured

	if nodeName != "" {
		// Get specific node
		node, err := params.MustGatherProvider.GetResource(context.Background(), gvk, "", nodeName)
		if err != nil {
			return api.NewToolCallResult("", fmt.Errorf("failed to get MachineConfigNode: %w", err)), nil
		}
		nodes = []unstructured.Unstructured{*node}
	} else {
		// List all nodes
		list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, "", api.ListOptions{})
		if err != nil {
			return api.NewToolCallResult("", fmt.Errorf("failed to list MachineConfigNodes: %w", err)), nil
		}
		nodes = list.Items
	}

	if len(nodes) == 0 {
		return api.NewToolCallResult("No MachineConfigNodes found", nil), nil
	}

	output := "MachineConfigNode Status:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	for i := range nodes {
		node := &nodes[i]
		name := node.GetName()

		// Get status
		status, found, err := unstructured.NestedMap(node.Object, "status")
		if !found || err != nil {
			if !onlyDegraded {
				output += fmt.Sprintf("Node: %s - Status: Unknown\n\n", name)
			}
			continue
		}

		// Check degraded state
		conditions, _, _ := unstructured.NestedSlice(status, "conditions")
		isDegraded := false
		for _, c := range conditions {
			if condMap, ok := c.(map[string]interface{}); ok {
				condType, _, _ := unstructured.NestedString(condMap, "type")
				condStatus, _, _ := unstructured.NestedString(condMap, "status")
				if condType == "Degraded" && condStatus == "True" {
					isDegraded = true
					break
				}
			}
		}

		if onlyDegraded && !isDegraded {
			continue
		}

		output += fmt.Sprintf("Node: %s", name)
		if isDegraded {
			output += " ⚠ DEGRADED"
		}
		output += "\n"
		output += strings.Repeat("-", 40) + "\n"

		// Config info
		if currentConfig, ok, _ := unstructured.NestedString(status, "configVersion", "desired"); ok {
			output += fmt.Sprintf("  Desired Config: %s\n", currentConfig)
		}
		if currentConfig, ok, _ := unstructured.NestedString(status, "configVersion", "current"); ok {
			output += fmt.Sprintf("  Current Config: %s\n", currentConfig)
		}

		// Conditions
		if len(conditions) > 0 {
			output += "\n  Conditions:\n"
			for _, c := range conditions {
				if condMap, ok := c.(map[string]interface{}); ok {
					condType, _, _ := unstructured.NestedString(condMap, "type")
					condStatus, _, _ := unstructured.NestedString(condMap, "status")
					condReason, _, _ := unstructured.NestedString(condMap, "reason")
					condMessage, _, _ := unstructured.NestedString(condMap, "message")

					symbol := "✓"
					if condStatus == "True" && (condType == "Degraded" || condType == "Draining") {
						symbol = "✗"
					} else if condStatus != "True" && condType != "Degraded" && condType != "Draining" {
						symbol = "✗"
					}

					output += fmt.Sprintf("    %s %s: %s", symbol, condType, condStatus)
					if condReason != "" {
						output += fmt.Sprintf(" (%s)", condReason)
					}
					output += "\n"
					if condMessage != "" && symbol == "✗" {
						output += fmt.Sprintf("       %s\n", condMessage)
					}
				}
			}
		}

		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}
