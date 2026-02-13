package monitoring

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
)

func alertTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "monitoring_alertmanager_status",
				Description: "Get AlertManager cluster status including peers, version, and uptime",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: alertManagerStatus,
		},
		{
			Tool: api.Tool{
				Name:        "monitoring_prometheus_rules",
				Description: "List Prometheus recording and alerting rules with grouping and health status",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"type": {
							Type:        "string",
							Description: "Filter by rule type: 'all', 'alerting', or 'recording'",
							Enum:        []interface{}{"all", "alerting", "recording"},
						},
						"group": {
							Type:        "string",
							Description: "Filter by rule group name (partial match)",
						},
						"health": {
							Type:        "string",
							Description: "Filter by health status: 'all', 'ok', 'err', 'unknown'",
							Enum:        []interface{}{"all", "ok", "err", "unknown"},
						},
					},
				},
			},
			Handler: prometheusRules,
		},
		{
			Tool: api.Tool{
				Name:        "monitoring_prometheus_alerts",
				Description: "List active Prometheus alerts with severity filtering and state breakdown",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"severity": {
							Type:        "string",
							Description: "Filter by severity level: 'all', 'critical', 'warning', 'info'",
							Enum:        []interface{}{"all", "critical", "warning", "info"},
						},
						"state": {
							Type:        "string",
							Description: "Filter by alert state: 'all', 'firing', 'pending'",
							Enum:        []interface{}{"all", "firing", "pending"},
						},
						"namespace": {
							Type:        "string",
							Description: "Filter by namespace (partial match)",
						},
					},
				},
			},
			Handler: prometheusAlerts,
		},
	}
}

func alertManagerStatus(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	// Get container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	// Read AlertManager status
	amPath := getAlertManagerPath(containerDir)
	statusFile := filepath.Join(amPath, "status.json")

	var status AlertManagerStatus
	if err := readJSON(statusFile, &status); err != nil {
		return api.NewToolCallResult("",
			fmt.Errorf("failed to read AlertManager status: %w", err)), nil
	}

	// Format output
	output := "AlertManager Status\n"
	output += strings.Repeat("=", 80) + "\n\n"

	// Cluster status
	clusterSymbol := healthSymbol(status.Cluster.Status)
	output += fmt.Sprintf("Cluster Status: %s %s\n", clusterSymbol, strings.ToUpper(status.Cluster.Status))
	output += fmt.Sprintf("Uptime: %s\n\n", status.Uptime)

	// Version info
	output += "Version Information:\n"
	output += fmt.Sprintf("  Version: %s\n", status.VersionInfo.Version)
	output += fmt.Sprintf("  Revision: %s\n", truncate(status.VersionInfo.Revision, 12))
	output += fmt.Sprintf("  Go Version: %s\n", status.VersionInfo.GoVersion)
	output += fmt.Sprintf("  Build Date: %s\n\n", status.VersionInfo.BuildDate)

	// Peers
	output += fmt.Sprintf("Cluster Peers (%d):\n", len(status.Cluster.Peers))
	if len(status.Cluster.Peers) == 0 {
		output += "  (no peers)\n"
	} else {
		for _, peer := range status.Cluster.Peers {
			output += fmt.Sprintf("  • %s - %s\n", peer.Name, peer.Address)
		}
	}

	return api.NewToolCallResult(output, nil), nil
}

