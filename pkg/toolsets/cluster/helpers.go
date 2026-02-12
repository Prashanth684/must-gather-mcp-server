package cluster

import "k8s.io/apimachinery/pkg/runtime/schema"

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
