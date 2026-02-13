package monitoring

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"gopkg.in/yaml.v3"
)

func configTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "monitoring_prometheus_config_summary",
				Description: "Get Prometheus configuration summary including scrape jobs, retention, and global settings",
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
			Handler: prometheusConfigSummary,
		},
		{
			Tool: api.Tool{
				Name:        "monitoring_servicemonitor_list",
				Description: "List ServiceMonitor custom resources that configure Prometheus scrape targets",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"namespace": {
							Type:        "string",
							Description: "Filter by namespace (partial match)",
						},
					},
				},
			},
			Handler: serviceMonitorList,
		},
	}
}

func prometheusConfigSummary(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	_ = params.GetString("replica", "prometheus-k8s-0")

	// Get container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	// Config is shared across replicas, so read from common prometheus directory
	promPath := getPrometheusCommonPath(containerDir)

	// Read config
	var configResp ConfigResponse
	configFile := filepath.Join(promPath, "status", "config.json")
	if err := readJSON(configFile, &configResp); err != nil {
		return api.NewToolCallResult("",
			fmt.Errorf("failed to read Prometheus config: %w", err)), nil
	}

	// Read flags for additional context
	var flags FlagsResponse
	flagsFile := filepath.Join(promPath, "status", "flags.json")
	readJSON(flagsFile, &flags)

	// Parse YAML config
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(configResp.YAML), &config); err != nil {
		return api.NewToolCallResult("",
			fmt.Errorf("failed to parse config YAML: %w", err)), nil
	}

	// Format output
	output := "Prometheus Configuration Summary\n"
	output += strings.Repeat("=", 80) + "\n\n"

	// Global settings
	if global, ok := config["global"].(map[string]interface{}); ok {
		output += "Global Settings:\n"

		if scrapeInterval, ok := global["scrape_interval"].(string); ok {
			output += fmt.Sprintf("  Scrape Interval: %s\n", scrapeInterval)
		}
		if scrapeTimeout, ok := global["scrape_timeout"].(string); ok {
			output += fmt.Sprintf("  Scrape Timeout: %s\n", scrapeTimeout)
		}
		if evalInterval, ok := global["evaluation_interval"].(string); ok {
			output += fmt.Sprintf("  Evaluation Interval: %s\n", evalInterval)
		}

		if externalLabels, ok := global["external_labels"].(map[string]interface{}); ok && len(externalLabels) > 0 {
			output += "  External Labels:\n"
			for k, v := range externalLabels {
				output += fmt.Sprintf("    %s: %v\n", k, v)
			}
		}

		output += "\n"
	}

	// Storage retention from flags
	if retention, ok := flags["storage.tsdb.retention.time"]; ok {
		output += fmt.Sprintf("Storage Retention: %s\n", retention)
	}
	if retentionSize, ok := flags["storage.tsdb.retention.size"]; ok {
		output += fmt.Sprintf("Storage Retention Size: %s\n", retentionSize)
	}
	output += "\n"

	// Scrape configs
	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		output += fmt.Sprintf("Scrape Jobs: %d\n\n", len(scrapeConfigs))

		// Collect job info
		type jobInfo struct {
			Name           string
			ScrapeInterval string
			ScrapeTimeout  string
			MetricsPath    string
		}

		var jobs []jobInfo

		for _, sc := range scrapeConfigs {
			if scrapeConfig, ok := sc.(map[string]interface{}); ok {
				job := jobInfo{}

				if name, ok := scrapeConfig["job_name"].(string); ok {
					job.Name = name
				}
				if interval, ok := scrapeConfig["scrape_interval"].(string); ok {
					job.ScrapeInterval = interval
				}
				if timeout, ok := scrapeConfig["scrape_timeout"].(string); ok {
					job.ScrapeTimeout = timeout
				}
				if path, ok := scrapeConfig["metrics_path"].(string); ok {
					job.MetricsPath = path
				}

				jobs = append(jobs, job)
			}
		}

		// Sort by name
		sort.Slice(jobs, func(i, j int) bool {
			return jobs[i].Name < jobs[j].Name
		})

		// Display job table
		output += fmt.Sprintf("%-50s %-12s %-12s\n", "Job Name", "Interval", "Timeout")
		output += strings.Repeat("-", 76) + "\n"

		for _, job := range jobs {
			interval := job.ScrapeInterval
			if interval == "" {
				interval = "(default)"
			}
			timeout := job.ScrapeTimeout
			if timeout == "" {
				timeout = "(default)"
			}

			output += fmt.Sprintf("%-50s %-12s %-12s\n",
				truncate(job.Name, 50), interval, timeout)
		}
		output += "\n"
	}

	// Rule files
	if ruleFiles, ok := config["rule_files"].([]interface{}); ok && len(ruleFiles) > 0 {
		output += fmt.Sprintf("Rule Files: %d\n", len(ruleFiles))
		for _, rf := range ruleFiles {
			if rfStr, ok := rf.(string); ok {
				output += fmt.Sprintf("  • %s\n", rfStr)
			}
		}
		output += "\n"
	}

	// Alerting config
	if alerting, ok := config["alerting"].(map[string]interface{}); ok {
		if amConfigs, ok := alerting["alertmanagers"].([]interface{}); ok && len(amConfigs) > 0 {
			output += fmt.Sprintf("AlertManager Configs: %d\n", len(amConfigs))

			for _, amc := range amConfigs {
				if amConfig, ok := amc.(map[string]interface{}); ok {
					if scheme, ok := amConfig["scheme"].(string); ok {
						output += fmt.Sprintf("  Scheme: %s\n", scheme)
					}
					if path, ok := amConfig["path_prefix"].(string); ok {
						output += fmt.Sprintf("  Path Prefix: %s\n", path)
					}
					if timeout, ok := amConfig["timeout"].(string); ok {
						output += fmt.Sprintf("  Timeout: %s\n", timeout)
					}
				}
			}
			output += "\n"
		}
	}

	return api.NewToolCallResult(output, nil), nil
}

