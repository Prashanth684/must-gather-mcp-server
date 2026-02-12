package mustgather

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceIndex provides fast in-memory access to must-gather resources
type ResourceIndex struct {
	// Primary indexes
	byGVK       map[schema.GroupVersionKind]map[string]*unstructured.Unstructured            // GVK -> name -> resource
	byNamespace map[string]map[schema.GroupVersionKind]map[string]*unstructured.Unstructured // namespace -> GVK -> name -> resource

	// Secondary indexes
	byLabel map[string]map[string]*unstructured.Unstructured // label:value -> name -> resource

	// Namespaces
	namespaces []string
}

// NewResourceIndex creates a new resource index
func NewResourceIndex() *ResourceIndex {
	return &ResourceIndex{
		byGVK:       make(map[schema.GroupVersionKind]map[string]*unstructured.Unstructured),
		byNamespace: make(map[string]map[schema.GroupVersionKind]map[string]*unstructured.Unstructured),
		byLabel:     make(map[string]map[string]*unstructured.Unstructured),
		namespaces:  make([]string, 0),
	}
}

// BuildIndex builds an index from a list of resources
func BuildIndex(resources []*unstructured.Unstructured, namespaces []string) *ResourceIndex {
	idx := NewResourceIndex()
	idx.namespaces = namespaces

	for _, resource := range resources {
		idx.Add(resource)
	}

	return idx
}

// Add adds a resource to the index
func (idx *ResourceIndex) Add(resource *unstructured.Unstructured) {
	gvk := resource.GroupVersionKind()
	name := resource.GetName()
	namespace := resource.GetNamespace()

	// Index by GVK
	if idx.byGVK[gvk] == nil {
		idx.byGVK[gvk] = make(map[string]*unstructured.Unstructured)
	}

	// Use namespaced name for namespaced resources
	key := name
	if namespace != "" {
		key = namespace + "/" + name
	}
	idx.byGVK[gvk][key] = resource

	// Index by namespace
	if namespace != "" {
		if idx.byNamespace[namespace] == nil {
			idx.byNamespace[namespace] = make(map[schema.GroupVersionKind]map[string]*unstructured.Unstructured)
		}
		if idx.byNamespace[namespace][gvk] == nil {
			idx.byNamespace[namespace][gvk] = make(map[string]*unstructured.Unstructured)
		}
		idx.byNamespace[namespace][gvk][name] = resource
	}

	// Index by labels
	labels := resource.GetLabels()
	for key, value := range labels {
		labelKey := key + "=" + value
		if idx.byLabel[labelKey] == nil {
			idx.byLabel[labelKey] = make(map[string]*unstructured.Unstructured)
		}
		idx.byLabel[labelKey][namespace+"/"+name] = resource
	}
}

// Get retrieves a resource by GVK, namespace, and name
func (idx *ResourceIndex) Get(gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
	gvkMap, found := idx.byGVK[gvk]
	if !found {
		return nil, fmt.Errorf("no resources found for GVK: %s", gvk.String())
	}

	key := name
	if namespace != "" {
		key = namespace + "/" + name
	}

	resource, found := gvkMap[key]
	if !found {
		return nil, fmt.Errorf("resource not found: %s/%s (GVK: %s)", namespace, name, gvk.String())
	}

	return resource.DeepCopy(), nil
}

// List retrieves all resources matching the given GVK and namespace
func (idx *ResourceIndex) List(gvk schema.GroupVersionKind, namespace string) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)

	if namespace != "" {
		// List resources in a specific namespace
		nsMap, found := idx.byNamespace[namespace]
		if !found {
			// Namespace exists but no resources of this type
			return resources, nil
		}

		gvkMap, found := nsMap[gvk]
		if !found {
			// GVK not found in this namespace
			return resources, nil
		}

		for _, resource := range gvkMap {
			resources = append(resources, resource.DeepCopy())
		}
	} else {
		// List resources across all namespaces
		gvkMap, found := idx.byGVK[gvk]
		if !found {
			// No resources of this type
			return resources, nil
		}

		for _, resource := range gvkMap {
			resources = append(resources, resource.DeepCopy())
		}
	}

	return resources, nil
}

// FindByLabel finds all resources matching the given label selector
// Supports simple label selectors like "key=value" or "key=value,key2=value2"
func (idx *ResourceIndex) FindByLabel(labelSelector string) ([]*unstructured.Unstructured, error) {
	if labelSelector == "" {
		return nil, nil
	}

	// Parse label selector (simple implementation for key=value pairs)
	selectors := strings.Split(labelSelector, ",")

	// Start with all resources matching the first selector
	var candidates map[string]*unstructured.Unstructured
	if len(selectors) > 0 {
		selector := strings.TrimSpace(selectors[0])
		candidates = idx.byLabel[selector]
		if candidates == nil {
			return make([]*unstructured.Unstructured, 0), nil
		}
	}

	// Filter by additional selectors
	for i := 1; i < len(selectors); i++ {
		selector := strings.TrimSpace(selectors[i])
		selectorMap := idx.byLabel[selector]
		if selectorMap == nil {
			// No resources match this selector
			return make([]*unstructured.Unstructured, 0), nil
		}

		// Intersect with candidates
		newCandidates := make(map[string]*unstructured.Unstructured)
		for key, resource := range candidates {
			if _, found := selectorMap[key]; found {
				newCandidates[key] = resource
			}
		}
		candidates = newCandidates
	}

	// Convert to slice
	resources := make([]*unstructured.Unstructured, 0, len(candidates))
	for _, resource := range candidates {
		resources = append(resources, resource.DeepCopy())
	}

	return resources, nil
}

// ListGVKs returns all GroupVersionKinds in the index
func (idx *ResourceIndex) ListGVKs() []schema.GroupVersionKind {
	gvks := make([]schema.GroupVersionKind, 0, len(idx.byGVK))
	for gvk := range idx.byGVK {
		gvks = append(gvks, gvk)
	}
	return gvks
}

// ListNamespaces returns all namespaces
func (idx *ResourceIndex) ListNamespaces() []string {
	return idx.namespaces
}

// Count returns the total number of resources in the index
func (idx *ResourceIndex) Count() int {
	count := 0
	for _, gvkMap := range idx.byGVK {
		count += len(gvkMap)
	}
	return count
}
