package diagnostics

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
)

func etcdTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "etcd_health",
				Description: "Get ETCD cluster health status from must-gather including endpoint health and alarms",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: etcdHealth,
		},
		{
			Tool: api.Tool{
				Name:        "etcd_object_count",
				Description: "Get ETCD object counts by resource type, useful for identifying resource buildup",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"sortBy": {
							Type:        "string",
							Description: "Sort by 'count' (default) or 'name'",
							Enum:        []interface{}{"count", "name"},
						},
						"top": {
							Type:        "integer",
							Description: "Show only top N resource types (0 for all)",
						},
					},
				},
			},
			Handler: etcdObjectCount,
		},
	}
}

func etcdHealth(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	health, err := params.MustGatherProvider.GetETCDHealth()
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get ETCD health: %w", err)), nil
	}

	output := "ETCD Cluster Health\n"
	output += "==================\n\n"

	if health.Healthy {
		output += "Status: ✓ Healthy\n\n"
	} else {
		output += "Status: ✗ UNHEALTHY\n\n"
	}

	output += "Endpoints:\n"
	for _, endpoint := range health.Endpoints {
		status := "✓"
		if endpoint.Health != "healthy" {
			status = "✗"
		}
		output += fmt.Sprintf("  %s %s - %s\n", status, endpoint.Address, endpoint.Health)
	}

	if len(health.Alarms) > 0 {
		output += "\nAlarms:\n"
		for _, alarm := range health.Alarms {
			output += fmt.Sprintf("  ⚠ %s\n", alarm)
		}
	} else {
		output += "\nNo alarms detected\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func etcdObjectCount(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	counts, err := params.MustGatherProvider.GetETCDObjectCount()
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to get ETCD object count: %w", err)), nil
	}

	sortBy := params.GetString("sortBy", "count")
	top := params.GetInt("top", 0)

	// Convert to sorted slice
	type entry struct {
		Resource string
		Count    int64
	}
	entries := make([]entry, 0, len(counts))
	totalCount := int64(0)
	for resource, count := range counts {
		entries = append(entries, entry{Resource: resource, Count: count})
		totalCount += count
	}

	// Sort
	if sortBy == "count" {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Count > entries[j].Count
		})
	} else {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Resource < entries[j].Resource
		})
	}

	// Apply top limit
	if top > 0 && len(entries) > top {
		entries = entries[:top]
	}

	// Format output
	output := "ETCD Object Count\n"
	output += "=================\n\n"
	output += fmt.Sprintf("Total Objects: %d\n", totalCount)
	output += fmt.Sprintf("Resource Types: %d\n\n", len(counts))

	if top > 0 {
		output += fmt.Sprintf("Top %d Resource Types", top)
		if sortBy == "count" {
			output += " (by count)"
		}
		output += ":\n\n"
	}

	// Table header
	output += fmt.Sprintf("%-50s %10s\n", "Resource Type", "Count")
	output += fmt.Sprintf("%s %s\n", string(make([]byte, 50)), string(make([]byte, 10)))
	for i := range output[len(output)-62 : len(output)-1] {
		if output[len(output)-62+i] == ' ' {
			output = output[:len(output)-62+i] + "-" + output[len(output)-61+i:]
		}
	}

	// Table rows
	for _, e := range entries {
		output += fmt.Sprintf("%-50s %10d\n", e.Resource, e.Count)
	}

	// Show percentage if filtered
	if top > 0 && top < len(counts) {
		displayedCount := int64(0)
		for _, e := range entries {
			displayedCount += e.Count
		}
		percentage := float64(displayedCount) / float64(totalCount) * 100
		output += fmt.Sprintf("\nShowing %d of %d resource types (%.1f%% of objects)\n",
			top, len(counts), percentage)
	}

	return api.NewToolCallResult(output, nil), nil
}

// Helper function to pretty print JSON
func prettyJSON(data string) string {
	var obj interface{}
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return data
	}

	pretty, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return data
	}

	return string(pretty)
}
