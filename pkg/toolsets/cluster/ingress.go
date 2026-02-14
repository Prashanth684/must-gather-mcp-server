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

func ingressTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "routes_list",
				Description: "List OpenShift Routes showing hosts, services, TLS configuration, and admission status for debugging application routing",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"namespace": {
							Type:        "string",
							Description: "Filter by namespace (optional)",
						},
						"admitted": {
							Type:        "string",
							Description: "Filter by admission status: all, true, false (default: all)",
							Enum:        []interface{}{"all", "true", "false"},
						},
						"host": {
							Type:        "string",
							Description: "Filter by hostname (partial match, optional)",
						},
					},
				},
			},
			Handler: routesList,
		},
		{
			Tool: api.Tool{
				Name:        "route_get",
				Description: "Get detailed information about a specific Route including TLS termination, backend weights, and admission conditions",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"name": {
							Type:        "string",
							Description: "Route name",
						},
						"namespace": {
							Type:        "string",
							Description: "Route namespace",
						},
					},
					Required: []string{"name", "namespace"},
				},
			},
			Handler: routeGet,
		},
		{
			Tool: api.Tool{
				Name:        "ingress_list",
				Description: "List Kubernetes Ingress resources with rules and backend services",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"namespace": {
							Type:        "string",
							Description: "Filter by namespace (optional)",
						},
					},
				},
			},
			Handler: ingressList,
		},
		{
			Tool: api.Tool{
				Name:        "ingresscontroller_status",
				Description: "Get IngressController (OpenShift router) status including availability, domain, replicas, and conditions",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"name": {
							Type:        "string",
							Description: "IngressController name (default: 'default')",
						},
					},
				},
			},
			Handler: ingressControllerStatus,
		},
	}
}

func routesList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	namespace := params.GetString("namespace", "")
	admittedFilter := params.GetString("admitted", "all")
	hostFilter := params.GetString("host", "")

	gvk := schema.GroupVersionKind{
		Group:   "route.openshift.io",
		Version: "v1",
		Kind:    "Route",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, namespace, api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list Routes: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No Routes found", nil), nil
	}

	output := "OpenShift Routes:\n"
	output += strings.Repeat("=", 100) + "\n\n"

	admittedCount := 0
	notAdmittedCount := 0

	for i := range list.Items {
		route := &list.Items[i]
		name := route.GetName()
		ns := route.GetNamespace()

		// Get spec
		spec, _, _ := unstructured.NestedMap(route.Object, "spec")
		host := ""
		serviceName := ""
		tlsTermination := "None"
		if spec != nil {
			host, _, _ = unstructured.NestedString(spec, "host")
			if to, ok, _ := unstructured.NestedMap(spec, "to"); ok {
				serviceName, _, _ = unstructured.NestedString(to, "name")
			}
			if tls, ok, _ := unstructured.NestedMap(spec, "tls"); ok {
				tlsTermination, _, _ = unstructured.NestedString(tls, "termination")
				if tlsTermination == "" {
					tlsTermination = "edge"
				}
			}
		}

		// Apply host filter
		if hostFilter != "" && !strings.Contains(strings.ToLower(host), strings.ToLower(hostFilter)) {
			continue
		}

		// Get status
		status, _, _ := unstructured.NestedMap(route.Object, "status")
		isAdmitted := false
		if status != nil {
			ingress, _, _ := unstructured.NestedSlice(status, "ingress")
			if len(ingress) > 0 {
				if ingressMap, ok := ingress[0].(map[string]interface{}); ok {
					conditions, _, _ := unstructured.NestedSlice(ingressMap, "conditions")
					for _, c := range conditions {
						if condMap, ok := c.(map[string]interface{}); ok {
							condType, _, _ := unstructured.NestedString(condMap, "type")
							condStatus, _, _ := unstructured.NestedString(condMap, "status")
							if condType == "Admitted" && condStatus == "True" {
								isAdmitted = true
								break
							}
						}
					}
				}
			}
		}

		// Apply admitted filter
		if admittedFilter == "true" && !isAdmitted {
			continue
		}
		if admittedFilter == "false" && isAdmitted {
			continue
		}

		if isAdmitted {
			admittedCount++
		} else {
			notAdmittedCount++
		}

		symbol := "✓"
		if !isAdmitted {
			symbol = "✗"
		}

		output += fmt.Sprintf("%s Route: %s/%s\n", symbol, ns, name)
		output += fmt.Sprintf("  Host: %s\n", host)
		output += fmt.Sprintf("  Service: %s\n", serviceName)
		output += fmt.Sprintf("  TLS: %s\n", tlsTermination)
		if isAdmitted {
			output += fmt.Sprintf("  Status: Admitted ✓\n")
		} else {
			output += fmt.Sprintf("  Status: Not Admitted ✗\n")
		}
		output += "\n"
	}

	// Add summary
	summary := fmt.Sprintf("Summary: %d admitted, %d not admitted (total: %d)\n\n",
		admittedCount, notAdmittedCount, len(list.Items))
	output = summary + output

	return api.NewToolCallResult(output, nil), nil
}

func routeGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	name := params.GetString("name", "")
	namespace := params.GetString("namespace", "")

	if name == "" || namespace == "" {
		return api.NewToolCallResult("", fmt.Errorf("name and namespace are required")), nil
	}

	gvk := schema.GroupVersionKind{
		Group:   "route.openshift.io",
		Version: "v1",
		Kind:    "Route",
	}

	route, err := params.MustGatherProvider.GetResource(context.Background(), gvk, namespace, name)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get Route: %w", err)), nil
	}

	output := fmt.Sprintf("Route: %s/%s\n", namespace, name)
	output += strings.Repeat("=", 80) + "\n\n"

	// Get spec
	spec, _, _ := unstructured.NestedMap(route.Object, "spec")
	if spec != nil {
		host, _, _ := unstructured.NestedString(spec, "host")
		path, _, _ := unstructured.NestedString(spec, "path")
		wildcardPolicy, _, _ := unstructured.NestedString(spec, "wildcardPolicy")

		output += "## Routing\n\n"
		output += fmt.Sprintf("Host: %s\n", host)
		if path != "" {
			output += fmt.Sprintf("Path: %s\n", path)
		}
		if wildcardPolicy != "" && wildcardPolicy != "None" {
			output += fmt.Sprintf("Wildcard Policy: %s\n", wildcardPolicy)
		}

		// Backend
		output += "\n## Backend\n\n"
		if to, ok, _ := unstructured.NestedMap(spec, "to"); ok {
			kind, _, _ := unstructured.NestedString(to, "kind")
			serviceName, _, _ := unstructured.NestedString(to, "name")
			weight, _, _ := unstructured.NestedInt64(to, "weight")

			output += fmt.Sprintf("Primary Service: %s/%s\n", kind, serviceName)
			if weight > 0 {
				output += fmt.Sprintf("Weight: %d\n", weight)
			}
		}

		// Alternate backends
		if alternateBackends, ok, _ := unstructured.NestedSlice(spec, "alternateBackends"); ok && len(alternateBackends) > 0 {
			output += "\nAlternate Backends:\n"
			for _, ab := range alternateBackends {
				if abMap, ok := ab.(map[string]interface{}); ok {
					kind, _, _ := unstructured.NestedString(abMap, "kind")
					serviceName, _, _ := unstructured.NestedString(abMap, "name")
					weight, _, _ := unstructured.NestedInt64(abMap, "weight")
					output += fmt.Sprintf("  - %s/%s (weight: %d)\n", kind, serviceName, weight)
				}
			}
		}

		// Port
		if port, ok, _ := unstructured.NestedMap(spec, "port"); ok {
			targetPort, _, _ := unstructured.NestedString(port, "targetPort")
			output += fmt.Sprintf("\nTarget Port: %s\n", targetPort)
		}

		// TLS
		if tls, ok, _ := unstructured.NestedMap(spec, "tls"); ok {
			output += "\n## TLS Configuration\n\n"
			termination, _, _ := unstructured.NestedString(tls, "termination")
			insecurePolicy, _, _ := unstructured.NestedString(tls, "insecureEdgeTerminationPolicy")
			destinationCA, _, _ := unstructured.NestedString(tls, "destinationCACertificate")

			output += fmt.Sprintf("Termination: %s\n", termination)
			if insecurePolicy != "" {
				output += fmt.Sprintf("Insecure Edge Policy: %s\n", insecurePolicy)
			}
			if destinationCA != "" {
				output += "Destination CA Certificate: Present\n"
			}
		}
	}

	// Get status
	status, _, _ := unstructured.NestedMap(route.Object, "status")
	if status != nil {
		output += "\n## Status\n\n"

		ingress, _, _ := unstructured.NestedSlice(status, "ingress")
		if len(ingress) > 0 {
			output += fmt.Sprintf("Router Admissions: %d\n\n", len(ingress))

			for idx, ing := range ingress {
				if ingressMap, ok := ing.(map[string]interface{}); ok {
					routerName, _, _ := unstructured.NestedString(ingressMap, "routerName")
					routerHost, _, _ := unstructured.NestedString(ingressMap, "host")

					output += fmt.Sprintf("Router %d: %s\n", idx+1, routerName)
					output += fmt.Sprintf("  Host: %s\n", routerHost)

					// Conditions
					conditions, _, _ := unstructured.NestedSlice(ingressMap, "conditions")
					if len(conditions) > 0 {
						output += "  Conditions:\n"
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
			}
		} else {
			output += "No router admissions found\n"
		}
	}

	return api.NewToolCallResult(output, nil), nil
}

func ingressList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	namespace := params.GetString("namespace", "")

	gvk := schema.GroupVersionKind{
		Group:   "networking.k8s.io",
		Version: "v1",
		Kind:    "Ingress",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, namespace, api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list Ingress resources: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No Ingress resources found", nil), nil
	}

	// Sort by namespace and name
	items := list.Items
	sort.Slice(items, func(i, j int) bool {
		if items[i].GetNamespace() != items[j].GetNamespace() {
			return items[i].GetNamespace() < items[j].GetNamespace()
		}
		return items[i].GetName() < items[j].GetName()
	})

	output := "Kubernetes Ingress Resources:\n"
	output += strings.Repeat("=", 80) + "\n\n"

	for i := range items {
		ingress := &items[i]
		name := ingress.GetName()
		ns := ingress.GetNamespace()

		output += fmt.Sprintf("Ingress: %s/%s\n", ns, name)

		// Get spec
		spec, _, _ := unstructured.NestedMap(ingress.Object, "spec")
		if spec != nil {
			// Ingress class
			if ingressClassName, ok, _ := unstructured.NestedString(spec, "ingressClassName"); ok {
				output += fmt.Sprintf("  Class: %s\n", ingressClassName)
			}

			// Default backend
			if defaultBackend, ok, _ := unstructured.NestedMap(spec, "defaultBackend"); ok {
				if service, ok, _ := unstructured.NestedMap(defaultBackend, "service"); ok {
					serviceName, _, _ := unstructured.NestedString(service, "name")
					output += fmt.Sprintf("  Default Backend: %s\n", serviceName)
				}
			}

			// Rules
			rules, _, _ := unstructured.NestedSlice(spec, "rules")
			if len(rules) > 0 {
				output += fmt.Sprintf("  Rules: %d\n", len(rules))
				for _, rule := range rules {
					if ruleMap, ok := rule.(map[string]interface{}); ok {
						host, _, _ := unstructured.NestedString(ruleMap, "host")
						if host != "" {
							output += fmt.Sprintf("    - Host: %s\n", host)
						}
					}
				}
			}

			// TLS
			tls, _, _ := unstructured.NestedSlice(spec, "tls")
			if len(tls) > 0 {
				output += fmt.Sprintf("  TLS: %d configuration(s)\n", len(tls))
			}
		}

		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func ingressControllerStatus(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	name := params.GetString("name", "default")

	gvk := schema.GroupVersionKind{
		Group:   "operator.openshift.io",
		Version: "v1",
		Kind:    "IngressController",
	}

	ic, err := params.MustGatherProvider.GetResource(context.Background(), gvk, "openshift-ingress-operator", name)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get IngressController: %w", err)), nil
	}

	output := fmt.Sprintf("IngressController: %s\n", name)
	output += strings.Repeat("=", 80) + "\n\n"

	// Get spec
	spec, _, _ := unstructured.NestedMap(ic.Object, "spec")
	if spec != nil {
		output += "## Configuration\n\n"

		domain, _, _ := unstructured.NestedString(spec, "domain")
		replicas, _, _ := unstructured.NestedInt64(spec, "replicas")

		output += fmt.Sprintf("Domain: %s\n", domain)
		if replicas > 0 {
			output += fmt.Sprintf("Replicas: %d\n", replicas)
		}

		// Endpoint publishing strategy
		if endpointPublishingStrategy, ok, _ := unstructured.NestedMap(spec, "endpointPublishingStrategy"); ok {
			strategyType, _, _ := unstructured.NestedString(endpointPublishingStrategy, "type")
			output += fmt.Sprintf("Endpoint Publishing: %s\n", strategyType)
		}

		// Route selector
		if routeSelector, ok, _ := unstructured.NestedMap(spec, "routeSelector"); ok {
			if matchLabels, ok, _ := unstructured.NestedMap(routeSelector, "matchLabels"); ok {
				output += "Route Selector:\n"
				for k, v := range matchLabels {
					output += fmt.Sprintf("  %s: %v\n", k, v)
				}
			}
		}
	}

	// Get status
	status, _, _ := unstructured.NestedMap(ic.Object, "status")
	if status != nil {
		output += "\n## Status\n\n"

		availableReplicas, _, _ := unstructured.NestedInt64(status, "availableReplicas")
		selector, _, _ := unstructured.NestedString(status, "selector")
		domain, _, _ := unstructured.NestedString(status, "domain")

		output += fmt.Sprintf("Available Replicas: %d\n", availableReplicas)
		if selector != "" {
			output += fmt.Sprintf("Selector: %s\n", selector)
		}
		if domain != "" {
			output += fmt.Sprintf("Domain: %s\n", domain)
		}

		// Conditions
		conditions, _, _ := unstructured.NestedSlice(status, "conditions")
		if len(conditions) > 0 {
			output += "\nConditions:\n"
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

					output += fmt.Sprintf("  %s %s: %s", symbol, condType, condStatus)
					if condReason != "" {
						output += fmt.Sprintf(" (%s)", condReason)
					}
					output += "\n"
					if condMessage != "" && condStatus != "True" {
						output += fmt.Sprintf("     %s\n", condMessage)
					}
				}
			}
		}
	}

	return api.NewToolCallResult(output, nil), nil
}
