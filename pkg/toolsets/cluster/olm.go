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

func olmTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "olm_subscriptions_status",
				Description: "Get Operator Lifecycle Manager subscription status to identify failed operator installations or upgrades",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"namespace": {
							Type:        "string",
							Description: "Filter by namespace (optional)",
						},
						"state": {
							Type:        "string",
							Description: "Filter by state: all, AtLatestKnown, UpgradePending, Upgrading, UpgradeFailed (default: all)",
							Enum:        []interface{}{"all", "AtLatestKnown", "UpgradePending", "Upgrading", "UpgradeFailed"},
						},
					},
				},
			},
			Handler: olmSubscriptionsStatus,
		},
		{
			Tool: api.Tool{
				Name:        "olm_catalogsources_status",
				Description: "Get CatalogSource status to debug operator catalog connectivity and availability issues",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"namespace": {
							Type:        "string",
							Description: "Filter by namespace (default: openshift-marketplace)",
						},
					},
				},
			},
			Handler: olmCatalogSourcesStatus,
		},
		{
			Tool: api.Tool{
				Name:        "olm_installplans_status",
				Description: "Get InstallPlan status showing pending operator installations and approval requirements",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"namespace": {
							Type:        "string",
							Description: "Filter by namespace (optional)",
						},
						"approved": {
							Type:        "string",
							Description: "Filter by approval status: all, true, false (default: all)",
							Enum:        []interface{}{"all", "true", "false"},
						},
					},
				},
			},
			Handler: olmInstallPlansStatus,
		},
	}
}

func olmSubscriptionsStatus(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	namespace := params.GetString("namespace", "")
	stateFilter := params.GetString("state", "all")

	gvk := schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "Subscription",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, namespace, api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list Subscriptions: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No OLM Subscriptions found", nil), nil
	}

	output := "OLM Subscriptions:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	healthyCounts := make(map[string]int)

	for i := range list.Items {
		sub := &list.Items[i]
		name := sub.GetName()
		ns := sub.GetNamespace()

		// Get spec
		spec, _, _ := unstructured.NestedMap(sub.Object, "spec")
		channel := ""
		source := ""
		sourceNamespace := ""
		if spec != nil {
			channel, _, _ = unstructured.NestedString(spec, "channel")
			source, _, _ = unstructured.NestedString(spec, "source")
			sourceNamespace, _, _ = unstructured.NestedString(spec, "sourceNamespace")
		}

		// Get status
		status, _, _ := unstructured.NestedMap(sub.Object, "status")
		state := "Unknown"
		installedCSV := ""
		currentCSV := ""
		if status != nil {
			state, _, _ = unstructured.NestedString(status, "state")
			installedCSV, _, _ = unstructured.NestedString(status, "installedCSV")
			currentCSV, _, _ = unstructured.NestedString(status, "currentCSV")
		}

		// Apply state filter
		if stateFilter != "all" && state != stateFilter {
			continue
		}

		healthyCounts[state]++

		stateSymbol := "ℹ"
		if state == "AtLatestKnown" {
			stateSymbol = "✓"
		} else if state == "UpgradeFailed" || state == "Failed" {
			stateSymbol = "✗"
		} else if state == "Upgrading" || state == "UpgradePending" {
			stateSymbol = "⏳"
		}

		output += fmt.Sprintf("%s Subscription: %s/%s\n", stateSymbol, ns, name)
		output += fmt.Sprintf("  State: %s\n", state)
		output += fmt.Sprintf("  Channel: %s\n", channel)
		output += fmt.Sprintf("  Source: %s/%s\n", sourceNamespace, source)

		if installedCSV != "" {
			output += fmt.Sprintf("  Installed CSV: %s\n", installedCSV)
		}
		if currentCSV != "" && currentCSV != installedCSV {
			output += fmt.Sprintf("  Current CSV: %s\n", currentCSV)
		}

		// Show conditions for non-healthy states
		if state != "AtLatestKnown" && status != nil {
			conditions, _, _ := unstructured.NestedSlice(status, "conditions")
			if len(conditions) > 0 {
				output += "  Conditions:\n"
				for _, c := range conditions {
					if condMap, ok := c.(map[string]interface{}); ok {
						condType, _, _ := unstructured.NestedString(condMap, "type")
						condStatus, _, _ := unstructured.NestedString(condMap, "status")
						condReason, _, _ := unstructured.NestedString(condMap, "reason")
						condMessage, _, _ := unstructured.NestedString(condMap, "message")

						output += fmt.Sprintf("    - %s: %s", condType, condStatus)
						if condReason != "" {
							output += fmt.Sprintf(" (%s)", condReason)
						}
						output += "\n"
						if condMessage != "" && condStatus != "True" {
							output += fmt.Sprintf("      %s\n", condMessage)
						}
					}
				}
			}
		}

		output += "\n"
	}

	// Add summary
	summary := "Summary: "
	summaryParts := []string{}
	for state, count := range healthyCounts {
		summaryParts = append(summaryParts, fmt.Sprintf("%d %s", count, state))
	}
	sort.Strings(summaryParts)
	summary += strings.Join(summaryParts, ", ") + "\n\n"

	output = summary + output

	return api.NewToolCallResult(output, nil), nil
}