func serviceMonitorList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	nsFilter := params.GetString("namespace", "")

	// ServiceMonitor is in monitoring.coreos.com/v1
	gvk := parseGVK("monitoring.coreos.com/v1", "ServiceMonitor")

	// Get all ServiceMonitor resources
	serviceMonitors, err := params.MustGatherProvider.ListResources(params.Context, gvk, "", api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("",
			fmt.Errorf("failed to list ServiceMonitor resources: %w", err)), nil
	}

	// Apply namespace filter and collect resources
	type monitorInfo struct {
		Name      string
		Namespace string
	}
	var filteredMonitors []monitorInfo

	for i := range serviceMonitors.Items {
		sm := &serviceMonitors.Items[i]
		ns := sm.GetNamespace()
		name := sm.GetName()

		if nsFilter != "" && !strings.Contains(strings.ToLower(ns), strings.ToLower(nsFilter)) {
			continue
		}

		filteredMonitors = append(filteredMonitors, monitorInfo{
			Name:      name,
			Namespace: ns,
		})
	}

	// Sort by namespace then name
	sort.Slice(filteredMonitors, func(i, j int) bool {
		if filteredMonitors[i].Namespace != filteredMonitors[j].Namespace {
			return filteredMonitors[i].Namespace < filteredMonitors[j].Namespace
		}
		return filteredMonitors[i].Name < filteredMonitors[j].Name
	})

	// Format output
	output := "ServiceMonitor Resources\n"
	output += strings.Repeat("=", 80) + "\n\n"

	output += fmt.Sprintf("Total: %d\n\n", len(filteredMonitors))

	if len(filteredMonitors) == 0 {
		output += "No ServiceMonitor resources found.\n"
		return api.NewToolCallResult(output, nil), nil
	}

	// Group by namespace
	byNamespace := make(map[string][]monitorInfo)
	for _, mon := range filteredMonitors {
		byNamespace[mon.Namespace] = append(byNamespace[mon.Namespace], mon)
	}

	// Get sorted namespaces
	namespaces := make([]string, 0, len(byNamespace))
	for ns := range byNamespace {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	// Display by namespace
	for _, ns := range namespaces {
		monitors := byNamespace[ns]
		output += fmt.Sprintf("Namespace: %s (%d)\n", ns, len(monitors))

		for _, mon := range monitors {
			output += fmt.Sprintf("  • %s\n", mon.Name)
		}
		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}
