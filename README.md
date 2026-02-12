# Must-Gather MCP Server

A Model Context Protocol (MCP) server that provides AI assistants with comprehensive access to OpenShift must-gather data for troubleshooting and diagnostics.

## Overview

The must-gather MCP server enables AI assistants to deeply analyze OpenShift clusters through must-gather archives, providing:
- Complete cluster configuration and status
- Network connectivity and performance data
- ETCD database health and metrics
- Pod and node diagnostics with logs
- Operator status and version tracking

## Features

### 🚀 Performance
- **Fast Resource Access**: In-memory indexing for ~11,000 resources
- **Quick Startup**: 5-10 seconds to load and index
- **Fast Queries**: <50ms for indexed resource lookups
- **On-Demand Logs**: Logs loaded only when requested

### 🛠️ Tool Categories (21 Tools Across 4 Toolsets)

#### Cluster Toolset (6 tools)
- `cluster_version_get` - OpenShift version, update status, capabilities
- `cluster_info_get` - Infrastructure (platform, region, topology, network config)
- `cluster_operators_list` - All operators with Available/Progressing/Degraded status
- `cluster_operator_get` - Detailed operator conditions, versions, related objects
- `cluster_nodes_list` - Nodes with roles, status, kubelet version
- `cluster_node_get` - Detailed node info (capacity, conditions, taints)

#### Core Toolset (3 tools)
- `resources_get` - Get any Kubernetes resource by kind/name/namespace
- `resources_list` - List resources with label/field selectors
- `namespaces_list` - List all namespaces

#### Diagnostics Toolset (9 tools)
**Pod Logs:**
- `pod_logs_get` - Container logs (current/previous) with tail support
- `pod_containers_list` - Discover containers with logs

**Node Diagnostics:**
- `nodes_list` - Nodes with diagnostic data available
- `node_diagnostics_get` - Comprehensive node diagnostics (kubelet, sysinfo, CPU/IRQ, hardware)
- `node_kubelet_logs` - Kubelet logs (auto-decompressed from .gz)

**ETCD:**
- `etcd_health` - Cluster health, endpoint status, alarms
- `etcd_object_count` - Resource type object counts
- `etcd_members_list` - Member IDs, peer/client URLs
- `etcd_endpoint_status` - DB size, quota usage, raft state, leader info

#### Network Toolset (3 tools)
- `network_scale_get` - Network resource counts (services, pods, policies)
- `network_ovn_resources` - OVN Kubernetes component resource usage
- `network_connectivity_check` - Pod connectivity test results with failure analysis

## Installation

### From Source
```bash
git clone <repository>
cd must-gather-mcp-server
make build
```

### Via npm (when published)
```bash
npx must-gather-mcp-server@latest --must-gather-path /path/to/must-gather
```

### Via Python (when published)
```bash
uvx must-gather-mcp-server --must-gather-path /path/to/must-gather
```

## Usage

### STDIO Mode (Default)

For use with Claude Desktop, MCP Inspector, and other MCP clients that use STDIO:

```bash
./must-gather-mcp-server --must-gather-path /path/to/must-gather
```

#### With Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

```json
{
  "mcpServers": {
    "must-gather": {
      "command": "/path/to/must-gather-mcp-server",
      "args": ["--must-gather-path", "/path/to/your/must-gather"]
    }
  }
}
```

#### With MCP Inspector

```bash
npx @modelcontextprotocol/inspector@latest ./must-gather-mcp-server \
  --must-gather-path /path/to/must-gather
```

### HTTP/SSE Mode

For use with agents like Goose or other HTTP-based MCP clients:

```bash
./must-gather-mcp-server \
  --must-gather-path /path/to/must-gather \
  --http \
  --http-addr localhost:8080
```

The server will start on `http://localhost:8080` with SSE endpoint at `http://localhost:8080/sse`.

#### With Goose

Configure Goose to connect to the HTTP endpoint:

