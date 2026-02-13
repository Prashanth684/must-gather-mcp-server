package monitoring

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
)

func prometheusTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "monitoring_prometheus_status",
				Description: "Get Prometheus server status including TSDB statistics, runtime information, and health",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"replica": {
							Type:        "string",
							Description: "Prometheus replica to query: 'prometheus-k8s-0', 'prometheus-k8s-1', or 'both'",
							Enum:        []interface{}{"prometheus-k8s-0", "prometheus-k8s-1", "both", "0", "1"},
						},
					},
				},
			},
			Handler: prometheusStatus,
		},
		{
			Tool: api.Tool{
				Name:        "monitoring_prometheus_targets",
				Description: "List Prometheus scrape targets with health status, job, and namespace filtering",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"replica": {
							Type:        "string",
							Description: "Prometheus replica to query: 'prometheus-k8s-0', 'prometheus-k8s-1', or 'both'",
							Enum:        []interface{}{"prometheus-k8s-0", "prometheus-k8s-1", "both", "0", "1"},
						},
						"health": {
							Type:        "string",
							Description: "Filter by health status: 'all', 'up', 'down', 'unknown'",
							Enum:        []interface{}{"all", "up", "down", "unknown"},
						},
						"job": {
							Type:        "string",
							Description: "Filter by job name (partial match)",
						},
						"namespace": {
							Type:        "string",
							Description: "Filter by namespace (partial match)",
						},
						"limit": {
							Type:        "integer",
							Description: "Maximum number of targets to show (0 for all)",
						},
					},
				},
			},
			Handler: prometheusTargets,
		},
		{
			Tool: api.Tool{
				Name:        "monitoring_prometheus_tsdb",
				Description: "Get detailed Prometheus TSDB statistics including top metrics by series count and label cardinality",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"replica": {
							Type:        "string",
							Description: "Prometheus replica to query: 'prometheus-k8s-0', 'prometheus-k8s-1', or 'both'",
							Enum:        []interface{}{"prometheus-k8s-0", "prometheus-k8s-1", "both", "0", "1"},
						},
						"top": {
							Type:        "integer",
							Description: "Number of top metrics/labels to show (default: 10)",
						},
					},
				},
			},
			Handler: prometheusTSDB,
		},
	}
}

