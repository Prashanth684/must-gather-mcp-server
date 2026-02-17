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

### 🛠️ Tool Categories (62 Tools Across 5 Toolsets)

#### Cluster Toolset (28 tools)
**Version & Info (6 tools):**
- `cluster_version_get` - OpenShift version, update status, capabilities
- `cluster_info_get` - Infrastructure (platform, region, topology, network config)
- `cluster_operators_list` - All operators with Available/Progressing/Degraded status
- `cluster_operator_get` - Detailed operator conditions, versions, related objects
- `cluster_nodes_list` - Nodes with roles, status, kubelet version
- `cluster_node_get` - Detailed node info (capacity, conditions, taints)

**Machine Configuration (4 tools):**
- `machineconfig_list` - List all MachineConfigs (OS and systemd configuration)
- `machineconfig_get` - Get detailed MachineConfig including ignition configuration
- `machineconfigpool_status` - Check pool degradation and update status
- `machineconfignode_status` - Per-node config application status

**Storage (4 tools):**
- `storage_classes_list` - List StorageClasses with provisioners and defaults
- `csi_drivers_status` - CSI driver capabilities and status
- `volume_attachments_list` - Find stuck or failing volume mounts
- `persistent_volumes_status` - PV status (available, bound, failed)

**Security & RBAC (2 tools):**
- `security_scc_list` - SecurityContextConstraints with privilege analysis
- `rbac_clusterroles_list` - ClusterRoles with dangerous permission detection

**OLM/Operators (3 tools):**
- `olm_subscriptions_status` - Operator subscription and upgrade status
- `olm_catalogsources_status` - Catalog source connectivity and health
- `olm_installplans_status` - Pending operator installations

**Admission Control (2 tools):**
- `admission_webhooks_list` - Validating and mutating webhooks
- `admission_policies_list` - CEL-based admission policies

**Configuration (2 tools):**
- `cluster_config_list` - List all config.openshift.io resources
- `cluster_config_get` - Get detailed cluster configuration

**Ingress & Routing (4 tools):**
- `routes_list` - List OpenShift Routes with hosts, services, TLS config, and admission status
- `route_get` - Get detailed Route information including backend weights and TLS termination
- `ingress_list` - List Kubernetes Ingress resources with rules and backends
- `ingresscontroller_status` - Get IngressController (router) status, replicas, and conditions

#### Core Toolset (6 tools)
**Resources (3 tools):**
- `resources_get` - Get any Kubernetes resource by kind/name/namespace
- `resources_list` - List resources with label/field selectors
- `namespaces_list` - List all namespaces

**Events (3 tools):**
- `events_list` - Filter events by type/namespace/reason
- `events_timeline` - Chronological event sequence for incident analysis
- `events_by_resource` - All events for specific pods/nodes/deployments

#### Diagnostics Toolset (17 tools)
**Pod Logs (2 tools):**
- `pod_logs_get` - Container logs (current/previous) with tail support
- `pod_containers_list` - Discover containers with logs

**Node Diagnostics (4 tools):**
- `nodes_list` - Nodes with diagnostic data available
- `node_diagnostics_get` - Comprehensive node diagnostics (kubelet, sysinfo, CPU/IRQ, hardware)
- `node_kubelet_logs` - Kubelet logs (auto-decompressed from .gz)
- `node_kubelet_logs_grep` - Filter kubelet logs by string with optional case-insensitive search

**Extended Node Diagnostics (3 tools):**
- `node_hardware_info` - CPU topology, PCI devices, network interfaces
- `node_dmesg_errors` - Parse kernel logs for errors, OOM kills, hardware issues
- `node_kernel_info` - Kernel boot parameters and configuration

**Host Service Logs (3 tools):**
- `host_service_logs_list` - List systemd services (kubelet, crio, NetworkManager, etc.)
- `host_service_logs_get` - Get specific service logs with tail support
- `host_service_logs_grep` - Search across all host service logs

**Static Pods (1 tool):**
- `static_pod_termination_logs` - Control plane pod crash logs (kube-apiserver, etcd, etc.)

**ETCD (4 tools):**
- `etcd_health` - Cluster health, endpoint status, alarms
- `etcd_object_count` - Resource type object counts
- `etcd_members_list` - Member IDs, peer/client URLs
- `etcd_endpoint_status` - DB size, quota usage, raft state, leader info

#### Network Toolset (3 tools)
- `network_scale_get` - Network resource counts (services, pods, policies)
- `network_ovn_resources` - OVN Kubernetes component resource usage
- `network_connectivity_check` - Pod connectivity test results with failure analysis

#### Monitoring Toolset (8 tools)
**Prometheus Core Health:**
- `monitoring_prometheus_status` - Server status with TSDB statistics and runtime information
- `monitoring_prometheus_targets` - Scrape targets with health filtering
- `monitoring_prometheus_tsdb` - Detailed TSDB statistics with top metrics and label cardinality

**Alert & Rule Management:**
- `monitoring_alertmanager_status` - AlertManager cluster status and version info
- `monitoring_prometheus_rules` - Recording and alerting rules with health status
- `monitoring_prometheus_alerts` - Active alerts with severity filtering

