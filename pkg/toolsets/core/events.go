package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func eventsTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "events_list",
				Description: "List Kubernetes events with filtering by type (Warning, Normal), namespace, and resource. Essential for troubleshooting failures and unexpected behavior.",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"type": {
							Type:        "string",
							Description: "Filter by event type: all, Warning, Normal (default: all)",
							Enum:        []interface{}{"all", "Warning", "Normal"},
						},
						"namespace": {
							Type:        "string",
							Description: "Filter by namespace (optional)",
						},
						"resource": {
							Type:        "string",
							Description: "Filter by involved object name (partial match, optional)",
						},
						"reason": {
							Type:        "string",
							Description: "Filter by reason (partial match, e.g., 'Failed', 'Unhealthy', 'BackOff')",
						},
						"limit": {
							Type:        "integer",
							Description: "Maximum number of events to return (default: 100)",
						},
					},
				},
			},
			Handler: eventsList,
		},
		{
			Tool: api.Tool{
				Name:        "events_timeline",
				Description: "Show events in chronological order to understand the sequence of what happened during an incident or failure",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"namespace": {
							Type:        "string",
							Description: "Filter by namespace (optional)",
						},
						"hours": {
							Type:        "integer",
							Description: "Show events from the last N hours (default: 1)",
						},
						"type": {
							Type:        "string",
							Description: "Filter by event type: all, Warning, Normal (default: all)",
							Enum:        []interface{}{"all", "Warning", "Normal"},
						},
						"limit": {
							Type:        "integer",
							Description: "Maximum number of events to return (default: 100)",
						},
					},
				},
			},
			Handler: eventsTimeline,
		},
		{
			Tool: api.Tool{
				Name:        "events_by_resource",
				Description: "Get all events for a specific Kubernetes resource (pod, node, deployment, etc.) to debug resource-specific issues",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"name": {
							Type:        "string",
							Description: "Resource name",
						},
						"namespace": {
							Type:        "string",
							Description: "Namespace (required for namespaced resources)",
						},
						"kind": {
							Type:        "string",
							Description: "Resource kind (e.g., Pod, Node, Deployment) - optional, searches all kinds if omitted",
						},
					},
					Required: []string{"name"},
				},
			},
			Handler: eventsByResource,
		},
	}
}

type eventInfo struct {
	Type           string
	Reason         string
	Message        string
	InvolvedObject string
	InvolvedKind   string
	Count          int64
	FirstTimestamp time.Time
	LastTimestamp  time.Time
	Namespace      string
}