func prometheusStatus(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	replica := params.GetString("replica", "both")

	// Get container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	// Format output
	output := "Prometheus Server Status\n"
	output += strings.Repeat("=", 80) + "\n\n"

	replicaNums := getReplicaNumbers(replica)

	for _, num := range replicaNums {
		replicaPath := getPrometheusReplicaPath(containerDir, num)

		// Read TSDB status
		var tsdbResp TSDBStatusResponse
		if err := readPrometheusJSON(replicaPath, "status/tsdb.json", &tsdbResp); err != nil {
			output += fmt.Sprintf("⚠ prometheus-k8s-%d: Failed to read TSDB status - %v\n\n", num, err)
			continue
		}
		tsdb := tsdbResp.Data

		// Read runtime info
		var runtimeResp RuntimeInfoResponse
		runtimeErr := readPrometheusJSON(replicaPath, "status/runtimeinfo.json", &runtimeResp)
		runtime := runtimeResp.Data

		output += fmt.Sprintf("Replica: prometheus-k8s-%d\n", num)
		output += strings.Repeat("-", 80) + "\n"

		if runtimeErr == nil {
			// Status
			if runtime.ReloadConfigSuccess {
				output += "Status: ✓ Config Loaded Successfully\n"
			} else {
				output += "Status: ✗ Config Reload Failed\n"
			}

			output += fmt.Sprintf("Start Time: %s\n", runtime.StartTime)
			output += fmt.Sprintf("Last Config: %s\n", runtime.LastConfigTime)
			output += fmt.Sprintf("Storage Retention: %s\n", runtime.StorageRetention)
			output += fmt.Sprintf("Goroutines: %s\n", formatNumber(runtime.GoroutineCount))
			output += fmt.Sprintf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS)
			output += fmt.Sprintf("Memory Limit: %s\n", formatBytes(runtime.GOMEMLIMIT))

			if runtime.CorruptionCount > 0 {
				output += fmt.Sprintf("⚠ Corruptions: %d\n", runtime.CorruptionCount)
			}
			output += "\n"
		}

		// TSDB Stats
		output += "TSDB Statistics:\n"
		output += fmt.Sprintf("  Total Series: %s\n", formatNumber(tsdb.HeadStats.NumSeries))
		output += fmt.Sprintf("  Label Pairs: %s\n", formatNumber(tsdb.HeadStats.NumLabelPairs))
		output += fmt.Sprintf("  Chunks: %s\n", formatNumber(tsdb.HeadStats.ChunkCount))

		if len(tsdb.SeriesCountByMetricName) > 0 {
			output += fmt.Sprintf("  Unique Metrics: %d\n", len(tsdb.SeriesCountByMetricName))
		}
		if len(tsdb.LabelValueCountByLabelName) > 0 {
			output += fmt.Sprintf("  Unique Labels: %d\n", len(tsdb.LabelValueCountByLabelName))
		}

		// Calculate total memory
		totalMem := int64(0)
		for _, mem := range tsdb.MemoryInBytesByLabelName {
			totalMem += mem.Value
		}
		if totalMem > 0 {
			output += fmt.Sprintf("  Label Memory: %s\n", formatBytes(totalMem))
		}

		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func prometheusTargets(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	replica := params.GetString("replica", "both")
	healthFilter := params.GetString("health", "all")
	jobFilter := params.GetString("job", "")
	nsFilter := params.GetString("namespace", "")
	limit := params.GetInt("limit", 0)

	// Get container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	output := "Prometheus Scrape Targets\n"
	output += strings.Repeat("=", 80) + "\n\n"

	replicaNums := getReplicaNumbers(replica)

	for _, num := range replicaNums {
		replicaPath := getPrometheusReplicaPath(containerDir, num)

		// Read active targets
		var targetsAPIResp ActiveTargetsAPIResponse
		if err := readPrometheusJSON(replicaPath, "active-targets.json", &targetsAPIResp); err != nil {
			output += fmt.Sprintf("⚠ prometheus-k8s-%d: Failed to read targets - %v\n\n", num, err)
			continue
		}
		targetsResp := targetsAPIResp.Data

		// Apply filters
		var filteredTargets []ActiveTarget
		healthCounts := make(map[string]int)

		for _, target := range targetsResp.ActiveTargets {
			// Count all for stats
			healthCounts[target.Health]++

			// Apply health filter
			if healthFilter != "all" && target.Health != healthFilter {
				continue
			}

			// Apply job filter
			job := getJob(target.Labels)
			if jobFilter != "" && !strings.Contains(strings.ToLower(job), strings.ToLower(jobFilter)) {
				continue
			}

			// Apply namespace filter
			ns := getNamespace(target.Labels)
			if nsFilter != "" && !strings.Contains(strings.ToLower(ns), strings.ToLower(nsFilter)) {
				continue
			}

			filteredTargets = append(filteredTargets, target)
		}

		// Sort by health (down first) then by job
		sort.Slice(filteredTargets, func(i, j int) bool {
			if filteredTargets[i].Health != filteredTargets[j].Health {
				// down first, then unknown, then up
				if filteredTargets[i].Health == "down" {
					return true
				}
				if filteredTargets[j].Health == "down" {
					return false
				}
				if filteredTargets[i].Health == "unknown" {
					return true
				}
				if filteredTargets[j].Health == "unknown" {
					return false
				}
			}
			return getJob(filteredTargets[i].Labels) < getJob(filteredTargets[j].Labels)
		})

		// Apply limit
		displayCount := len(filteredTargets)
		if limit > 0 && len(filteredTargets) > limit {
			filteredTargets = filteredTargets[:limit]
		}

		output += fmt.Sprintf("Replica: prometheus-k8s-%d\n", num)
		output += strings.Repeat("-", 80) + "\n"

		output += fmt.Sprintf("Total Targets: %d\n", len(targetsResp.ActiveTargets))
		output += "Health Summary:\n"
		for _, health := range []string{"up", "down", "unknown"} {
			if count := healthCounts[health]; count > 0 {
				sym := healthSymbol(health)
				output += fmt.Sprintf("  %s %s: %d\n", sym, strings.ToUpper(health), count)
			}
		}

		if healthFilter != "all" || jobFilter != "" || nsFilter != "" {
			output += fmt.Sprintf("\nFiltered Results: %d targets", displayCount)
			if limit > 0 && displayCount > limit {
				output += fmt.Sprintf(" (showing first %d)", limit)
			}
			output += "\n"
		}
		output += "\n"

		// List targets
		for _, target := range filteredTargets {
			sym := healthSymbol(target.Health)
			job := getJob(target.Labels)
			ns := getNamespace(target.Labels)

			output += fmt.Sprintf("%s [%s] %s\n", sym, strings.ToUpper(target.Health), job)

			if ns != "" {
				output += fmt.Sprintf("    Namespace: %s\n", ns)
			}

			output += fmt.Sprintf("    URL: %s\n", truncate(target.ScrapeURL, 70))

			if target.Health != "up" && target.LastError != "" {
				output += fmt.Sprintf("    Error: %s\n", truncate(target.LastError, 65))
			}

			if target.LastScrape != "" {
				output += fmt.Sprintf("    Last Scrape: %s (duration: %s)\n",
					target.LastScrape, formatDuration(target.LastScrapeDuration))
			}

			output += "\n"
		}

		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func prometheusTSDB(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	replica := params.GetString("replica", "both")
	top := params.GetInt("top", 10)

	// Get container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	output := "Prometheus TSDB Details\n"
	output += strings.Repeat("=", 80) + "\n\n"

	replicaNums := getReplicaNumbers(replica)

	for _, num := range replicaNums {
		replicaPath := getPrometheusReplicaPath(containerDir, num)

		// Read TSDB status
		var tsdbResp TSDBStatusResponse
		if err := readPrometheusJSON(replicaPath, "status/tsdb.json", &tsdbResp); err != nil {
			output += fmt.Sprintf("⚠ prometheus-k8s-%d: Failed to read TSDB status - %v\n\n", num, err)
			continue
		}
		tsdb := tsdbResp.Data

		output += fmt.Sprintf("Replica: prometheus-k8s-%d\n", num)
		output += strings.Repeat("-", 80) + "\n\n"

		// Head stats
		output += "Head Block Statistics:\n"
		output += fmt.Sprintf("  Series: %s\n", formatNumber(tsdb.HeadStats.NumSeries))
		output += fmt.Sprintf("  Label Pairs: %s\n", formatNumber(tsdb.HeadStats.NumLabelPairs))
		output += fmt.Sprintf("  Chunks: %s\n", formatNumber(tsdb.HeadStats.ChunkCount))
		output += "\n"

		// Top metrics by series count
		if len(tsdb.SeriesCountByMetricName) > 0 {
			displayTop := top
			if displayTop > len(tsdb.SeriesCountByMetricName) {
				displayTop = len(tsdb.SeriesCountByMetricName)
			}

			output += fmt.Sprintf("Top %d Metrics by Series Count:\n", displayTop)
			output += fmt.Sprintf("%-60s %12s\n", "Metric Name", "Series")
			output += strings.Repeat("-", 74) + "\n"

			for i := 0; i < displayTop; i++ {
				metric := tsdb.SeriesCountByMetricName[i]
				output += fmt.Sprintf("%-60s %12s\n",
					truncate(metric.Name, 60), formatNumber(metric.Value))
			}
			output += "\n"
		}

		// Top labels by cardinality
		if len(tsdb.LabelValueCountByLabelName) > 0 {
			displayTop := top
			if displayTop > len(tsdb.LabelValueCountByLabelName) {
				displayTop = len(tsdb.LabelValueCountByLabelName)
			}

			output += fmt.Sprintf("Top %d Labels by Cardinality:\n", displayTop)
			output += fmt.Sprintf("%-60s %12s\n", "Label Name", "Values")
			output += strings.Repeat("-", 74) + "\n"

			for i := 0; i < displayTop; i++ {
				label := tsdb.LabelValueCountByLabelName[i]
				output += fmt.Sprintf("%-60s %12s\n",
					truncate(label.Name, 60), formatNumber(label.Value))
			}
			output += "\n"
		}

		// Top labels by memory usage
		if len(tsdb.MemoryInBytesByLabelName) > 0 {
			displayTop := top
			if displayTop > len(tsdb.MemoryInBytesByLabelName) {
				displayTop = len(tsdb.MemoryInBytesByLabelName)
			}

			output += fmt.Sprintf("Top %d Labels by Memory Usage:\n", displayTop)
			output += fmt.Sprintf("%-60s %12s\n", "Label Name", "Memory")
			output += strings.Repeat("-", 74) + "\n"

			for i := 0; i < displayTop; i++ {
				label := tsdb.MemoryInBytesByLabelName[i]
				output += fmt.Sprintf("%-60s %12s\n",
					truncate(label.Name, 60), formatBytes(label.Value))
			}
			output += "\n"
		}

		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}
