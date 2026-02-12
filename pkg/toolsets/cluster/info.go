package cluster

import (
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func infoTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "cluster_info_get",
				Description: "Get OpenShift cluster infrastructure information including platform, region, topology, and network configuration",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: clusterInfoGet,
		},
	}
}

func clusterInfoGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	output := "OpenShift Cluster Information\n"
	output += strings.Repeat("=", 80) + "\n\n"

	// Get Infrastructure
	infraGVK := parseGVK("config.openshift.io/v1", "Infrastructure")
	infraOpts := api.ListOptions{}

	infraResources, err := params.MustGatherProvider.ListResources(params.Context, infraGVK, "", infraOpts)
	if err == nil && len(infraResources.Items) > 0 {
		infra := &infraResources.Items[0]

		output += "## Infrastructure\n\n"

		// Platform
		if platform, ok := getNestedString(infra, "status", "platform"); ok {
			output += fmt.Sprintf("Platform: %s\n", platform)
		}

		// Infrastructure name
		if infraName, ok := getNestedString(infra, "status", "infrastructureName"); ok {
			output += fmt.Sprintf("Infrastructure Name: %s\n", infraName)
		}

		// Control plane topology
		if topology, ok := getNestedString(infra, "status", "controlPlaneTopology"); ok {
			output += fmt.Sprintf("Control Plane Topology: %s\n", topology)
		}

		// Infrastructure topology
		if topology, ok := getNestedString(infra, "status", "infrastructureTopology"); ok {
			output += fmt.Sprintf("Infrastructure Topology: %s\n", topology)
		}

		// CPU partitioning
		if cpuPart, ok := getNestedString(infra, "status", "cpuPartitioning"); ok {
			output += fmt.Sprintf("CPU Partitioning: %s\n", cpuPart)
		}

		// Platform-specific details
		if platformStatus, found, _ := unstructured.NestedMap(infra.Object, "status", "platformStatus"); found {
			platformType, _ := platformStatus["type"].(string)
			output += fmt.Sprintf("\nPlatform Details (%s):\n", platformType)

			switch strings.ToLower(platformType) {
			case "aws":
				if aws, ok := platformStatus["aws"].(map[string]interface{}); ok {
					if region, ok := aws["region"].(string); ok {
						output += fmt.Sprintf("  Region: %s\n", region)
					}
					if serviceEndpoints, ok := aws["serviceEndpoints"].([]interface{}); ok && len(serviceEndpoints) > 0 {
						output += "  Service Endpoints:\n"
						for _, sep := range serviceEndpoints {
							if sepMap, ok := sep.(map[string]interface{}); ok {
								name, _ := sepMap["name"].(string)
								url, _ := sepMap["url"].(string)
								output += fmt.Sprintf("    - %s: %s\n", name, url)
							}
						}
					}
				}
			case "azure":
				if azure, ok := platformStatus["azure"].(map[string]interface{}); ok {
					if location, ok := azure["location"].(string); ok {
						output += fmt.Sprintf("  Location: %s\n", location)
					}
					if resourceGroup, ok := azure["resourceGroupName"].(string); ok {
						output += fmt.Sprintf("  Resource Group: %s\n", resourceGroup)
					}
				}
			case "gcp":
				if gcp, ok := platformStatus["gcp"].(map[string]interface{}); ok {
					if region, ok := gcp["region"].(string); ok {
						output += fmt.Sprintf("  Region: %s\n", region)
					}
					if projectID, ok := gcp["projectID"].(string); ok {
						output += fmt.Sprintf("  Project ID: %s\n", projectID)
					}
				}
			}
		}

		// API server URLs
		if apiURL, ok := getNestedString(infra, "status", "apiServerURL"); ok {
			output += fmt.Sprintf("\nAPI Server URL: %s\n", apiURL)
		}
		if apiInternalURL, ok := getNestedString(infra, "status", "apiServerInternalURI"); ok {
			output += fmt.Sprintf("API Server Internal URI: %s\n", apiInternalURL)
		}

		output += "\n"
	}

	// Get Network configuration
	networkGVK := parseGVK("config.openshift.io/v1", "Network")
	networkOpts := api.ListOptions{}

	networkResources, err := params.MustGatherProvider.ListResources(params.Context, networkGVK, "", networkOpts)
	if err == nil && len(networkResources.Items) > 0 {
		network := &networkResources.Items[0]

		output += "## Network Configuration\n\n"

		// Cluster network
		if clusterNetwork, found, _ := unstructured.NestedSlice(network.Object, "status", "clusterNetwork"); found && len(clusterNetwork) > 0 {
			output += "Cluster Networks:\n"
			for i, cn := range clusterNetwork {
				if cnMap, ok := cn.(map[string]interface{}); ok {
					cidr, _ := cnMap["cidr"].(string)
					hostPrefix, _ := cnMap["hostPrefix"].(float64)
					output += fmt.Sprintf("  %d. CIDR: %s, Host Prefix: %.0f\n", i+1, cidr, hostPrefix)
				}
			}
			output += "\n"
		}

		// Service network
		if serviceNetwork, found, _ := unstructured.NestedStringSlice(network.Object, "status", "serviceNetwork"); found && len(serviceNetwork) > 0 {
			output += "Service Networks:\n"
			for i, sn := range serviceNetwork {
				output += fmt.Sprintf("  %d. %s\n", i+1, sn)
			}
			output += "\n"
		}

		// Network type
		if networkType, ok := getNestedString(network, "status", "networkType"); ok {
			output += fmt.Sprintf("Network Type: %s\n", networkType)
		}

		// External IP policy
		if externalIP, found, _ := unstructured.NestedMap(network.Object, "spec", "externalIP"); found {
			if policy, ok := externalIP["policy"].(map[string]interface{}); ok {
				output += "\nExternal IP Policy:\n"
				if allowedCIDRs, ok := policy["allowedCIDRs"].([]interface{}); ok && len(allowedCIDRs) > 0 {
					output += "  Allowed CIDRs:\n"
					for _, cidr := range allowedCIDRs {
						output += fmt.Sprintf("    - %v\n", cidr)
					}
				}
				if rejectedCIDRs, ok := policy["rejectedCIDRs"].([]interface{}); ok && len(rejectedCIDRs) > 0 {
					output += "  Rejected CIDRs:\n"
					for _, cidr := range rejectedCIDRs {
						output += fmt.Sprintf("    - %v\n", cidr)
					}
				}
			}
		}
	}

	return api.NewToolCallResult(output, nil), nil
}