func eventsList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	typeFilter := params.GetString("type", "all")
	namespace := params.GetString("namespace", "")
	resource := params.GetString("resource", "")
	reasonFilter := params.GetString("reason", "")
	limit := params.GetInt("limit", 100)

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Event",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, namespace, api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list events: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No events found", nil), nil
	}

	// Parse and filter events
	events := make([]eventInfo, 0)
	for i := range list.Items {
		event := &list.Items[i]

		eventType, _, _ := unstructured.NestedString(event.Object, "type")
		reason, _, _ := unstructured.NestedString(event.Object, "reason")
		message, _, _ := unstructured.NestedString(event.Object, "message")
		count, _, _ := unstructured.NestedInt64(event.Object, "count")
		ns := event.GetNamespace()

		// Involved object
		involvedObj, _, _ := unstructured.NestedString(event.Object, "involvedObject", "name")
		involvedKind, _, _ := unstructured.NestedString(event.Object, "involvedObject", "kind")

		// Apply filters
		if typeFilter != "all" && eventType != typeFilter {
			continue
		}
		if resource != "" && !strings.Contains(strings.ToLower(involvedObj), strings.ToLower(resource)) {
			continue
		}
		if reasonFilter != "" && !strings.Contains(strings.ToLower(reason), strings.ToLower(reasonFilter)) {
			continue
		}

		// Parse timestamps
		firstTimestamp, _ := time.Parse(time.RFC3339, getTimestamp(event, "firstTimestamp"))
		lastTimestamp, _ := time.Parse(time.RFC3339, getTimestamp(event, "lastTimestamp"))

		events = append(events, eventInfo{
			Type:           eventType,
			Reason:         reason,
			Message:        message,
			InvolvedObject: involvedObj,
			InvolvedKind:   involvedKind,
			Count:          count,
			FirstTimestamp: firstTimestamp,
			LastTimestamp:  lastTimestamp,
			Namespace:      ns,
		})
	}

	// Sort by last timestamp (most recent first)
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp.After(events[j].LastTimestamp)
	})

	// Apply limit
	if limit > 0 && limit < len(events) {
		events = events[:limit]
	}

	// Format output
	output := fmt.Sprintf("Events (showing %d of %d total):\n", len(events), len(list.Items))
	output += strings.Repeat("=", 100) + "\n\n"

	// Group by type
	warnings := 0
	normals := 0
	for _, e := range events {
		if e.Type == "Warning" {
			warnings++
		} else {
			normals++
		}
	}
	output += fmt.Sprintf("Summary: %d Warning, %d Normal\n\n", warnings, normals)

	for _, e := range events {
		symbol := "ℹ"
		if e.Type == "Warning" {
			symbol = "⚠"
		}

		output += fmt.Sprintf("%s [%s] %s/%s\n", symbol, e.Type, e.InvolvedKind, e.InvolvedObject)
		if e.Namespace != "" {
			output += fmt.Sprintf("  Namespace: %s\n", e.Namespace)
		}
		output += fmt.Sprintf("  Reason: %s\n", e.Reason)
		output += fmt.Sprintf("  Message: %s\n", e.Message)
		if e.Count > 1 {
			output += fmt.Sprintf("  Count: %d (first: %s, last: %s)\n",
				e.Count, e.FirstTimestamp.Format(time.RFC3339), e.LastTimestamp.Format(time.RFC3339))
		} else {
			output += fmt.Sprintf("  Time: %s\n", e.LastTimestamp.Format(time.RFC3339))
		}
		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func eventsTimeline(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	namespace := params.GetString("namespace", "")
	hours := params.GetInt("hours", 1)
	typeFilter := params.GetString("type", "all")
	limit := params.GetInt("limit", 100)

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Event",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, namespace, api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list events: %w", err)), nil
	}

	if len(list.Items) == 0 {
		return api.NewToolCallResult("No events found", nil), nil
	}

	// Parse events
	events := make([]eventInfo, 0)
	cutoffTime := time.Now().Add(-time.Duration(hours) * time.Hour)

	for i := range list.Items {
		event := &list.Items[i]

		eventType, _, _ := unstructured.NestedString(event.Object, "type")
		reason, _, _ := unstructured.NestedString(event.Object, "reason")
		message, _, _ := unstructured.NestedString(event.Object, "message")
		ns := event.GetNamespace()

		// Involved object
		involvedObj, _, _ := unstructured.NestedString(event.Object, "involvedObject", "name")
		involvedKind, _, _ := unstructured.NestedString(event.Object, "involvedObject", "kind")

		// Apply filters
		if typeFilter != "all" && eventType != typeFilter {
			continue
		}

		// Parse timestamp
		lastTimestamp, _ := time.Parse(time.RFC3339, getTimestamp(event, "lastTimestamp"))

		// Filter by time
		if lastTimestamp.Before(cutoffTime) {
			continue
		}

		events = append(events, eventInfo{
			Type:           eventType,
			Reason:         reason,
			Message:        message,
			InvolvedObject: involvedObj,
			InvolvedKind:   involvedKind,
			LastTimestamp:  lastTimestamp,
			Namespace:      ns,
		})
	}

	// Sort chronologically (oldest first)
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp.Before(events[j].LastTimestamp)
	})

	// Apply limit
	if limit > 0 && limit < len(events) {
		events = events[:limit]
	}

	// Format output
	output := fmt.Sprintf("Event Timeline (last %d hours, showing %d events):\n", hours, len(events))
	output += strings.Repeat("=", 100) + "\n\n"

	for _, e := range events {
		symbol := "ℹ"
		if e.Type == "Warning" {
			symbol = "⚠"
		}

		timeStr := e.LastTimestamp.Format("15:04:05")
		output += fmt.Sprintf("%s %s | %s/%s: %s - %s\n",
			timeStr, symbol, e.InvolvedKind, e.InvolvedObject, e.Reason, e.Message)
	}

	return api.NewToolCallResult(output, nil), nil
}

