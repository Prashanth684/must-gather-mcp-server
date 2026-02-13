package monitoring

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// findContainerDir finds the must-gather container directory
// Must-gather structure can be: must-gather-root/quay-io-...-sha256-.../
func findContainerDir(mustGatherPath string) (string, error) {
	entries, err := os.ReadDir(mustGatherPath)
	if err != nil {
		return "", err
	}

	// Look for directory starting with "quay" or containing "sha256"
	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, "quay") || strings.Contains(name, "sha256") {
				return filepath.Join(mustGatherPath, name), nil
			}
		}
	}

	return "", fmt.Errorf("container directory not found")
}

// getPrometheusReplicaPath builds path to Prometheus replica data
func getPrometheusReplicaPath(containerDir string, replicaNum int) string {
	return filepath.Join(containerDir, "monitoring", "prometheus",
		fmt.Sprintf("prometheus-k8s-%d", replicaNum))
}

// getPrometheusCommonPath builds path to common Prometheus data
func getPrometheusCommonPath(containerDir string) string {
	return filepath.Join(containerDir, "monitoring", "prometheus")
}

// getAlertManagerPath builds path to AlertManager data
func getAlertManagerPath(containerDir string) string {
	return filepath.Join(containerDir, "monitoring", "alertmanager")
}

// readJSON reads and unmarshals a JSON file
func readJSON(filePath string, v interface{}) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return nil
}

// readPrometheusJSON reads JSON from a Prometheus replica directory
func readPrometheusJSON(replicaPath, filename string, v interface{}) error {
	dataFile := filepath.Join(replicaPath, filename)
	return readJSON(dataFile, v)
}

// getReplicaNumbers converts replica parameter to numbers
func getReplicaNumbers(replicaParam string) []int {
	switch replicaParam {
	case "prometheus-k8s-0", "0":
		return []int{0}
	case "prometheus-k8s-1", "1":
		return []int{1}
	default: // "both"
		return []int{0, 1}
	}
}

// formatBytes formats bytes as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatNumber formats a number with thousands separators
func formatNumber(n int64) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// formatDuration formats duration in seconds to human-readable string
func formatDuration(seconds float64) string {
	if seconds < 0.001 {
		return fmt.Sprintf("%.2fμs", seconds*1000000)
	} else if seconds < 1 {
		return fmt.Sprintf("%.2fms", seconds*1000)
	} else if seconds < 60 {
		return fmt.Sprintf("%.2fs", seconds)
	} else if seconds < 3600 {
		return fmt.Sprintf("%.1fm", seconds/60)
	} else if seconds < 86400 {
		return fmt.Sprintf("%.1fh", seconds/3600)
	}
	return fmt.Sprintf("%.1fd", seconds/86400)
}

// healthSymbol returns a symbol for health status
func healthSymbol(health string) string {
	switch strings.ToLower(health) {
	case "up", "healthy", "ok", "firing":
		return "✓"
	case "down", "unhealthy", "error":
		return "✗"
	default:
		return "⚠"
	}
}

// severitySymbol returns a symbol for severity level
func severitySymbol(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "✗"
	case "warning":
		return "⚠"
	case "info":
		return "ℹ"
	default:
		return "•"
	}
}

// statusSymbol returns a symbol for rule/alert state
func statusSymbol(state string) string {
	switch strings.ToLower(state) {
	case "firing":
		return "✗"
	case "pending":
		return "⚠"
	case "inactive":
		return "○"
	default:
		return "•"
	}
}

// truncate truncates a string to maxLen, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// getSeverity extracts severity from labels
func getSeverity(labels map[string]string) string {
	if sev, ok := labels["severity"]; ok {
		return sev
	}
	return "unknown"
}

// getNamespace extracts namespace from labels
func getNamespace(labels map[string]string) string {
	if ns, ok := labels["namespace"]; ok {
		return ns
	}
	return ""
}

// getJob extracts job from labels
func getJob(labels map[string]string) string {
	if job, ok := labels["job"]; ok {
		return job
	}
	return ""
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseGVK parses apiVersion and kind into GroupVersionKind
func parseGVK(apiVersion, kind string) schema.GroupVersionKind {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		// Fallback for simple case
		return schema.GroupVersionKind{
			Group:   "",
			Version: apiVersion,
			Kind:    kind,
		}
	}
	return schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    kind,
	}
}
