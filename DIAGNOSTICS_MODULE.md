# Diagnostics Module Implementation

## Overview

Successfully implemented a comprehensive diagnostics module for the must-gather MCP server that provides access to pod logs and node diagnostics data.

## Implementation Details

### API Extensions (pkg/api/logs.go)

Added new types for log retrieval:

```go
type LogType string
const (
    LogTypeCurrent          LogType = "current"
    LogTypePrevious         LogType = "previous"
    LogTypePreviousInsecure LogType = "previous.insecure"
)

type PodLogOptions struct {
    Namespace string
    Pod       string
    Container string
    LogType   LogType
    TailLines int
    Follow    bool // Not applicable for must-gather
}

type NodeDiagnostics struct {
    NodeName      string
    KubeletLog    string // Decompressed kubelet log
    SysInfo       string
    CPUAffinities string
    IRQAffinities string
    PodsInfo      string
    PodResources  string
    Lscpu         string
    Lspci         string
    Dmesg         string
    ProcCmdline   string
}
```

### Provider Extensions (pkg/api/mustgather.go)

Extended MustGatherProvider interface with 4 new methods:

```go
type MustGatherProvider interface {
    // ... existing methods ...

    // Pod Logs
    GetPodLog(opts PodLogOptions) (string, error)
    ListPodContainers(namespace, pod string) ([]string, error)

    // Node Diagnostics
    GetNodeDiagnostics(nodeName string) (*NodeDiagnostics, error)
    ListNodes() ([]string, error)
}
```

### Log Retrieval Implementation (pkg/mustgather/logs.go)

Implemented comprehensive log access:

**Pod Logs:**
- `GetPodLog()` - Retrieves container logs from namespaces/{ns}/pods/{pod}/{container}/{container}/logs/
- Supports current, previous, and previous.insecure logs
- Optional tail functionality to limit output
- Path construction handles container name appearing twice in directory structure

**Node Diagnostics:**
- `GetNodeDiagnostics()` - Retrieves all node diagnostic data:
  - Kubelet logs (gzipped) - automatically decompressed
  - System info (sysinfo.log)
  - CPU affinities (cpu_affinities.json)
  - IRQ affinities (irq_affinities.json)
  - Pods info (pods_info.json)
  - Pod resources (podresources.json)
  - CPU info (lscpu)
  - PCI devices (lspci)
  - Kernel messages (dmesg)
  - Boot parameters (proc_cmdline)

**Helper Functions:**
- `TailLines()` - Extracts last N lines from content (exported for reuse)
- `readGzipFile()` - Decompresses .gz files (kubelet logs)
- `readTextFile()` - Simple file reading
- `ListPodContainers()` - Discovers containers with logs
- `ListNodes()` - Lists all nodes with diagnostic data

### Diagnostics Toolset (pkg/toolsets/diagnostics/)

Created a complete diagnostics toolset with 7 MCP tools:

#### Pod Logs Tools (pod_logs.go)

1. **pod_logs_get**
   - Description: Get logs for a specific pod container
   - Parameters:
     - `namespace` (required) - Pod namespace
     - `pod` (required) - Pod name
     - `container` (optional) - Container name (defaults to first container)
     - `previous` (boolean) - Get previous logs from crash/restart
     - `tail` (integer) - Number of lines from end (0 for all)

2. **pod_containers_list**
   - Description: List all containers for a pod that have logs available
   - Parameters:
     - `namespace` (required) - Pod namespace
     - `pod` (required) - Pod name

#### Node Diagnostics Tools (nodes.go)

3. **nodes_list**
   - Description: List all nodes with diagnostic data available
   - No parameters required

4. **node_diagnostics_get**
   - Description: Get comprehensive diagnostic information for a node
   - Parameters:
     - `node` (required) - Node name
     - `include` (optional) - Comma-separated list: kubelet,sysinfo,cpu,irq,pods,lscpu,lspci,dmesg,cmdline (default: all)
     - `kubeletTail` (integer) - Number of lines from kubelet log (default: 100, 0 for all)