func eventsByResource(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	name := params.GetString("name", "")
	namespace := params.GetString("namespace", "")
	kindFilter := params.GetString("kind", "")

	if name == "" {
		return api.NewToolCallResult("", fmt.Errorf("resource name is required")), nil
	}

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Event",
	}

	list, err := params.MustGatherProvider.ListResources(context.Background(), gvk, namespace, api.ListOptions{})
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to list events: %w", err)), nil
	}

	// Filter events for this resource
	events := make([]eventInfo, 0)

	for i := range list.Items {
		event := &list.Items[i]

		involvedObj, _, _ := unstructured.NestedString(event.Object, "involvedObject", "name")
		involvedKind, _, _ := unstructured.NestedString(event.Object, "involvedObject", "kind")

		// Match resource name
		if involvedObj != name {
			continue
		}

		// Match kind if specified
		if kindFilter != "" && involvedKind != kindFilter {
			continue
		}

		eventType, _, _ := unstructured.NestedString(event.Object, "type")
		reason, _, _ := unstructured.NestedString(event.Object, "reason")
		message, _, _ := unstructured.NestedString(event.Object, "message")
		count, _, _ := unstructured.NestedInt64(event.Object, "count")

		firstTimestamp, _ := time.Parse(time.RFC3339, getTimestamp(event, "firstTimestamp"))
		lastTimestamp, _ := time.Parse(time.RFC3339, getTimestamp(event, "lastTimestamp"))

		events = append(events, eventInfo{
			Type:           eventType,
			Reason:         reason,
			Message:        message,
			InvolvedObject: involvedObj,
			InvolvedKind:   involvedKind,
			Count:          count,
			FirstTimestamp: firstTimestamp,
			LastTimestamp:  lastTimestamp,
		})
	}

	if len(events) == 0 {
		return api.NewToolCallResult(fmt.Sprintf("No events found for resource: %s", name), nil), nil
	}

	// Sort chronologically
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp.Before(events[j].LastTimestamp)
	})

	// Format output
	kindDisplay := events[0].InvolvedKind
	output := fmt.Sprintf("Events for %s/%s:\n", kindDisplay, name)
	output += strings.Repeat("=", 80) + "\n\n"
	output += fmt.Sprintf("Found %d events\n\n", len(events))

	for _, e := range events {
		symbol := "ℹ"
		if e.Type == "Warning" {
			symbol = "⚠"
		}

		output += fmt.Sprintf("%s [%s] %s\n", symbol, e.Type, e.Reason)
		output += fmt.Sprintf("  %s\n", e.Message)
		if e.Count > 1 {
			output += fmt.Sprintf("  Count: %d (first: %s, last: %s)\n",
				e.Count, e.FirstTimestamp.Format(time.RFC3339), e.LastTimestamp.Format(time.RFC3339))
		} else {
			output += fmt.Sprintf("  Time: %s\n", e.LastTimestamp.Format(time.RFC3339))
		}
		output += "\n"
	}

	return api.NewToolCallResult(output, nil), nil
}

func getTimestamp(event *unstructured.Unstructured, field string) string {
	ts, _, _ := unstructured.NestedString(event.Object, field)
	if ts == "" {
		// Try eventTime
		ts, _, _ = unstructured.NestedString(event.Object, "eventTime")
	}
	return ts
}