**Configuration & Discovery:**
- `monitoring_prometheus_config_summary` - Configuration overview with scrape jobs and global settings
- `monitoring_servicemonitor_list` - ServiceMonitor CRD listing for scrape target discovery

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
  --must-gather-path string    Path to must-gather directory (required)
  --http                       Run in HTTP/SSE mode instead of STDIO
  --port string                HTTP server port (replaces deprecated --http-addr)
  --http-addr string           HTTP server address (deprecated, use --port)

  --config string              Path to configuration file
  --config-dir string          Path to drop-in configuration directory

  --log-level int              Log verbosity level (0-9)

  --require-oauth              Require OAuth authentication
  --oauth-audience string      OAuth audience for token validation
  --authorization-url string   OIDC authorization server URL

  --version                    Show version information
  -h, --help                   help for must-gather-mcp-server
```

For complete configuration options, see [Authentication Guide](docs/AUTHENTICATION.md).

## Example Queries

### Cluster Analysis
- "What version of OpenShift is this cluster running?"
- "Show me all degraded cluster operators"
- "List all master nodes and their status"
- "What platform is this cluster on and what region?"

### Machine Configuration
- "Show me all MachineConfigPools and their status"
- "Are any nodes degraded due to machine config issues?"
- "What MachineConfigs are applied to worker nodes?"

### Storage Troubleshooting
- "Which is the default StorageClass?"
- "Show me all failing volume attachments"
- "List all PersistentVolumes that are in Failed state"
- "What CSI drivers are available in the cluster?"

### Security & RBAC
- "Which SecurityContextConstraints allow privileged containers?"
- "Show me ClusterRoles with wildcard permissions"
- "List all users with access to the privileged SCC"

### Operator Lifecycle (OLM)
- "Are there any operator subscriptions in UpgradeFailed state?"
- "Show me pending InstallPlans that need approval"
- "Is the certified-operators catalog source healthy?"

### Events & Incidents
- "Show me all Warning events from the last hour"
- "What events happened to pod X?"
- "Timeline of events for namespace Y in the last 2 hours"
- "Find all events with reason 'BackOff' or 'Failed'"

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
- "Filter kubelet logs for node Z by 'error' string"
- "Search for 'OOM' in kubelet logs for all nodes"
- "List all nodes with diagnostic data"
- "Get comprehensive diagnostics for node A"

### Extended Node Diagnostics
- "Show me hardware information for node X (CPU, PCI devices)"
- "Parse dmesg for errors and OOM kills across all nodes"
- "What are the kernel boot parameters for node Y?"

### Host-Level Debugging
- "List all host systemd service logs"
- "Show me crio service logs for the last 100 lines"
- "Search for 'error' across all host service logs"

### Control Plane Debugging
- "Show me kube-apiserver termination logs for master-0"
- "Are there any control plane pod crashes?"

### Admission Control
- "List all validating and mutating webhooks"
- "What admission policies are configured?"

### Configuration
- "List all cluster configuration resources"
- "Show me the OAuth configuration"
- "What FeatureGates are enabled?"

### Ingress & Routing
- "List all Routes showing their admission status"
- "Show me details for route X in namespace Y"
- "What's the status of the default IngressController?"
- "List all Routes with TLS edge termination"
- "Are there any Kubernetes Ingress resources?"

### Monitoring & Observability
- "What's the Prometheus server status and TSDB statistics?"
- "Show me all failing Prometheus scrape targets"
- "List all critical alerts currently firing"
- "What are the top metrics by series count?"
- "Show me the AlertManager cluster status"
- "List all ServiceMonitors in the cluster"
- "What alerting rules are configured?"

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
│  │              62 MCP Tools (5 Toolsets)                   │   │
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
│   │   └── core/                      # Nodes, PVs
│   ├── namespaces/                    # Namespaced resources
│   │   └── {namespace}/
│   │       ├── core/                  # Pods, services, etc.
│   │       └── pods/                  # Pod logs
│   ├── nodes/                         # Node diagnostics
│   ├── etcd_info/                     # ETCD health and metrics
│   ├── network_logs/                  # Network scale and OVN metrics
│   ├── pod_network_connectivity_check/ # Connectivity test results
│   └── monitoring/                    # Prometheus and AlertManager data
│       ├── alertmanager/              # AlertManager status
│       ├── prometheus/                # Prometheus metrics and configuration
│       │   ├── prometheus-k8s-0/      # Replica 0 (targets, TSDB stats)
│       │   ├── prometheus-k8s-1/      # Replica 1 (targets, TSDB stats)
│       │   ├── rules.json             # Alerting and recording rules
│       │   └── status/                # Shared configuration and flags
│       └── servicemonitors/           # ServiceMonitor CRDs
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

- [Tools Reference](docs/TOOLS.md) - Complete reference for all 62 MCP tools
- [Authentication Guide](docs/AUTHENTICATION.md) - OAuth/OIDC setup and configuration
- [Development Guide](docs/DEVELOPMENT.md) - Building, testing, and contributing

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
