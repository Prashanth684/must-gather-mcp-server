package mustgather

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// LoadResult contains the result of loading a must-gather
type LoadResult struct {
	Resources  []*unstructured.Unstructured
	Namespaces []string
	Metadata   *LoadMetadata
}

// LoadMetadata contains metadata extracted during loading
type LoadMetadata struct {
	Path           string
	Version        string
	StartTime      time.Time
	EndTime        time.Time
	ResourceCount  int
	NamespaceCount int
}

// Load loads a must-gather from the specified path
func Load(mustGatherPath string) (*LoadResult, error) {
	// Verify path exists
	if _, err := os.Stat(mustGatherPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("must-gather path does not exist: %s", mustGatherPath)
	}

	result := &LoadResult{
		Resources: make([]*unstructured.Unstructured, 0),
		Metadata: &LoadMetadata{
			Path: mustGatherPath,
		},
	}

	// Find the actual must-gather container directory
	containerDir, err := findContainerDir(mustGatherPath)
	if err != nil {
		// If no container dir found, use the path as-is
		containerDir = mustGatherPath
	}

	// Load metadata files
	if err := loadMetadata(containerDir, result.Metadata); err != nil {
		// Non-fatal, just log
		fmt.Printf("Warning: could not load metadata: %v\n", err)
	}

	// Load cluster-scoped resources
	clusterScopedDir := filepath.Join(containerDir, "cluster-scoped-resources")
	if _, err := os.Stat(clusterScopedDir); err == nil {
		resources, err := loadClusterScopedResources(clusterScopedDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load cluster-scoped resources: %w", err)
		}
		result.Resources = append(result.Resources, resources...)
	}

	// Load namespaced resources
	namespacesDir := filepath.Join(containerDir, "namespaces")
	if _, err := os.Stat(namespacesDir); err == nil {
		resources, namespaces, err := loadNamespacedResources(namespacesDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load namespaced resources: %w", err)
		}
		result.Resources = append(result.Resources, resources...)
		result.Namespaces = namespaces
	}

	result.Metadata.ResourceCount = len(result.Resources)
	result.Metadata.NamespaceCount = len(result.Namespaces)

	return result, nil
}

// findContainerDir finds the must-gather container directory
// Must-gather structure can be: must-gather-root/quay-io-...-sha256-.../
func findContainerDir(basePath string) (string, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return "", err
	}

	// Look for directory starting with "quay" or containing "sha256"
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

// loadMetadata loads metadata from timestamp and version files
func loadMetadata(containerDir string, metadata *LoadMetadata) error {
	// Load version
	versionFile := filepath.Join(containerDir, "version")
	if data, err := os.ReadFile(versionFile); err == nil {
		metadata.Version = strings.TrimSpace(string(data))
	}

	// Load timestamps
	timestampFile := filepath.Join(containerDir, "timestamp")
	if data, err := os.ReadFile(timestampFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "started ") {
				timeStr := strings.TrimPrefix(line, "started ")
				if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
					metadata.StartTime = t
				}
			} else if strings.HasPrefix(line, "ended ") {
				timeStr := strings.TrimPrefix(line, "ended ")
				if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
					metadata.EndTime = t
				}
			}
		}
	}

	return nil
}

// loadClusterScopedResources loads cluster-scoped resources
// Structure: cluster-scoped-resources/{api-group}/{resource-type}/{resource-name}.yaml
func loadClusterScopedResources(clusterScopedDir string) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)

	// Walk all subdirectories
	err := filepath.Walk(clusterScopedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process YAML files
		if info.IsDir() || !isYAMLFile(path) {
			return nil
		}

		// Load the resource
		resource, err := loadSingleResourceFile(path)
		if err != nil {
			fmt.Printf("Warning: failed to load %s: %v\n", path, err)
			return nil // Continue processing other files
		}

		if resource != nil {
			resources = append(resources, resource)
		}

		return nil
	})

	return resources, err
}

// loadNamespacedResources loads namespaced resources
// Structure: namespaces/{namespace}/{api-group}/{resource-type}.yaml
func loadNamespacedResources(namespacesDir string) ([]*unstructured.Unstructured, []string, error) {
	resources := make([]*unstructured.Unstructured, 0)
	namespaceSet := make(map[string]bool)

	// List namespace directories
	namespaceEntries, err := os.ReadDir(namespacesDir)
	if err != nil {
		return nil, nil, err
	}

	for _, nsEntry := range namespaceEntries {
		if !nsEntry.IsDir() {
			continue
		}

		namespace := nsEntry.Name()
		namespaceSet[namespace] = true
		nsDir := filepath.Join(namespacesDir, namespace)

		// Walk the namespace directory
		err := filepath.Walk(nsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Only process YAML files
			if info.IsDir() || !isYAMLFile(path) {
				return nil
			}

			// Load resources (can be multiple per file)
			fileResources, err := loadMultiResourceFile(path)
			if err != nil {
				fmt.Printf("Warning: failed to load %s: %v\n", path, err)
				return nil // Continue processing
			}

			resources = append(resources, fileResources...)
			return nil
		})

		if err != nil {
			return nil, nil, err
		}
	}

	// Convert namespace set to slice
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	return resources, namespaces, nil
}

// loadSingleResourceFile loads a single resource from a YAML file
func loadSingleResourceFile(path string) (*unstructured.Unstructured, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse YAML
	var obj map[string]interface{}
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, err
	}

	// Skip empty files
	if len(obj) == 0 {
		return nil, nil
	}

	return &unstructured.Unstructured{Object: obj}, nil
}

// loadMultiResourceFile loads multiple resources from a YAML file
// Handles both single resources and lists
func loadMultiResourceFile(path string) ([]*unstructured.Unstructured, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	resources := make([]*unstructured.Unstructured, 0)

	// Try to parse as a list first
	var obj map[string]interface{}
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, err
	}

	// Skip empty files
	if len(obj) == 0 {
		return resources, nil
	}

	// Check if it's a list
	kind, _, _ := unstructured.NestedString(obj, "kind")
	if strings.HasSuffix(kind, "List") {
		// Extract items from list - don't use NestedSlice as it does deep copy
		// which fails on YAML int types
		items, found := obj["items"]
		if found {
			if itemSlice, ok := items.([]interface{}); ok {
				for _, item := range itemSlice {
					if itemMap, ok := item.(map[string]interface{}); ok {
						resources = append(resources, &unstructured.Unstructured{Object: itemMap})
					}
				}
			}
		}
	} else {
		// Single resource
		resources = append(resources, &unstructured.Unstructured{Object: obj})
	}

	return resources, nil
}

// isYAMLFile returns true if the file has a YAML extension
func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}