```yaml
# goose config
mcp_servers:
  must-gather:
    url: http://localhost:8080/sse
```

Then start Goose and it will connect to the MCP server.

## Command Line Options

```
Flags:
  --must-gather-path string   Path to must-gather directory (required)
  --http                      Run in HTTP/SSE mode instead of STDIO
  --http-addr string          HTTP server address (default "localhost:8080")
  --version                   Show version information
  -h, --help                  help for must-gather-mcp-server
```

## Example Queries

### Cluster Analysis
- "What version of OpenShift is this cluster running?"
- "Show me all degraded cluster operators"
- "List all master nodes and their status"
- "What platform is this cluster on and what region?"

### ETCD Monitoring
- "Check ETCD cluster health"
- "What's the ETCD database size and quota usage?"
- "Is there any raft lag in the ETCD cluster?"
- "List all ETCD members"

### Network Troubleshooting
- "Show me all failing network connectivity checks"
- "What's the network scale of this cluster?"
- "Which OVN components are using the most resources?"

### Pod & Node Diagnostics
- "Get logs for pod X in namespace Y"
- "Show me kubelet logs for node Z"
- "List all nodes with diagnostic data"
- "Get comprehensive diagnostics for node A"

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
```

## Architecture

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
│   │   └── core/                      # Nodes, PVs
│   ├── namespaces/                    # Namespaced resources
│   │   └── {namespace}/
│   │       ├── core/                  # Pods, services, etc.
│   │       └── pods/                  # Pod logs
│   ├── nodes/                         # Node diagnostics
│   ├── etcd_info/                     # ETCD health and metrics
│   ├── network_logs/                  # Network scale and OVN metrics
│   └── pod_network_connectivity_check/ # Connectivity test results
```

### Tool Categories

**Indexed Resources** (fast queries):
- Cluster resources (operators, version, infrastructure, nodes)
- Core resources (pods, services, configmaps, etc.)
- All Kubernetes/OpenShift API resources

**On-Demand Data** (read from files):
- Pod container logs
- Node diagnostics (kubelet logs, sysinfo, hardware info)
- ETCD detailed status
- Network connectivity checks

## Documentation

- [CLUSTER_TOOLSET.md](CLUSTER_TOOLSET.md) - Cluster-level tools documentation
- [DIAGNOSTICS_MODULE.md](DIAGNOSTICS_MODULE.md) - Diagnostics tools documentation
- [NETWORK_AND_ETCD_TOOLS.md](NETWORK_AND_ETCD_TOOLS.md) - Network and ETCD tools documentation
- [LOADER_ANALYSIS.md](LOADER_ANALYSIS.md) - Loader implementation details
- [TESTING_GUIDE.md](TESTING_GUIDE.md) - Testing instructions

## Performance

### Startup
- Load time: ~5-10 seconds for 11,000 resources
- Index time: ~2-3 seconds
- Memory usage: ~100-200MB (depending on cluster size)

### Query Performance
- Indexed queries: <50ms
- Log retrieval: <500ms (most cases)
- Kubelet log decompression: <1s (371K .gz file)

## Troubleshooting

### Must-Gather Not Found
```
Error: must-gather path does not exist: /path
```
Solution: Verify the path points to the extracted must-gather directory (not the .tar file).

### No Container Directory Found
The loader automatically detects the container directory (usually named `quay-io-okd-scos-content-sha256-...`). If it fails, check that the must-gather was properly extracted.

### Missing Tools
```
Registered 0 toolsets
```
Solution: Ensure toolset imports are present in `cmd/must-gather-mcp-server/cmd/root.go`.

## Contributing

Contributions are welcome! Please ensure:
- Code is formatted with `make fmt`
- Tests pass with `make test`
- Linter passes with `make lint`
- Documentation is updated for new features

## License

Apache License 2.0

## Version

Run `./must-gather-mcp-server --version` to see version information.