5. **node_kubelet_logs**
   - Description: Get kubelet logs for a specific node
   - Parameters:
     - `node` (required) - Node name
     - `tail` (integer) - Number of lines from end (0 for all)

#### ETCD Tools (etcd.go)

6. **etcd_health**
   - Description: Get ETCD cluster health status
   - Shows endpoint health and alarms
   - No parameters required

7. **etcd_object_count**
   - Description: Get ETCD object counts by resource type
   - Parameters:
     - `sortBy` (optional) - Sort by 'count' or 'name' (default: count)
     - `top` (integer) - Show only top N resource types (0 for all)

### Build Fix

Fixed undefined `tailLines` error:
- Exported `tailLines` as `TailLines` from pkg/mustgather/logs.go
- Added import in pkg/toolsets/diagnostics/nodes.go
- Updated all references to use `mustgather.TailLines()`

## Testing

Successfully built and verified:
```bash
$ make build
go fmt ./...
go mod tidy
go build ... -o _output/bin/must-gather-mcp-server ./cmd/must-gather-mcp-server

$ _output/bin/must-gather-mcp-server --must-gather-path /home/psundara/Downloads/must-gather-Prashanth-Testcase-failure
Loading must-gather from: /home/psundara/Downloads/must-gather-Prashanth-Testcase-failure
Loaded 11100 resources from 69 namespaces
Building resource index...
Index built with 10655 resources
Registered 2 toolsets
Registering 3 tools from toolset: core
Registering 7 tools from toolset: diagnostics
Starting must-gather MCP server...
```

## Must-Gather Structure

The implementation correctly handles the must-gather directory structure:

```
must-gather-Prashanth-Testcase-failure/
└── quay-io-okd-scos-content-sha256-.../
    ├── namespaces/
    │   └── {namespace}/
    │       ├── pods/
    │       │   └── {pod-name}/
    │       │       └── {container-name}/
    │       │           └── {container-name}/
    │       │               └── logs/
    │       │                   ├── current.log
    │       │                   ├── previous.log
    │       │                   └── previous.insecure.log
    │       └── {api-group}/
    │           └── {resource-type}.yaml
    ├── nodes/
    │   └── {node-name}/
    │       ├── {node-name}_logs_kubelet.gz
    │       ├── sysinfo.log
    │       ├── cpu_affinities.json
    │       ├── irq_affinities.json
    │       ├── pods_info.json
    │       ├── podresources.json
    │       ├── lscpu
    │       ├── lspci
    │       ├── dmesg
    │       └── proc_cmdline
    └── etcd_info/
        ├── health.json
        └── object-count.json
```

## Key Features

1. **Automatic Decompression**: Kubelet logs are stored as .gz files and automatically decompressed
2. **Flexible Filtering**: Node diagnostics can include/exclude specific diagnostic types
3. **Tail Support**: Both pod and node logs support limiting output to last N lines
4. **Container Discovery**: Automatically discovers containers with logs available
5. **Previous Logs**: Supports retrieving logs from previous container crashes/restarts
6. **Comprehensive ETCD Info**: Provides both health status and resource counts

## Usage Example

Using with Claude Desktop or any MCP client:

```json
{
  "mcpServers": {
    "must-gather": {
      "command": "/path/to/must-gather-mcp-server",
      "args": ["--must-gather-path", "/path/to/must-gather-directory"]
    }
  }
}
```

AI assistants can then use tools like:
- "Get logs for pod openshift-etcd/etcd-control-plane in the etcd container"
- "Show me kubelet logs for node worker-0"
- "List all nodes with diagnostic data"
- "Check ETCD cluster health"
- "Show top 10 ETCD object types by count"

## Next Steps

Potential enhancements:
1. Add grep/filtering capabilities for logs
2. Implement log streaming simulation (replay logs chronologically)
3. Add correlation between pod logs and events
4. Implement log analysis tools (error detection, patterns)
5. Add node comparison tools
6. Implement resource usage analysis from node diagnostics
