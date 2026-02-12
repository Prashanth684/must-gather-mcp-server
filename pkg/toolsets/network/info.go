package network

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
)

func networkInfoTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "network_scale_get",
				Description: "Get network scale information including count of services, endpoints, pods, and network policies",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: networkScaleGet,
		},
		{
			Tool: api.Tool{
				Name:        "network_ovn_resources",
				Description: "Get OVN Kubernetes pod resource usage (CPU and memory for OVN components)",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: networkOVNResources,
		},
	}
}

func networkScaleGet(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	// Find container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	scaleFile := filepath.Join(containerDir, "network_logs", "cluster_scale")

	// Check if file exists
	if _, err := os.Stat(scaleFile); os.IsNotExist(err) {
		return api.NewToolCallResult("", fmt.Errorf("network scale data not found")), nil
	}

	// Read the file
	data, err := os.ReadFile(scaleFile)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to read network scale data: %w", err)), nil
	}

	output := "Network Scale Information\n"
	output += strings.Repeat("=", 80) + "\n\n"

	// Parse the scale data
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		output += line + "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func networkOVNResources(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	// Find container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	ovnFile := filepath.Join(containerDir, "network_logs", "ovn_kubernetes_top_pods")

	// Check if file exists
	if _, err := os.Stat(ovnFile); os.IsNotExist(err) {
		return api.NewToolCallResult("", fmt.Errorf("OVN resource data not found")), nil
	}

	// Read the file
	file, err := os.Open(ovnFile)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to read OVN resource data: %w", err)), nil
	}
	defer file.Close()

	output := "OVN Kubernetes Resource Usage\n"
	output += strings.Repeat("=", 80) + "\n\n"

	scanner := bufio.NewScanner(file)

	// Track totals
	type podResources struct {
		pod        string
		containers map[string]struct {
			cpu    string
			memory string
		}
		totalCPU    int64
		totalMemory int64
	}

	pods := make(map[string]*podResources)
	var headerRead bool

	for scanner.Scan() {
		line := scanner.Text()

		if !headerRead {
			output += line + "\n"
			output += strings.Repeat("-", 80) + "\n"
			headerRead = true
			continue
		}

		// Parse line: POD NAME CPU(cores) MEMORY(bytes)
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		podName := fields[0]
		containerName := fields[1]
		cpu := fields[2]
		memory := fields[3]

		if _, exists := pods[podName]; !exists {
			pods[podName] = &podResources{
				pod:        podName,
				containers: make(map[string]struct{ cpu, memory string }),
			}
		}

		pods[podName].containers[containerName] = struct{ cpu, memory string }{
			cpu:    cpu,
			memory: memory,
		}

		// Parse CPU (remove 'm' for millicores)
		cpuVal := strings.TrimSuffix(cpu, "m")
		if cpuNum, err := strconv.ParseInt(cpuVal, 10, 64); err == nil {
			pods[podName].totalCPU += cpuNum
		}

		// Parse memory (remove 'Mi')
		memVal := strings.TrimSuffix(memory, "Mi")
		if memNum, err := strconv.ParseInt(memVal, 10, 64); err == nil {
			pods[podName].totalMemory += memNum
		}

		output += line + "\n"
	}

	if err := scanner.Err(); err != nil {
		return api.NewToolCallResult("", fmt.Errorf("error reading OVN resource data: %w", err)), nil
	}

	// Add summary
	output += "\n" + strings.Repeat("=", 80) + "\n"
	output += "Summary by Pod\n"
	output += strings.Repeat("=", 80) + "\n\n"
	output += fmt.Sprintf("%-45s %10s %15s %12s\n", "POD", "CONTAINERS", "TOTAL CPU", "TOTAL MEMORY")
	output += strings.Repeat("-", 80) + "\n"

	for podName, res := range pods {
		output += fmt.Sprintf("%-45s %10d %12dm %11dMi\n",
			truncatePodName(podName, 45),
			len(res.containers),
			res.totalCPU,
			res.totalMemory)
	}

	return api.NewToolCallResult(output, nil), nil
}

// Helper functions

func findContainerDir(basePath string) (string, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, "quay") || strings.Contains(name, "sha256") {
				return filepath.Join(basePath, name), nil
			}
		}
	}

	return "", fmt.Errorf("container directory not found")
}

func truncatePodName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen-3] + "..."
}
