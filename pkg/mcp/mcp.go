package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
	"github.com/openshift/must-gather-mcp-server/pkg/version"
)

// Server represents the MCP server
type Server struct {
	server   *mcp.Server
	provider api.MustGatherProvider
	toolsets []api.Toolset
}

// NewServer creates a new MCP server
func NewServer(provider api.MustGatherProvider, toolsets []api.Toolset) (*Server, error) {
	s := &Server{
		provider: provider,
		toolsets: toolsets,
	}

	// Create MCP server
	s.server = mcp.NewServer(
		&mcp.Implementation{
			Name:    version.BinaryName,
			Version: version.Version,
		},
		&mcp.ServerOptions{
			Capabilities: &mcp.ServerCapabilities{
				Tools: &mcp.ToolCapabilities{},
			},
		},
	)

	// Register all tools
	if err := s.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return s, nil
}

// ServeStdio starts the MCP server with STDIO transport
func (s *Server) ServeStdio(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.LoggingTransport{
		Transport: &mcp.StdioTransport{},
		Writer:    os.Stderr,
	})
}

// ServeHTTP starts the MCP server with HTTP/SSE transport
func (s *Server) ServeHTTP(ctx context.Context, addr string) error {
	// Create SSE handler
	handler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
		return s.server
	}, nil)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	fmt.Printf("Starting MCP server on http://%s\n", addr)
	fmt.Printf("SSE endpoint: http://%s/sse (GET request to establish connection)\n", addr)
	fmt.Printf("Message endpoint: http://%s/messages/<session-id> (POST for sending messages)\n", addr)

	// Start HTTP server
	errChan := make(chan error, 1)
	go func() {
		errChan <- httpServer.ListenAndServe()
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		fmt.Println("Shutting down HTTP server...")
		return httpServer.Shutdown(context.Background())
	case err := <-errChan:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("HTTP server error: %w", err)
		}
		return nil
	}
}

// registerTools registers all tools from toolsets
func (s *Server) registerTools() error {
	for _, toolset := range s.toolsets {
		tools := toolset.GetTools()
		fmt.Printf("Registering %d tools from toolset: %s\n", len(tools), toolset.Name())

		for _, tool := range tools {
			if err := s.registerTool(tool); err != nil {
				return fmt.Errorf("failed to register tool %s: %w", tool.Tool.Name, err)
			}
		}
	}

	return nil
}

// registerTool registers a single tool with the MCP server
func (s *Server) registerTool(serverTool api.ServerTool) error {
	// Convert to MCP SDK format
	mcpTool, handler, err := ServerToolToMCPTool(s, serverTool)
	if err != nil {
		return err
	}

	// Register with MCP server
	s.server.AddTool(mcpTool, handler)
	return nil
}

// ServerToolToMCPTool converts our ServerTool to MCP SDK format
func ServerToolToMCPTool(s *Server, tool api.ServerTool) (*mcp.Tool, mcp.ToolHandler, error) {
	mcpTool := &mcp.Tool{
		Name:        tool.Tool.Name,
		Description: tool.Tool.Description,
		InputSchema: tool.Tool.InputSchema,
	}

	mcpHandler := func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Convert request to our internal format
		toolCallRequest, err := MCPRequestToToolCallRequest(request)
		if err != nil {
			return nil, fmt.Errorf("failed to convert request for tool %s: %w", tool.Tool.Name, err)
		}

		// Call the tool handler
		result, err := tool.Handler(api.ToolHandlerParams{
			Context:            ctx,
			MustGatherProvider: s.provider,
			ToolCallRequest:    toolCallRequest,
		})
		if err != nil {
			return nil, err
		}

		// Return result
		return NewTextResult(result.Content, result.Error), nil
	}

	return mcpTool, mcpHandler, nil
}

// ToolCallRequest implements api.ToolCallRequest
type ToolCallRequest struct {
	Name      string
	arguments map[string]any
}

var _ api.ToolCallRequest = (*ToolCallRequest)(nil)

// GetArguments returns the tool call arguments
func (t *ToolCallRequest) GetArguments() map[string]any {
	return t.arguments
}

// MCPRequestToToolCallRequest converts MCP request to our internal format
func MCPRequestToToolCallRequest(request *mcp.CallToolRequest) (*ToolCallRequest, error) {
	params, ok := request.GetParams().(*mcp.CallToolParamsRaw)
	if !ok {
		return nil, errors.New("invalid tool call parameters")
	}

	var arguments map[string]any
	if err := json.Unmarshal(params.Arguments, &arguments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	return &ToolCallRequest{
		Name:      params.Name,
		arguments: arguments,
	}, nil
}

// NewTextResult creates a text result
func NewTextResult(content string, err error) *mcp.CallToolResult {
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: content,
			},
		},
	}
}
