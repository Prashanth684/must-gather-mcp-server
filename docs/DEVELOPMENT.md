# Development Guide

Guide for developers working on must-gather-mcp-server, including architecture, testing, and deployment.

## Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        MCP Client                               │
│                  (Claude Desktop, Goose, etc.)                  │
└────────────────────────────┬────────────────────────────────────┘
                             │ MCP Protocol
                             │ (STDIO or HTTP/SSE)
┌────────────────────────────▼────────────────────────────────────┐
│                   Must-Gather MCP Server                        │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              61 MCP Tools (5 Toolsets)                   │   │
│  │  Cluster | Core | Diagnostics | Network | Monitoring    │   │
│  └─────────────────────┬────────────────────────────────────┘   │
│                        │                                         │
│  ┌─────────────────────▼──────────────┬──────────────────────┐  │
│  │    In-Memory Index                 │  On-Demand Files     │  │
│  │  • ~11k YAML resources             │  • Pod logs          │  │
│  │  • GVK, namespace, labels          │  • Node diagnostics  │  │
│  │  • <50ms lookups                   │  • ETCD metrics      │  │
│  └────────────────────────────────────┴──────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                   Must-Gather Archive                           │
│  cluster-scoped-resources/ | namespaces/ | nodes/ | etcd_info/  │
│  network_logs/ | pod_network_connectivity_check/ | monitoring/  │
└─────────────────────────────────────────────────────────────────┘
```

### Data Loading

1. **Startup**: Loads YAML resources from cluster-scoped-resources/ and namespaces/
2. **Indexing**: Builds in-memory index by GVK, namespace, and labels (~5-10s)
3. **Query**: Fast lookups using indexed data (<50ms)
4. **Logs**: Loaded on-demand when tools are called (not indexed)

### Directory Structure

```
must-gather/
├── quay-io-okd-scos-content-sha256-.../ (container directory)
│   ├── cluster-scoped-resources/      # Cluster-wide resources
│   │   ├── config.openshift.io/       # Cluster config, operators, version
│   │   ├── core/                      # Nodes, PVs
│   │   ├── machineconfiguration.openshift.io/
│   │   ├── rbac.authorization.k8s.io/
│   │   └── storage.k8s.io/
│   ├── namespaces/                    # Namespaced resources
│   │   └── {namespace}/
│   │       ├── core/                  # Pods, services, etc.
│   │       └── pods/                  # Pod logs
│   ├── nodes/                         # Node diagnostics
│   │   └── {node}/
│   │       ├── kubelet.log.gz
│   │       ├── lscpu
│   │       ├── lspci
│   │       └── dmesg
│   ├── host_service_logs/             # Systemd service logs
│   │   └── masters/
│   │       └── {node}/
│   ├── static-pods/                   # Static pod termination logs
│   ├── etcd_info/                     # ETCD health and metrics
│   ├── network_logs/                  # Network scale and OVN metrics
│   ├── pod_network_connectivity_check/ # Connectivity test results
│   └── monitoring/                    # Prometheus and AlertManager data
│       ├── alertmanager/
│       ├── prometheus/
│       │   ├── prometheus-k8s-0/
│       │   ├── prometheus-k8s-1/
│       │   ├── rules.json
│       │   └── status/
│       └── servicemonitors/
```

### Tool Categories

**Indexed Resources** (fast queries):
- Cluster resources (operators, version, infrastructure, nodes)
- Core resources (pods, services, configmaps)
- All Kubernetes/OpenShift API resources

**On-Demand Data** (read from files):
- Pod container logs
- Node diagnostics (kubelet logs, sysinfo, hardware info)
- ETCD detailed status
- Network connectivity checks
- Monitoring data (Prometheus, AlertManager)

## Building

### Requirements
- Go 1.25 or later
- Make

### Build Commands

```bash
# Build for current platform
make build

# Build for all platforms
make build-all-platforms

# Run tests
make test

# Run linter
make lint

# Format code
make fmt