func olmCatalogSourcesStatus(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	namespace := params.GetString("namespace", "openshift-marketplace")

	gvk := schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "CatalogSource",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, namespace, api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list CatalogSources: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult(fmt.Sprintf("No CatalogSources found in namespace: %s", namespace), nil), nil
	}

	output := "OLM CatalogSources:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	for i := range list.Items {
		cs := &list.Items[i]
		name := cs.GetName()
		ns := cs.GetNamespace()

		// Get spec
		spec, _, _ := unstructured.NestedMap(cs.Object, "spec")
		sourceType := ""
		image := ""
		displayName := ""
		publisher := ""
		if spec != nil {
			sourceType, _, _ = unstructured.NestedString(spec, "sourceType")
			image, _, _ = unstructured.NestedString(spec, "image")
			displayName, _, _ = unstructured.NestedString(spec, "displayName")
			publisher, _, _ = unstructured.NestedString(spec, "publisher")
		}

		// Get status
		status, _, _ := unstructured.NestedMap(cs.Object, "status")
		connectionState := "Unknown"
		if status != nil {
			if connState, ok, _ := unstructured.NestedMap(status, "connectionState"); ok {
				connectionState, _, _ = unstructured.NestedString(connState, "lastObservedState")
			}
		}

		stateSymbol := "ℹ"
		if connectionState == "READY" {
			stateSymbol = "✓"
		} else if connectionState == "TRANSIENT_FAILURE" || connectionState == "SHUTDOWN" {
			stateSymbol = "✗"
		}

		output += fmt.Sprintf("%s CatalogSource: %s/%s\n", stateSymbol, ns, name)
		if displayName != "" {
			output += fmt.Sprintf("  Display Name: %s\n", displayName)
		}
		if publisher != "" {
			output += fmt.Sprintf("  Publisher: %s\n", publisher)
		}
		output += fmt.Sprintf("  Source Type: %s\n", sourceType)
		output += fmt.Sprintf("  Connection State: %s\n", connectionState)
		if image != "" {
			output += fmt.Sprintf("  Image: %s\n", image)
		}

		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func olmInstallPlansStatus(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	namespace := params.GetString("namespace", "")
	approvedFilter := params.GetString("approved", "all")

	gvk := schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "InstallPlan",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, namespace, api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list InstallPlans: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No OLM InstallPlans found", nil), nil
	}

	output := "OLM InstallPlans:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	approvedCount := 0
	pendingCount := 0

	for i := range list.Items {
		ip := &list.Items[i]
		name := ip.GetName()
		ns := ip.GetNamespace()

		// Get spec
		spec, _, _ := unstructured.NestedMap(ip.Object, "spec")
		approved := false
		clusterServiceVersionNames := []string{}
		if spec != nil {
			approved, _, _ = unstructured.NestedBool(spec, "approved")
			clusterServiceVersionNames, _, _ = unstructured.NestedStringSlice(spec, "clusterServiceVersionNames")
		}

		// Apply approved filter
		if approvedFilter == "true" && !approved {
			continue
		}
		if approvedFilter == "false" && approved {
			continue
		}

		if approved {
			approvedCount++
		} else {
			pendingCount++
		}

		// Get status
		status, _, _ := unstructured.NestedMap(ip.Object, "status")
		phase := "Unknown"
		if status != nil {
			phase, _, _ = unstructured.NestedString(status, "phase")
		}

		phaseSymbol := "ℹ"
		if phase == "Complete" {
			phaseSymbol = "✓"
		} else if phase == "Failed" {
			phaseSymbol = "✗"
		} else if !approved {
			phaseSymbol = "⏸"
		}

		approvalStatus := "Approved ✓"
		if !approved {
			approvalStatus = "Pending Approval ⏸"
		}

		output += fmt.Sprintf("%s InstallPlan: %s/%s\n", phaseSymbol, ns, name)
		output += fmt.Sprintf("  Phase: %s\n", phase)
		output += fmt.Sprintf("  Approval: %s\n", approvalStatus)

		if len(clusterServiceVersionNames) > 0 {
			output += fmt.Sprintf("  CSVs (%d):\n", len(clusterServiceVersionNames))
			for _, csv := range clusterServiceVersionNames {
				output += fmt.Sprintf("    - %s\n", csv)
			}
		}

		output += "\n"
	}

	// Add summary
	summary := fmt.Sprintf("Summary: %d approved, %d pending approval (total: %d)\n\n",
		approvedCount, pendingCount, len(list.Items))

	output = summary + output

	return api.NewToolCallResult(output, nil), nil
}
