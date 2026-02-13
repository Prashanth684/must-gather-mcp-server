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

func storageTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "storage_classes_list",
				Description: "List all StorageClasses showing provisioners, reclaim policies, and which is the default class",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: storageClassesList,
		},
		{
			Tool: api.Tool{
				Name:        "csi_drivers_status",
				Description: "Get CSI driver status and capabilities for debugging storage provisioning issues",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"driver": {
							Type:        "string",
							Description: "Optional: specific driver name to inspect",
						},
					},
				},
			},
			Handler: csiDriversStatus,
		},
		{
			Tool: api.Tool{
				Name:        "volume_attachments_list",
				Description: "List VolumeAttachments to find stuck or failing volume mount operations",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"node": {
							Type:        "string",
							Description: "Filter by node name (partial match)",
						},
						"attached": {
							Type:        "string",
							Description: "Filter by attachment status: all, true, false (default: all)",
							Enum:        []interface{}{"all", "true", "false"},
						},
					},
				},
			},
			Handler: volumeAttachmentsList,
		},
		{
			Tool: api.Tool{
				Name:        "persistent_volumes_status",
				Description: "Get PersistentVolume status showing available, bound, and failed volumes",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"phase": {
							Type:        "string",
							Description: "Filter by phase: all, Available, Bound, Released, Failed (default: all)",
							Enum:        []interface{}{"all", "Available", "Bound", "Released", "Failed"},
						},
					},
				},
			},
			Handler: persistentVolumesStatus,
		},
	}
}

func storageClassesList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	gvk := schema.GroupVersionKind{
		Group:   "storage.k8s.io",
		Version: "v1",
		Kind:    "StorageClass",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, "", api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list StorageClasses: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No StorageClasses found", nil), nil
	}

	// Sort by name
	items := list.Items
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetName() < items[j].GetName()
	})

	output := fmt.Sprintf("StorageClasses (%d):\n\n", len(items))
	output += fmt.Sprintf("%-40s %-30s %-15s %-10s\n", "NAME", "PROVISIONER", "RECLAIM", "DEFAULT")
	output += strings.Repeat("-", 100) + "\n"

	for i := range items {
		sc := &items[i]
		name := sc.GetName()

		provisioner, _, _ := unstructured.NestedString(sc.Object, "provisioner")
		reclaimPolicy, _, _ := unstructured.NestedString(sc.Object, "reclaimPolicy")
		volumeBindingMode, _, _ := unstructured.NestedString(sc.Object, "volumeBindingMode")

		// Check if default
		isDefault := "no"
		annotations := sc.GetAnnotations()
		if annotations != nil {
			if val, ok := annotations["storageclass.kubernetes.io/is-default-class"]; ok && val == "true" {
				isDefault = "yes ⭐"
			}
		}

		if reclaimPolicy == "" {
			reclaimPolicy = "Delete"
		}

		output += fmt.Sprintf("%-40s %-30s %-15s %-10s\n", name, provisioner, reclaimPolicy, isDefault)

		if volumeBindingMode != "" && volumeBindingMode != "Immediate" {
			output += fmt.Sprintf("  Binding Mode: %s\n", volumeBindingMode)
		}
	}

	return api.NewToolCallResult(output, nil), nil
}