func prometheusRules(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	typeFilter := params.GetString("type", "all")
	groupFilter := params.GetString("group", "")
	healthFilter := params.GetString("health", "all")

	// Get container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	// Read rules from common Prometheus directory
	promPath := getPrometheusCommonPath(containerDir)
	rulesFile := filepath.Join(promPath, "rules.json")

	var rulesAPIResp RuleGroupsAPIResponse
	if err := readJSON(rulesFile, &rulesAPIResp); err != nil {
		return api.NewToolCallResult("",
			fmt.Errorf("failed to read Prometheus rules: %w", err)), nil
	}
	rulesResp := rulesAPIResp.Data

	// Apply filters and collect stats
	var filteredGroups []RuleGroup
	totalRules := 0
	alertingRules := 0
	recordingRules := 0
	healthyRules := 0
	errorRules := 0

	for _, group := range rulesResp.Groups {
		// Filter by group name
		if groupFilter != "" && !strings.Contains(strings.ToLower(group.Name), strings.ToLower(groupFilter)) {
			continue
		}

		var filteredRules []Rule
		for _, rule := range group.Rules {
			// Filter by type
			if typeFilter != "all" && rule.Type != typeFilter {
				continue
			}

			// Filter by health
			if healthFilter != "all" {
				if healthFilter == "ok" && rule.Health != "ok" {
					continue
				}
				if healthFilter == "err" && rule.Health != "err" {
					continue
				}
				if healthFilter == "unknown" && rule.Health != "unknown" {
					continue
				}
			}

			filteredRules = append(filteredRules, rule)
			totalRules++

			if rule.Type == "alerting" {
				alertingRules++
			} else {
				recordingRules++
			}

			if rule.Health == "ok" {
				healthyRules++
			} else if rule.Health == "err" {
				errorRules++
			}
		}

		if len(filteredRules) > 0 {
			groupCopy := group
			groupCopy.Rules = filteredRules
			filteredGroups = append(filteredGroups, groupCopy)
		}
	}

	// Sort groups by name
	sort.Slice(filteredGroups, func(i, j int) bool {
		return filteredGroups[i].Name < filteredGroups[j].Name
	})

	// Format output
	output := "Prometheus Rules\n"
	output += strings.Repeat("=", 80) + "\n\n"

	output += fmt.Sprintf("Total Groups: %d\n", len(filteredGroups))
	output += fmt.Sprintf("Total Rules: %d (Alerting: %d, Recording: %d)\n",
		totalRules, alertingRules, recordingRules)
	output += fmt.Sprintf("Health: ✓ %d OK, ✗ %d Errors\n\n",
		healthyRules, errorRules)

	// List groups and rules
	for _, group := range filteredGroups {
		output += fmt.Sprintf("Group: %s\n", group.Name)
		output += fmt.Sprintf("  File: %s\n", truncate(group.File, 70))
		output += fmt.Sprintf("  Interval: %.0fs | Rules: %d\n",
			group.Interval, len(group.Rules))
		output += "\n"

		for _, rule := range group.Rules {
			healthSym := healthSymbol(rule.Health)
			ruleType := strings.ToUpper(rule.Type[:1])

			output += fmt.Sprintf("  %s [%s] %s\n", healthSym, ruleType, rule.Name)

			if rule.Type == "alerting" {
				severity := getSeverity(rule.Labels)
				sevSym := severitySymbol(severity)
				output += fmt.Sprintf("      Severity: %s %s", sevSym, severity)

				if len(rule.Alerts) > 0 {
					firingCount := 0
					for _, alert := range rule.Alerts {
						if alert.State == "firing" {
							firingCount++
						}
					}
					if firingCount > 0 {
						output += fmt.Sprintf(" | Firing: %d", firingCount)
					}
				}
				output += "\n"
			}

			if rule.Health != "ok" && rule.LastError != "" {
				output += fmt.Sprintf("      Error: %s\n", truncate(rule.LastError, 60))
			}
		}
		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func prometheusAlerts(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	severityFilter := params.GetString("severity", "all")
	stateFilter := params.GetString("state", "all")
	namespaceFilter := params.GetString("namespace", "")

	// Get container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	// Read rules to get active alerts
	promPath := getPrometheusCommonPath(containerDir)
	rulesFile := filepath.Join(promPath, "rules.json")

	var rulesAPIResp RuleGroupsAPIResponse
	if err := readJSON(rulesFile, &rulesAPIResp); err != nil {
		return api.NewToolCallResult("",
			fmt.Errorf("failed to read Prometheus rules: %w", err)), nil
	}
	rulesResp := rulesAPIResp.Data

	// Collect all alerts
	type AlertWithRule struct {
		Alert     Alert
		RuleName  string
		GroupName string
		Severity  string
	}

	var allAlerts []AlertWithRule
	severityCounts := make(map[string]int)
	stateCounts := make(map[string]int)

	for _, group := range rulesResp.Groups {
		for _, rule := range group.Rules {
			if rule.Type != "alerting" {
				continue
			}

			severity := getSeverity(rule.Labels)

			for _, alert := range rule.Alerts {
				// Apply filters
				if severityFilter != "all" && severity != severityFilter {
					continue
				}

				if stateFilter != "all" && alert.State != stateFilter {
					continue
				}

				ns := getNamespace(alert.Labels)
				if namespaceFilter != "" && !strings.Contains(strings.ToLower(ns), strings.ToLower(namespaceFilter)) {
					continue
				}

				allAlerts = append(allAlerts, AlertWithRule{
					Alert:     alert,
					RuleName:  rule.Name,
					GroupName: group.Name,
					Severity:  severity,
				})

				severityCounts[severity]++
				stateCounts[alert.State]++
			}
		}
	}

	// Sort alerts by severity (critical first) then by state
	sort.Slice(allAlerts, func(i, j int) bool {
		sevOrder := map[string]int{"critical": 0, "warning": 1, "info": 2, "unknown": 3}
		iOrder := sevOrder[allAlerts[i].Severity]
		jOrder := sevOrder[allAlerts[j].Severity]
		if iOrder != jOrder {
			return iOrder < jOrder
		}
		// Then by state (firing first)
		if allAlerts[i].Alert.State != allAlerts[j].Alert.State {
			return allAlerts[i].Alert.State == "firing"
		}
		return allAlerts[i].RuleName < allAlerts[j].RuleName
	})

	// Format output
	output := "Prometheus Active Alerts\n"
	output += strings.Repeat("=", 80) + "\n\n"

	output += fmt.Sprintf("Total Alerts: %d\n", len(allAlerts))
	output += "By Severity:\n"
	for _, sev := range []string{"critical", "warning", "info"} {
		if count := severityCounts[sev]; count > 0 {
			sym := severitySymbol(sev)
			output += fmt.Sprintf("  %s %s: %d\n", sym, strings.Title(sev), count)
		}
	}
	output += "By State:\n"
	for _, state := range []string{"firing", "pending"} {
		if count := stateCounts[state]; count > 0 {
			sym := statusSymbol(state)
			output += fmt.Sprintf("  %s %s: %d\n", sym, strings.Title(state), count)
		}
	}
	output += "\n"

	if len(allAlerts) == 0 {
		output += "No active alerts found.\n"
		return api.NewToolCallResult(output, nil), nil
	}

	// List alerts
	for _, item := range allAlerts {
		alert := item.Alert
		sevSym := severitySymbol(item.Severity)
		stateSym := statusSymbol(alert.State)

		output += fmt.Sprintf("%s %s [%s] %s\n",
			sevSym, stateSym, strings.ToUpper(item.Severity), item.RuleName)

		// Show key labels
		ns := getNamespace(alert.Labels)
		if ns != "" {
			output += fmt.Sprintf("    Namespace: %s\n", ns)
		}

		output += fmt.Sprintf("    State: %s | Active At: %s\n",
			alert.State, alert.ActiveAt)

		// Show summary annotation if present
		if summary, ok := alert.Annotations["summary"]; ok {
			output += fmt.Sprintf("    Summary: %s\n", truncate(summary, 70))
		} else if msg, ok := alert.Annotations["message"]; ok {
			output += fmt.Sprintf("    Message: %s\n", truncate(msg, 70))
		}

		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}
