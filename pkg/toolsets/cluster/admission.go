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

func admissionTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "admission_webhooks_list",
				Description: "List admission webhooks (validating and mutating) to identify webhook failures that block pod creation or updates",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"type": {
							Type:        "string",
							Description: "Filter by type: all, validating, mutating (default: all)",
							Enum:        []interface{}{"all", "validating", "mutating"},
						},
						"name": {
							Type:        "string",
							Description: "Filter by webhook name (partial match)",
						},
					},
				},
			},
			Handler: admissionWebhooksList,
		},
		{
			Tool: api.Tool{
				Name:        "admission_policies_list",
				Description: "List ValidatingAdmissionPolicies (CEL-based policies) and their bindings",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: admissionPoliciesList,
		},
	}
}

func admissionWebhooksList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	typeFilter := params.GetString("type", "all")
	nameFilter := params.GetString("name", "")

	output := "Admission Webhooks:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	// Get validating webhooks
	if typeFilter == "all" || typeFilter == "validating" {
		vGVK := schema.GroupVersionKind{
			Group:   "admissionregistration.k8s.io",
			Version: "v1",
			Kind:    "ValidatingWebhookConfiguration",
		}

		vList, err := params.MustGatherProvider.ListResources(context.Background(), vGVK, "", api.ListOptions{})
		if err == nil && len(vList.Items) > 0 {
			output += "## Validating Webhooks\n\n"

			for i := range vList.Items {
				config := &vList.Items[i]
				name := config.GetName()

				if nameFilter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(nameFilter)) {
					continue
				}

				output += fmt.Sprintf("ValidatingWebhookConfiguration: %s\n", name)

				// Get webhooks
				webhooks, _, _ := unstructured.NestedSlice(config.Object, "webhooks")
				if len(webhooks) > 0 {
					output += fmt.Sprintf("  Webhooks: %d\n", len(webhooks))
					for _, wh := range webhooks {
						if whMap, ok := wh.(map[string]interface{}); ok {
							whName, _, _ := unstructured.NestedString(whMap, "name")
							failurePolicy, _, _ := unstructured.NestedString(whMap, "failurePolicy")
							sideEffects, _, _ := unstructured.NestedString(whMap, "sideEffects")

							output += fmt.Sprintf("    - %s\n", whName)
							if failurePolicy != "" {
								output += fmt.Sprintf("      Failure Policy: %s\n", failurePolicy)
							}
							if sideEffects != "" {
								output += fmt.Sprintf("      Side Effects: %s\n", sideEffects)
							}

							// Get client config
							if clientConfig, ok, _ := unstructured.NestedMap(whMap, "clientConfig"); ok {
								if service, ok, _ := unstructured.NestedMap(clientConfig, "service"); ok {
									svcName, _, _ := unstructured.NestedString(service, "name")
									svcNamespace, _, _ := unstructured.NestedString(service, "namespace")
									output += fmt.Sprintf("      Service: %s/%s\n", svcNamespace, svcName)
								}
							}
						}
					}
				}
				output += "\n"
			}
		}
	}

	// Get mutating webhooks
	if typeFilter == "all" || typeFilter == "mutating" {
		mGVK := schema.GroupVersionKind{
			Group:   "admissionregistration.k8s.io",
			Version: "v1",
			Kind:    "MutatingWebhookConfiguration",
		}

		mList, err := params.MustGatherProvider.ListResources(context.Background(), mGVK, "", api.ListOptions{})
		if err == nil && len(mList.Items) > 0 {
			output += "## Mutating Webhooks\n\n"

			for i := range mList.Items {
				config := &mList.Items[i]
				name := config.GetName()

				if nameFilter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(nameFilter)) {
					continue
				}

				output += fmt.Sprintf("MutatingWebhookConfiguration: %s\n", name)

				// Get webhooks
				webhooks, _, _ := unstructured.NestedSlice(config.Object, "webhooks")
				if len(webhooks) > 0 {
					output += fmt.Sprintf("  Webhooks: %d\n", len(webhooks))
					for _, wh := range webhooks {
						if whMap, ok := wh.(map[string]interface{}); ok {
							whName, _, _ := unstructured.NestedString(whMap, "name")
							failurePolicy, _, _ := unstructured.NestedString(whMap, "failurePolicy")

							output += fmt.Sprintf("    - %s\n", whName)
							if failurePolicy != "" {
								output += fmt.Sprintf("      Failure Policy: %s\n", failurePolicy)
							}

							// Get client config
							if clientConfig, ok, _ := unstructured.NestedMap(whMap, "clientConfig"); ok {
								if service, ok, _ := unstructured.NestedMap(clientConfig, "service"); ok {
									svcName, _, _ := unstructured.NestedString(service, "name")
									svcNamespace, _, _ := unstructured.NestedString(service, "namespace")
									output += fmt.Sprintf("      Service: %s/%s\n", svcNamespace, svcName)
								}
							}
						}
					}
				}
				output += "\n"
			}
		}
	}

	return api.NewToolCallResult(output, nil), nil
}

func admissionPoliciesList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	gvk := schema.GroupVersionKind{
		Group:   "admissionregistration.k8s.io",
		Version: "v1",
		Kind:    "ValidatingAdmissionPolicy",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, "", api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list ValidatingAdmissionPolicies: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No ValidatingAdmissionPolicies found", nil), nil
	}

	// Sort by name
	items := list.Items
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetName() < items[j].GetName()
	})

	output := "ValidatingAdmissionPolicies:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	for i := range items {
		policy := &items[i]
		name := policy.GetName()

		output += fmt.Sprintf("Policy: %s\n", name)

		// Get spec
		spec, _, _ := unstructured.NestedMap(policy.Object, "spec")
		if spec != nil {
			// Failure policy
			if failurePolicy, ok, _ := unstructured.NestedString(spec, "failurePolicy"); ok {
				output += fmt.Sprintf("  Failure Policy: %s\n", failurePolicy)
			}

			// Validations
			if validations, ok, _ := unstructured.NestedSlice(spec, "validations"); ok {
				output += fmt.Sprintf("  Validations: %d\n", len(validations))
			}

			// Match constraints
			if matchConstraints, ok, _ := unstructured.NestedMap(spec, "matchConstraints"); ok {
				if resourceRules, ok, _ := unstructured.NestedSlice(matchConstraints, "resourceRules"); ok {
					output += fmt.Sprintf("  Resource Rules: %d\n", len(resourceRules))
				}
			}
		}

		output += "\n"
	}

	// Get bindings
	bindingGVK := schema.GroupVersionKind{
		Group:   "admissionregistration.k8s.io",
		Version: "v1",
		Kind:    "ValidatingAdmissionPolicyBinding",
	}

	bindingList, err := params.MustGatherProvider.ListResources(context.Background(), bindingGVK, "", api.ListOptions{})
	if err == nil && len(bindingList.Items) > 0 {
		output += "ValidatingAdmissionPolicyBindings:\n"
		output += strings.Repeat("-", 40) + "\n\n"

		for i := range bindingList.Items {
			binding := &bindingList.Items[i]
			name := binding.GetName()

			spec, _, _ := unstructured.NestedMap(binding.Object, "spec")
			policyName := ""
			if spec != nil {
				policyName, _, _ = unstructured.NestedString(spec, "policyName")
			}

			output += fmt.Sprintf("Binding: %s -> Policy: %s\n", name, policyName)
		}
	}

	return api.NewToolCallResult(output, nil), nil
}