func csiDriversStatus(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	driverFilter := params.GetString("driver", "")

	gvk := schema.GroupVersionKind{
		Group:   "storage.k8s.io",
		Version: "v1",
		Kind:    "CSIDriver",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, "", api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list CSIDrivers: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No CSIDrivers found", nil), nil
	}

	output := "CSI Drivers:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	for i := range list.Items {
		driver := &list.Items[i]
		name := driver.GetName()

		if driverFilter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(driverFilter)) {
			continue
		}

		output += fmt.Sprintf("Driver: %s\n", name)
		output += strings.Repeat("-", 40) + "\n"

		// Get spec
		spec, found, _ := unstructured.NestedMap(driver.Object, "spec")
		if found {
			if attachRequired, ok, _ := unstructured.NestedBool(spec, "attachRequired"); ok {
				output += fmt.Sprintf("  Attach Required: %v\n", attachRequired)
			}
			if podInfoOnMount, ok, _ := unstructured.NestedBool(spec, "podInfoOnMount"); ok {
				output += fmt.Sprintf("  Pod Info On Mount: %v\n", podInfoOnMount)
			}
			if volumeLifecycleModes, ok, _ := unstructured.NestedStringSlice(spec, "volumeLifecycleModes"); ok && len(volumeLifecycleModes) > 0 {
				output += fmt.Sprintf("  Volume Lifecycle Modes: %v\n", volumeLifecycleModes)
			}
			if storageCapacity, ok, _ := unstructured.NestedBool(spec, "storageCapacity"); ok {
				output += fmt.Sprintf("  Storage Capacity Tracking: %v\n", storageCapacity)
			}
		}

		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func volumeAttachmentsList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	nodeFilter := params.GetString("node", "")
	attachedFilter := params.GetString("attached", "all")

	gvk := schema.GroupVersionKind{
		Group:   "storage.k8s.io",
		Version: "v1",
		Kind:    "VolumeAttachment",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, "", api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list VolumeAttachments: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No VolumeAttachments found", nil), nil
	}

	output := "VolumeAttachments:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	attached := 0
	detached := 0

	for i := range list.Items {
		va := &list.Items[i]
		name := va.GetName()

		// Get spec
		spec, found, _ := unstructured.NestedMap(va.Object, "spec")
		if !found {
			continue
		}

		nodeName, _, _ := unstructured.NestedString(spec, "nodeName")
		attacher, _, _ := unstructured.NestedString(spec, "attacher")

		// Apply node filter
		if nodeFilter != "" && !strings.Contains(strings.ToLower(nodeName), strings.ToLower(nodeFilter)) {
			continue
		}

		// Get status
		status, found, _ := unstructured.NestedMap(va.Object, "status")
		isAttached := false
		if found {
			isAttached, _, _ = unstructured.NestedBool(status, "attached")
		}

		// Apply attached filter
		if attachedFilter == "true" && !isAttached {
			continue
		}
		if attachedFilter == "false" && isAttached {
			continue
		}

		if isAttached {
			attached++
		} else {
			detached++
		}

		attachStatus := "✗ Not Attached"
		if isAttached {
			attachStatus = "✓ Attached"
		}

		output += fmt.Sprintf("VolumeAttachment: %s\n", name)
		output += fmt.Sprintf("  Node: %s\n", nodeName)
		output += fmt.Sprintf("  Attacher: %s\n", attacher)
		output += fmt.Sprintf("  Status: %s\n", attachStatus)

		// Show attach error if not attached
		if !isAttached && status != nil {
			if attachError, ok, _ := unstructured.NestedMap(status, "attachError"); ok {
				if message, ok, _ := unstructured.NestedString(attachError, "message"); ok {
					output += fmt.Sprintf("  Error: %s\n", message)
				}
			}
		}

		output += "\n"
	}

	output = fmt.Sprintf("Summary: %d attached, %d not attached (total: %d)\n\n", attached, detached, len(list.Items)) + output

	return api.NewToolCallResult(output, nil), nil
}

func persistentVolumesStatus(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	phaseFilter := params.GetString("phase", "all")

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "PersistentVolume",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, "", api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list PersistentVolumes: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No PersistentVolumes found", nil), nil
	}

	// Count by phase
	phaseCounts := make(map[string]int)

	output := "PersistentVolumes:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	for i := range list.Items {
		pv := &list.Items[i]
		name := pv.GetName()

		// Get status
		status, found, _ := unstructured.NestedMap(pv.Object, "status")
		if !found {
			continue
		}

		phase, _, _ := unstructured.NestedString(status, "phase")

		// Count phases
		phaseCounts[phase]++

		// Apply filter
		if phaseFilter != "all" && phase != phaseFilter {
			continue
		}

		// Get spec
		spec, _, _ := unstructured.NestedMap(pv.Object, "spec")

		storageClassName, _, _ := unstructured.NestedString(spec, "storageClassName")
		capacity, _, _ := unstructured.NestedMap(spec, "capacity")
		storageSize := ""
		if capacity != nil {
			if size, ok := capacity["storage"]; ok {
				storageSize = fmt.Sprintf("%v", size)
			}
		}

		claimRef, _, _ := unstructured.NestedMap(spec, "claimRef")
		claimName := ""
		claimNamespace := ""
		if claimRef != nil {
			claimName, _, _ = unstructured.NestedString(claimRef, "name")
			claimNamespace, _, _ = unstructured.NestedString(claimRef, "namespace")
		}

		phaseSymbol := "ℹ"
		if phase == "Bound" {
			phaseSymbol = "✓"
		} else if phase == "Failed" {
			phaseSymbol = "✗"
		}

		output += fmt.Sprintf("%s PersistentVolume: %s\n", phaseSymbol, name)
		output += fmt.Sprintf("  Phase: %s\n", phase)
		if storageSize != "" {
			output += fmt.Sprintf("  Capacity: %s\n", storageSize)
		}
		if storageClassName != "" {
			output += fmt.Sprintf("  Storage Class: %s\n", storageClassName)
		}
		if claimName != "" {
			output += fmt.Sprintf("  Claim: %s/%s\n", claimNamespace, claimName)
		}

		// Show message if failed
		if phase == "Failed" {
			if message, ok, _ := unstructured.NestedString(status, "message"); ok {
				output += fmt.Sprintf("  Message: %s\n", message)
			}
		}

		output += "\n"
	}

	// Add summary at top
	summary := fmt.Sprintf("Summary: ")
	summaryParts := []string{}
	for _, p := range []string{"Bound", "Available", "Released", "Failed"} {
		if count := phaseCounts[p]; count > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%d %s", count, p))
		}
	}
	summary += strings.Join(summaryParts, ", ")
	summary += fmt.Sprintf(" (total: %d)\n\n", len(list.Items))

	output = summary + output

	return api.NewToolCallResult(output, nil), nil
}
