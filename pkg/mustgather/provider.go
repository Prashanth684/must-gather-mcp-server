package mustgather

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Provider implements the MustGatherProvider interface
type Provider struct {
	path     string
	index    *ResourceIndex
	metadata *api.MustGatherMetadata
}

// NewProvider creates a new must-gather provider
func NewProvider(mustGatherPath string) (*Provider, error) {
	fmt.Printf("Loading must-gather from: %s\n", mustGatherPath)

	// Load the must-gather
	result, err := Load(mustGatherPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load must-gather: %w", err)
	}

	fmt.Printf("Loaded %d resources from %d namespaces\n", result.Metadata.ResourceCount, result.Metadata.NamespaceCount)

	// Build index
	fmt.Printf("Building resource index...\n")
	index := BuildIndex(result.Resources, result.Namespaces)
	fmt.Printf("Index built with %d resources\n", index.Count())

	// Convert metadata
	metadata := &api.MustGatherMetadata{
		Path:           result.Metadata.Path,
		Version:        result.Metadata.Version,
		StartTime:      result.Metadata.StartTime,
		EndTime:        result.Metadata.EndTime,
		ResourceCount:  result.Metadata.ResourceCount,
		NamespaceCount: result.Metadata.NamespaceCount,
	}

	return &Provider{
		path:     mustGatherPath,
		index:    index,
		metadata: metadata,
	}, nil
}

// GetMetadata returns must-gather metadata
func (p *Provider) GetMetadata() *api.MustGatherMetadata {
	return p.metadata
}

// GetResource retrieves a specific resource
func (p *Provider) GetResource(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
	return p.index.Get(gvk, namespace, name)
}

// ListResources lists resources matching the given criteria
func (p *Provider) ListResources(ctx context.Context, gvk schema.GroupVersionKind, namespace string, opts api.ListOptions) (*unstructured.UnstructuredList, error) {
	var resources []*unstructured.Unstructured
	var err error

	// If label selector is provided, use it
	if opts.LabelSelector != "" {
		resources, err = p.index.FindByLabel(opts.LabelSelector)
		if err != nil {
			return nil, err
		}

		// Filter by GVK and namespace
		filtered := make([]*unstructured.Unstructured, 0)
		for _, resource := range resources {
			if resource.GroupVersionKind() != gvk {
				continue
			}
			if namespace != "" && resource.GetNamespace() != namespace {
				continue
			}
			filtered = append(filtered, resource)
		}
		resources = filtered
	} else {
		// Use GVK and namespace directly
		resources, err = p.index.List(gvk, namespace)
		if err != nil {
			return nil, err
		}
	}

	// Apply field selector if provided (basic implementation)
	if opts.FieldSelector != "" {
		resources, err = p.applyFieldSelector(resources, opts.FieldSelector)
		if err != nil {
			return nil, err
		}
	}

	// Apply limit
	if opts.Limit > 0 && len(resources) > opts.Limit {
		resources = resources[:opts.Limit]
	}

	return &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": gvk.GroupVersion().String(),
			"kind":       gvk.Kind + "List",
		},
		Items: convertToUnstructuredSlice(resources),
	}, nil
}

// ListNamespaces returns all namespaces
func (p *Provider) ListNamespaces(ctx context.Context) ([]string, error) {
	return p.index.ListNamespaces(), nil
}

