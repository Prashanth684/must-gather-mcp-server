# Testing Guide for Diagnostics Module

## Build Status

✅ **Build Successful**
```bash
$ make build
go fmt ./...
go mod tidy
go build ... -o _output/bin/must-gather-mcp-server ./cmd/must-gather-mcp-server
```

✅ **Server Startup**
```bash
$ _output/bin/must-gather-mcp-server --must-gather-path /path/to/must-gather
Loading must-gather from: /path/to/must-gather
Loaded 11100 resources from 69 namespaces
Building resource index...
Index built with 10655 resources
Registered 2 toolsets
Registering 3 tools from toolset: core
Registering 7 tools from toolset: diagnostics
Starting must-gather MCP server...
```

## Available Tools

### Core Toolset (3 tools)
1. `resources_get` - Get a specific Kubernetes resource
2. `resources_list` - List Kubernetes resources with filtering
3. `namespaces_list` - List all namespaces

### Diagnostics Toolset (7 tools)
1. `pod_logs_get` - Get pod container logs
2. `pod_containers_list` - List containers for a pod
3. `nodes_list` - List all nodes with diagnostic data
4. `node_diagnostics_get` - Get comprehensive node diagnostics
5. `node_kubelet_logs` - Get kubelet logs for a node
6. `etcd_health` - Get ETCD cluster health status
7. `etcd_object_count` - Get ETCD object counts

## Test Data Available

### Pod Logs
Sample pod with logs:
- **Namespace**: openshift-apiserver
- **Pod**: apiserver-9f5b9f9d4-5djkq
- **Containers**:
  - openshift-apiserver (246K current logs)
  - fix-audit-permissions
  - openshift-apiserver-check-endpoints

### Node Diagnostics
6 nodes with full diagnostic data:
- ip-10-0-122-129.us-east-2.compute.internal
- ip-10-0-124-129.us-east-2.compute.internal
- ip-10-0-49-48.us-east-2.compute.internal
- ip-10-0-6-220.us-east-2.compute.internal
- ip-10-0-67-249.us-east-2.compute.internal
- ip-10-0-97-146.us-east-2.compute.internal

Each node has:
- Kubelet logs (compressed .gz, 371K)
- System info (396K)
- CPU affinities (129K JSON)
- IRQ affinities (644 bytes JSON)
- Pod info (25K JSON)
- Pod resources (9.5K JSON)
- CPU info (lscpu, 210 bytes)
- PCI devices (lspci, 2.3K)
- Kernel boot parameters (proc_cmdline, 461 bytes)
- Kernel messages (dmesg, if available)

## Testing with MCP Inspector

Install and run the MCP inspector:

```bash
npx @modelcontextprotocol/inspector _output/bin/must-gather-mcp-server \
  --must-gather-path /path/to/must-gather
```

## Sample Tool Calls

### 1. List Nodes
```json
{
  "name": "nodes_list",
  "arguments": {}
}
```

Expected output:
```
Found 6 nodes with diagnostic data:

1. ip-10-0-122-129.us-east-2.compute.internal
2. ip-10-0-124-129.us-east-2.compute.internal
3. ip-10-0-49-48.us-east-2.compute.internal
4. ip-10-0-6-220.us-east-2.compute.internal
5. ip-10-0-67-249.us-east-2.compute.internal
6. ip-10-0-97-146.us-east-2.compute.internal
```

### 2. Get Node Diagnostics
```json
{
  "name": "node_diagnostics_get",
  "arguments": {
    "node": "ip-10-0-122-129.us-east-2.compute.internal",
    "include": "kubelet,sysinfo",
    "kubeletTail": 50
  }
}
```

Expected: Kubelet logs (last 50 lines) + system info

### 3. Get Kubelet Logs Only
```json
{
  "name": "node_kubelet_logs",
  "arguments": {
    "node": "ip-10-0-122-129.us-east-2.compute.internal",
    "tail": 100
  }
}
```

Expected: Last 100 lines of decompressed kubelet logs

### 4. List Pod Containers
```json
{
  "name": "pod_containers_list",
  "arguments": {
    "namespace": "openshift-apiserver",
    "pod": "apiserver-9f5b9f9d4-5djkq"
  }
}
```

Expected output:
```
Containers for pod openshift-apiserver/apiserver-9f5b9f9d4-5djkq:

1. fix-audit-permissions
2. openshift-apiserver
3. openshift-apiserver-check-endpoints
```

### 5. Get Pod Logs
```json
{
  "name": "pod_logs_get",
  "arguments": {
    "namespace": "openshift-apiserver",
    "pod": "apiserver-9f5b9f9d4-5djkq",
    "container": "openshift-apiserver",
    "tail": 50
  }
}
```

Expected: Last 50 lines of current logs for the openshift-apiserver container

### 6. Get Previous Logs
```json
{
  "name": "pod_logs_get",
  "arguments": {
    "namespace": "openshift-apiserver",
    "pod": "apiserver-9f5b9f9d4-5djkq",
    "container": "openshift-apiserver",
    "previous": true
  }
}
```

Expected: Previous logs (from last restart/crash)

### 7. Check ETCD Health
```json
{
  "name": "etcd_health",
  "arguments": {}
}
```

Expected: ETCD cluster health status with endpoints and alarms

### 8. Get ETCD Object Counts
```json
{
  "name": "etcd_object_count",
  "arguments": {
    "sortBy": "count",
    "top": 10
  }
}
```

Expected: Top 10 resource types by object count

## Integration with Claude Desktop

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

Then ask Claude:
- "List all nodes in the cluster"
- "Show me kubelet logs for node ip-10-0-122-129"
- "Get logs for pod apiserver-9f5b9f9d4-5djkq in openshift-apiserver namespace"
- "What's the ETCD health status?"
- "Show me the top 20 resource types in ETCD"

## Verification Checklist

- [x] Build succeeds without errors
- [x] Server starts and loads must-gather data
- [x] Core toolset registered (3 tools)
- [x] Diagnostics toolset registered (7 tools)
- [x] Pod logs directory structure detected
- [x] Node diagnostics directory structure detected
- [x] Kubelet logs (.gz) files present
- [ ] Test with MCP inspector
- [ ] Verify pod log retrieval
- [ ] Verify node diagnostics retrieval
- [ ] Verify kubelet log decompression
- [ ] Test with Claude Desktop

## Next Steps for Testing

1. **Quick Test**: Run with mcp-inspector and try the sample tool calls above
2. **Integration Test**: Add to Claude Desktop and test natural language queries
3. **Performance Test**: Check response times for large logs
4. **Edge Cases**: Test with missing logs, empty files, non-existent pods/nodes

## Known Limitations

1. **No streaming**: Logs are returned in full (with optional tail)
2. **Memory usage**: Large logs loaded into memory
3. **No filtering**: No grep/regex support yet (Phase 4 feature)
4. **No correlation**: Logs not correlated with events/resources yet

## Performance Expectations

- **Startup time**: ~5-10 seconds (loading 11,100 resources)
- **Query time**: <50ms for indexed resource queries
- **Log retrieval**: <500ms for most logs
- **Kubelet decompression**: <1s for 371K .gz file
