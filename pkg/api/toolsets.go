package api

import (
	"context"

	"github.com/google/jsonschema-go/jsonschema"
)

// ServerTool represents a tool that can be registered with the MCP server
type ServerTool struct {
	Tool    Tool            // Tool metadata and schema
	Handler ToolHandlerFunc // Function to execute the tool
}

// Tool represents a tool definition
type Tool struct {
	Name        string
	Description string
	InputSchema *jsonschema.Schema
}

// Toolset represents a collection of related tools
type Toolset interface {
	// Name returns the toolset name
	Name() string

	// GetTools returns all tools in this toolset
	GetTools() []ServerTool
}

// ToolCallRequest provides access to tool call arguments
type ToolCallRequest interface {
	GetArguments() map[string]any
}

// ToolCallResult represents the result of a tool call
type ToolCallResult struct {
	Content string
	Error   error
}

// NewToolCallResult creates a new ToolCallResult
func NewToolCallResult(content string, err error) *ToolCallResult {
	return &ToolCallResult{
		Content: content,
		Error:   err,
	}
}

// ToolHandlerFunc is the signature for tool handler functions
type ToolHandlerFunc func(params ToolHandlerParams) (*ToolCallResult, error)

// ToolHandlerParams contains all parameters passed to a tool handler
type ToolHandlerParams struct {
	context.Context
	MustGatherProvider MustGatherProvider
	ToolCallRequest    ToolCallRequest
}

// GetString returns a string argument value with default
func (p ToolHandlerParams) GetString(key, defaultValue string) string {
	args := p.ToolCallRequest.GetArguments()
	if val, ok := args[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// GetBool returns a boolean argument value with default
func (p ToolHandlerParams) GetBool(key string, defaultValue bool) bool {
	args := p.ToolCallRequest.GetArguments()
	if val, ok := args[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// GetInt returns an int argument value with default
func (p ToolHandlerParams) GetInt(key string, defaultValue int) int {
	args := p.ToolCallRequest.GetArguments()
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return defaultValue
}
