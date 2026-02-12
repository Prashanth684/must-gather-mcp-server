package api

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MustGatherProvider abstracts access to must-gather data
type MustGatherProvider interface {
	// Metadata returns must-gather metadata
	GetMetadata() *MustGatherMetadata

	// Resource access
	GetResource(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error)
	ListResources(ctx context.Context, gvk schema.GroupVersionKind, namespace string, opts ListOptions) (*unstructured.UnstructuredList, error)

	// Namespace operations
	ListNamespaces(ctx context.Context) ([]string, error)

	// Specialized access
	GetETCDHealth() (*ETCDHealth, error)
	GetETCDObjectCount() (map[string]int64, error)

	// Log access
	GetPodLog(opts PodLogOptions) (string, error)
	ListPodContainers(namespace, pod string) ([]string, error)

	// Node diagnostics
	GetNodeDiagnostics(nodeName string) (*NodeDiagnostics, error)
	ListNodes() ([]string, error)
}

// MustGatherMetadata contains metadata about the must-gather
type MustGatherMetadata struct {
	Path           string
	Version        string
	StartTime      time.Time
	EndTime        time.Time
	ResourceCount  int
	NamespaceCount int
}

// ListOptions contains options for listing resources
type ListOptions struct {
	LabelSelector string
	FieldSelector string
	Limit         int
}

// ETCDHealth contains ETCD health information
type ETCDHealth struct {
	Healthy   bool
	Endpoints []ETCDEndpoint
	Alarms    []string
}

// ETCDEndpoint represents an ETCD endpoint
type ETCDEndpoint struct {
	Address string
	Health  string
}