# Clean build artifacts
make clean
```

### Build Output

```bash
$ make build
go fmt ./...
go mod tidy
go build -ldflags "-X main.version=dev" -o _output/bin/must-gather-mcp-server ./cmd/must-gather-mcp-server

# Binary location
_output/bin/must-gather-mcp-server
```

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Run specific package tests
go test ./pkg/config/... -v
go test ./pkg/http/... -v

# Run with coverage
go test ./... -cover

# Generate coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Test Coverage

Current test coverage:
- ✅ `pkg/config` - 100% pass (15 tests)
- ✅ `pkg/http` - 100% pass (28 tests)
- ✅ All packages build successfully
- ✅ Binary size: ~30MB

### Testing with MCP Inspector

Install and run the MCP inspector:

```bash
npx @modelcontextprotocol/inspector _output/bin/must-gather-mcp-server \
  --must-gather-path /path/to/must-gather
```

### Testing with Claude Desktop

Add to Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "must-gather": {
      "command": "/path/to/must-gather-mcp-server",
      "args": [
        "--must-gather-path",
        "/path/to/must-gather"
      ]
    }
  }
}
```

### Testing with Goose

#### 1. Build the Server

```bash
make build
```

#### 2. Start in HTTP Mode

```bash
./must-gather-mcp-server \
  --must-gather-path /path/to/must-gather \
  --http \
  --port 8080
```

#### 3. Configure Goose

Add to Goose configuration file:

```yaml
mcp_servers:
  must-gather:
    url: http://localhost:8080/sse
```

#### 4. Start Goose

```bash
goose session start
```

## Performance

### Startup Performance
- Load time: ~5-10 seconds for 11,000 resources
- Index time: ~2-3 seconds
- Memory usage: ~100-200MB (depending on cluster size)

### Query Performance
- Indexed queries: <50ms
- Log retrieval: <500ms (most cases)
- Kubelet log decompression: <1s (371K .gz file)

### HTTP Server Performance
- Concurrent request handling (Go standard library)
- Connection pooling for OIDC verification
- Efficient JWT parsing
- OpenTelemetry with minimal overhead

### Memory Usage
- Binary size: ~30MB (with all dependencies)
- Startup memory: <50MB
- Per-request overhead: <1MB

### Latency Impact
- Unprotected mode: ~0ms auth overhead
- JWT offline validation: <1ms
- OIDC online validation: ~5-50ms (network dependent)
- OpenTelemetry tracing: <1ms

## Project Structure

```
must-gather-mcp-server/
├── cmd/
│   └── must-gather-mcp-server/
│       └── cmd/
│           └── root.go            # CLI entry point
├── pkg/
│   ├── config/                    # Configuration management
│   │   ├── config.go
│   │   ├── config_test.go
│   │   ├── defaults.go
│   │   └── telemetry.go
│   ├── http/                      # HTTP server & auth
│   │   ├── authorization.go
│   │   ├── authorization_test.go
│   │   ├── middleware.go
│   │   ├── middleware_test.go
│   │   ├── http.go
│   │   ├── wellknown.go
│   │   └── wellknown_test.go
│   ├── loader/                    # Must-gather loading
│   │   └── loader.go
│   ├── mcp/                       # MCP server
│   │   └── mcp.go
│   └── toolsets/                  # Tool implementations
│       ├── cluster/
│       ├── core/
│       ├── diagnostics/
│       ├── monitoring/
│       └── network/
├── docs/                          # Documentation
│   ├── AUTHENTICATION.md
│   ├── TOOLS.md
│   └── DEVELOPMENT.md
├── Makefile
├── README.md
├── go.mod
└── go.sum
```

## Adding New Tools

### 1. Create Tool Function

```go
// pkg/toolsets/mytoolset/tool.go
func MyNewTool(ctx context.Context, loader *loader.Loader, args map[string]interface{}) (string, error) {
    // Extract parameters
    param := args["param"].(string)

    // Query data from loader
    resources := loader.GetResourcesByKind("MyKind")

    // Process and format output
    var output strings.Builder
    // ...

    return output.String(), nil
}
```