// GetETCDHealth returns ETCD health information
func (p *Provider) GetETCDHealth() (*api.ETCDHealth, error) {
	// Find container directory
	containerDir, err := findContainerDir(p.path)
	if err != nil {
		containerDir = p.path
	}

	etcdInfoDir := filepath.Join(containerDir, "etcd_info")

	// Read endpoint_health.json
	healthFile := filepath.Join(etcdInfoDir, "endpoint_health.json")
	healthData, err := os.ReadFile(healthFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read ETCD health data: %w", err)
	}

	// Parse health data - handle different formats
	// First, try to detect if it's an array or object by inspecting the JSON
	var rawData interface{}
	if err := json.Unmarshal(healthData, &rawData); err != nil {
		return nil, fmt.Errorf("failed to parse ETCD health data: %w", err)
	}

	type healthEndpoint struct {
		Endpoint string      `json:"endpoint"`
		Health   interface{} `json:"health"` // Can be bool, string "true"/"false", or nested object
	}

	var endpoints []healthEndpoint

	// Check if it's an array or single object
	switch rawData.(type) {
	case []interface{}:
		// It's an array
		if err := json.Unmarshal(healthData, &endpoints); err != nil {
			return nil, fmt.Errorf("failed to parse ETCD health data array: %w", err)
		}
	case map[string]interface{}:
		// It's a single object
		var singleEndpoint healthEndpoint
		if err := json.Unmarshal(healthData, &singleEndpoint); err != nil {
			return nil, fmt.Errorf("failed to parse ETCD health data object: %w", err)
		}
		endpoints = []healthEndpoint{singleEndpoint}
	default:
		return nil, fmt.Errorf("unexpected ETCD health data format: %T", rawData)
	}

	health := &api.ETCDHealth{
		Healthy:   true,
		Endpoints: make([]api.ETCDEndpoint, len(endpoints)),
		Alarms:    make([]string, 0),
	}

	for i, ep := range endpoints {
		healthStatus := "healthy"
		isHealthy := false

		// Parse health value which can be bool, string, or object
		switch h := ep.Health.(type) {
		case bool:
			isHealthy = h
		case string:
			isHealthy = (h == "true" || h == "healthy")
		case map[string]interface{}:
			// Health might be nested object with "health" field
			if healthVal, ok := h["health"]; ok {
				switch hv := healthVal.(type) {
				case bool:
					isHealthy = hv
				case string:
					isHealthy = (hv == "true" || hv == "healthy")
				}
			}
		}

		if !isHealthy {
			healthStatus = "unhealthy"
			health.Healthy = false
		}

		health.Endpoints[i] = api.ETCDEndpoint{
			Address: ep.Endpoint,
			Health:  healthStatus,
		}
	}

	// Read alarm_list.json if it exists
	alarmFile := filepath.Join(etcdInfoDir, "alarm_list.json")
	if alarmData, err := os.ReadFile(alarmFile); err == nil {
		var alarms []struct {
			Alarm string `json:"alarm"`
		}
		if err := json.Unmarshal(alarmData, &alarms); err == nil {
			for _, alarm := range alarms {
				health.Alarms = append(health.Alarms, alarm.Alarm)
			}
		}
	}

	return health, nil
}

// GetETCDObjectCount returns ETCD object counts by resource type
func (p *Provider) GetETCDObjectCount() (map[string]int64, error) {
	// Find container directory
	containerDir, err := findContainerDir(p.path)
	if err != nil {
		containerDir = p.path
	}

	etcdInfoDir := filepath.Join(containerDir, "etcd_info")
	objectCountFile := filepath.Join(etcdInfoDir, "object_count.json")

	data, err := os.ReadFile(objectCountFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read ETCD object count data: %w", err)
	}

	// Parse object count (format is map[string]string with numeric values)
	var rawCounts map[string]string
	if err := json.Unmarshal(data, &rawCounts); err != nil {
		return nil, fmt.Errorf("failed to parse ETCD object count data: %w", err)
	}

	// Convert string values to int64
	counts := make(map[string]int64)
	for resource, countStr := range rawCounts {
		var count int64
		fmt.Sscanf(countStr, "%d", &count)
		counts[resource] = count
	}

	return counts, nil
}

// applyFieldSelector applies a basic field selector to resources
// Supports simple field selectors like "status.phase=Failed"
func (p *Provider) applyFieldSelector(resources []*unstructured.Unstructured, fieldSelector string) ([]*unstructured.Unstructured, error) {
	// Parse field selector (simple implementation)
	parts := strings.SplitN(fieldSelector, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid field selector: %s", fieldSelector)
	}

	field := parts[0]
	value := parts[1]

	// Split field path
	fieldPath := strings.Split(field, ".")

	filtered := make([]*unstructured.Unstructured, 0)
	for _, resource := range resources {
		// Navigate to the field
		fieldValue, found, err := unstructured.NestedString(resource.Object, fieldPath...)
		if err != nil || !found {
			continue
		}

		if fieldValue == value {
			filtered = append(filtered, resource)
		}
	}

	return filtered, nil
}

// convertToUnstructuredSlice converts []*unstructured.Unstructured to []unstructured.Unstructured
func convertToUnstructuredSlice(resources []*unstructured.Unstructured) []unstructured.Unstructured {
	result := make([]unstructured.Unstructured, len(resources))
	for i, resource := range resources {
		result[i] = *resource
	}
	return result
}