### 2. Register Tool

```go
// pkg/toolsets/mytoolset/toolset.go
func NewToolset(loader *loader.Loader) *mcp.Toolset {
    return &mcp.Toolset{
        Name:  "mytoolset",
        Tools: []mcp.Tool{
            {
                Name:        "my_new_tool",
                Description: "Does something useful",
                InputSchema: map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "param": map[string]interface{}{
                            "type":        "string",
                            "description": "Parameter description",
                        },
                    },
                    "required": []string{"param"},
                },
                Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
                    return MyNewTool(ctx, loader, args)
                },
            },
        },
    }
}
```

### 3. Import in Root Command

```go
// cmd/must-gather-mcp-server/cmd/root.go
import (
    "github.com/openshift/must-gather-mcp-server/pkg/toolsets/mytoolset"
)

// In runServer()
mcpServer.RegisterToolset(mytoolset.NewToolset(loader))
```

## Deployment

### Development

```bash
# Unprotected, verbose logging
./must-gather-mcp-server \
  --must-gather-path /data/must-gather \
  --http \
  --port 8080 \
  --log-level 5
```

### Staging

Create `staging-config.toml`:
```toml
port = "8080"
log_level = 2
must_gather_path = "/data/must-gather"

require_oauth = true
oauth_audience = "must-gather-mcp-staging"
authorization_url = "https://keycloak-staging.example.com/realms/openshift"

[telemetry]
enabled = true
endpoint = "http://jaeger-staging:4318"
```

Run:
```bash
./must-gather-mcp-server --config staging-config.toml
```

### Production

See [AUTHENTICATION.md](AUTHENTICATION.md) for production deployment examples.

## Troubleshooting

### Must-Gather Not Found

```
Error: must-gather path does not exist: /path
```

**Solution:** Verify the path points to the extracted must-gather directory (not the .tar file).

### No Container Directory Found

The loader automatically detects the container directory (usually named `quay-io-okd-scos-content-sha256-...`). If it fails, check that the must-gather was properly extracted.

### Missing Tools

```
Registered 0 toolsets
```

**Solution:** Ensure toolset imports are present in `cmd/must-gather-mcp-server/cmd/root.go`.

### Build Failures

**Issue:** `go build` fails with dependency errors

**Solution:**
```bash
go mod tidy
go mod download
make build
```

### Test Failures

**Issue:** Tests fail with import errors

**Solution:**
```bash
go mod tidy
go test ./... -v
```

## Contributing

### Code Style

- Follow standard Go formatting (`go fmt`)
- Use meaningful variable names
- Add comments for exported functions
- Keep functions focused and small

### Pull Request Checklist

- [ ] Code is formatted with `make fmt`
- [ ] Tests pass with `make test`
- [ ] Linter passes with `make lint`
- [ ] Documentation is updated for new features
- [ ] Example usage provided for new tools
- [ ] No breaking changes to existing tools

### Testing Checklist

- [ ] Unit tests for new code
- [ ] Integration tests with MCP inspector
- [ ] Manual testing with Claude Desktop or Goose
- [ ] Performance testing for data-heavy operations
- [ ] Error case handling

## Future Enhancements

### Planned Features
1. API service status tools
2. Custom Resource Definitions (CRD) analysis
3. Network policy analysis
4. Resource quota and limit analysis
5. Image pull errors analysis
6. Certificate expiration checking
7. Must-gather collection metadata tools

### Performance Improvements
1. Streaming log retrieval
2. Log filtering with regex/grep
3. Event correlation with resources
4. Caching for frequently accessed data

### Observability
1. Prometheus metrics for auth events
2. Rate limiting middleware
3. Audit logging
4. Admin API for configuration

## Version History

Run `./must-gather-mcp-server --version` to see version information.

## License

Apache License 2.0
