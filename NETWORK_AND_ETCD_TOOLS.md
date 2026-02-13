# Network and ETCD Tools Implementation

## Overview

Added comprehensive network and extended ETCD tools to provide deep insights into cluster networking and ETCD database status.

## New Network Toolset (3 tools)

### Data Sources

The network toolset reads from:
- **network_logs/cluster_scale** - Network resource counts
- **network_logs/ovn_kubernetes_top_pods** - OVN component resource usage
- **pod_network_connectivity_check/podnetworkconnectivitychecks.yaml** - Connectivity test results

### Tools

#### 1. network_scale_get
Get network scale information showing the size of the cluster's network configuration.

**Parameters**: None

**Returns**:
- Number of services
- Number of endpoints
- Number of pods
- Number of network policies
- Number of egress firewalls

**Example Output**:
```
Network Scale Information
================================================================================

services amount: 92
endpoints amount: 91
pods amount: 296
network policies amount: 48
egress firewalls amount: 0
```

**Use Cases**:
- Understanding cluster network complexity
- Planning capacity for network controllers
- Identifying network policy sprawl

#### 2. network_ovn_resources
Get OVN Kubernetes pod resource usage showing CPU and memory consumption of network components.

**Parameters**: None

**Returns**:
- Table of all OVN pods and containers with CPU/memory usage
- Summary by pod with total resource consumption

**Example Output**:
```
OVN Kubernetes Resource Usage
================================================================================

POD                                      NAME                          CPU(cores)   MEMORY(bytes)
--------------------------------------------------------------------------------
ovnkube-control-plane-5cbbfb9fb9-qgfwr   kube-rbac-proxy               1m           16Mi
ovnkube-control-plane-5cbbfb9fb9-qgfwr   ovnkube-cluster-manager       2m           163Mi
ovnkube-node-9pv6v                       kube-rbac-proxy-node          1m           13Mi
ovnkube-node-9pv6v                       nbdb                          2m           5Mi
ovnkube-node-9pv6v                       northd                        1m           10Mi
ovnkube-node-9pv6v                       ovn-controller                1m           17Mi
ovnkube-node-9pv6v                       ovnkube-controller            9m           287Mi
ovnkube-node-9pv6v                       sbdb                          2m           12Mi
...

================================================================================
Summary by Pod
================================================================================

POD                                           CONTAINERS  TOTAL CPU  TOTAL MEMORY
--------------------------------------------------------------------------------
ovnkube-control-plane-5cbbfb9fb9-qgfwr                2         3m         179Mi
ovnkube-node-9pv6v                                    8        17m         359Mi
ovnkube-node-fcv6v                                    8        16m         360Mi
...
```

**Use Cases**:
- Identifying resource-intensive network components
- Troubleshooting OVN performance issues
- Capacity planning for network infrastructure

#### 3. network_connectivity_check
Get pod network connectivity check results showing reachability between cluster components.

**Parameters**:
- `status` (optional) - Filter by status: all, failing, degraded (default: all)

**Returns**:
- Total connectivity checks
- Count of failing and degraded checks
- Detailed check results with source, target, and failure information

**Example Output**:
```
Pod Network Connectivity Checks
================================================================================

Total Checks: 156
Failing: 12
Degraded: 0

Showing 12 checks:
--------------------------------------------------------------------------------

1. ✗ FAILING
   Name: network-check-source-ip-10-0-124-129-to-kubernetes-apiserver-endpoint-ip-10-0-122-129
   Source: network-check-source-86f6944d5-g6trv
   Target: 10.0.122.129:17697
   Message: kubernetes-apiserver-endpoint-ip-10-0-122-129: failed to establish a TCP connection to 10.0.122.129:17697: dial tcp 10.0.122.129:17697: i/o timeout
   Recent Failures (3 of 142):
     - 2026-01-12T16:56:53Z (latency: 10.000117127s)
     - 2026-01-12T16:55:53Z (latency: 10.000617146s)
     - 2026-01-12T16:54:53Z (latency: 10.001026427s)

2. ✓ Reachable
   Name: network-check-source-ip-10-0-124-129-to-kubernetes-default-service-cluster
   Source: network-check-source-86f6944d5-g6trv
   Target: 172.30.0.1:443

...
```

**Use Cases**:
- Diagnosing network connectivity issues between components
- Identifying network partition or firewall problems
- Validating cluster network health after changes

**Filter Examples**:
```json
{
  "name": "network_connectivity_check",
  "arguments": {
    "status": "failing"
  }
}
```

## Extended ETCD Tools (2 new tools)

Added to the existing diagnostics toolset to complement `etcd_health` and `etcd_object_count`.

### Data Sources

- **etcd_info/member_list.json** - ETCD cluster member information
- **etcd_info/endpoint_status.json** - Detailed endpoint status including DB sizes and raft state

### Tools

#### 4. etcd_members_list
Get ETCD cluster member information including IDs, peer URLs, and client URLs.

**Parameters**: None

**Returns**:
- Cluster ID
- Current member ID
- Raft term
- List of all members with names, IDs, and URLs

**Example Output**:
```
ETCD Cluster Members
================================================================================

Cluster ID: 5818742644909804811
Current Member ID: 8695595020199737803
Raft Term: 9
Total Members: 3

--------------------------------------------------------------------------------

Member 1:
  Name: ip-10-0-97-146.us-east-2.compute.internal
  ID: 3279992772859492223
  Peer URLs: https://10.0.97.146:2380
  Client URLs: https://10.0.97.146:2379

Member 2:
  Name: ip-10-0-122-129.us-east-2.compute.internal
  ID: 8695595020199737803
  Peer URLs: https://10.0.122.129:2380
  Client URLs: https://10.0.122.129:2379

Member 3:
  Name: ip-10-0-49-48.us-east-2.compute.internal
  ID: 9546278856345108396
  Peer URLs: https://10.0.49.48:2380
  Client URLs: https://10.0.49.48:2379
```

**Use Cases**:
- Verifying ETCD cluster membership
- Identifying member IDs for troubleshooting
- Confirming peer and client URL configuration

#### 5. etcd_endpoint_status
Get detailed ETCD endpoint status including DB size, leader info, raft state, and quota usage.

**Parameters**: None

**Returns**:
- Per-endpoint status with DB sizes, leader information, and raft state
- Warnings for high DB usage (>80%)
- Warnings for raft lag
- Summary with averages and leader identification

**Example Output**:
```
ETCD Endpoint Status
================================================================================

Total Endpoints: 3

Endpoint 1: https://10.0.97.146:2379
--------------------------------------------------------------------------------
  Member ID: 3279992772859492223
  Version: 3.6.5
  Storage Version: 3.6.0

  Database:
    Size: 56.11 MB
    In Use: 53.97 MB
    Quota: 8.00 GB
    Usage: 0.66%

  Raft:
    Term: 9
    Index: 88570
    Applied Index: 88570
    Revision: 78133

Endpoint 2: https://10.0.122.129:2379 (LEADER)
--------------------------------------------------------------------------------
  Member ID: 8695595020199737803
  Version: 3.6.5
  Storage Version: 3.6.0

  Database:
    Size: 56.42 MB
    In Use: 53.98 MB
    Quota: 8.00 GB
    Usage: 0.66%

  Raft:
    Term: 9
    Index: 88571
    Applied Index: 88571
    Revision: 78134

Endpoint 3: https://10.0.49.48:2379
--------------------------------------------------------------------------------
  Member ID: 9546278856345108396
  Version: 3.6.5
  Storage Version: 3.6.0

  Database:
    Size: 55.65 MB
    In Use: 54.02 MB
    Quota: 8.00 GB
    Usage: 0.66%

  Raft:
    Term: 9
    Index: 88571
    Applied Index: 88571
    Revision: 78134

================================================================================
Summary
================================================================================

Average DB Size: 56.06 MB
Average DB In Use: 53.99 MB
Leader ID: 8695595020199737803
```

**Warnings**:
- Database usage >80%: `⚠ WARNING: Database usage is above 80%`
- Raft lag: `⚠ Lag: N (index - applied index)`

**Use Cases**:
- Monitoring ETCD database space usage
- Identifying database defragmentation needs
- Detecting raft consensus issues
- Verifying leader election
- Capacity planning for ETCD storage

## Tool Count Summary

**Total Tools**: 30

### By Toolset:
- **Cluster**: 6 tools (version, info, operators, nodes)
- **Core**: 3 tools (resources get/list, namespaces)
- **Diagnostics**: 9 tools (pod logs, node diagnostics, ETCD health + extended)
  - etcd_health
  - etcd_object_count
  - etcd_members_list
  - etcd_endpoint_status
  - pod_logs_get
  - pod_containers_list
  - nodes_list
  - node_diagnostics_get
  - node_kubelet_logs
- **Network**: 3 tools
  - network_scale_get
  - network_ovn_resources
  - network_connectivity_check
- **Monitoring**: 8 tools (Prometheus and AlertManager observability)
  - monitoring_prometheus_status
  - monitoring_prometheus_targets
  - monitoring_prometheus_tsdb
  - monitoring_alertmanager_status
  - monitoring_prometheus_rules
  - monitoring_prometheus_alerts
  - monitoring_prometheus_config_summary
  - monitoring_servicemonitor_list

## Build Status

✅ **Build Successful**
```
$ make build
# Build successful

$ ./must-gather-mcp-server --must-gather-path /path/to/must-gather
Loading must-gather from: /path/to/must-gather
Loaded 11100 resources from 69 namespaces
Building resource index...
Index built with 10655 resources
Registered 4 toolsets
Registering 6 tools from toolset: cluster
Registering 3 tools from toolset: core
Registering 9 tools from toolset: diagnostics
Registering 3 tools from toolset: network
Starting must-gather MCP server...
```

## AI Assistant Integration

These tools enable AI assistants to perform advanced network and ETCD troubleshooting:

**Example Queries**:

**Network**:
- "How many network policies are configured in the cluster?"
- "Show me OVN resource usage - are any components using excessive memory?"
- "Are there any network connectivity failures?"
- "Show me all failing network connectivity checks"

**ETCD**:
- "List all ETCD cluster members"
- "What's the ETCD database size and usage percentage?"
- "Is there any raft lag in the ETCD cluster?"
- "Who is the current ETCD leader?"
- "Are we approaching the ETCD database quota?"

**Combined Analysis**:
- "Check if ETCD is healthy and show me any network connectivity issues"
- "What's the network scale and is ETCD handling it well?"
- "Show me ETCD status and OVN resource consumption"

## Use Cases

### Network Troubleshooting
1. **Connectivity Issues**: Use `network_connectivity_check` to identify failed connections between cluster components
2. **Performance Problems**: Use `network_ovn_resources` to find resource-constrained network pods
3. **Scale Assessment**: Use `network_scale_get` to understand network complexity

### ETCD Monitoring
1. **Capacity Planning**: Use `etcd_endpoint_status` to monitor DB size and quota usage
2. **Cluster Health**: Use `etcd_members_list` and `etcd_endpoint_status` together to verify cluster membership and leader status
3. **Performance Diagnosis**: Check raft lag and DB fragmentation using `etcd_endpoint_status`

### Comprehensive Cluster Analysis
Combine network, ETCD, and cluster tools for full stack troubleshooting:
```
1. Check cluster version and operators status
2. Verify ETCD health and database status
3. Check network connectivity between components
4. Review OVN resource usage
5. Analyze node and pod diagnostics
```

## Future Enhancements

Potential additions:
1. **network_policy_analyze** - Analyze network policies for conflicts or gaps
2. **ovn_database_dump** - Extract and analyze OVN database contents (from ovnk_database_store.tar.gz)
3. **etcd_defrag_recommend** - Recommend defragmentation based on DB usage
4. **network_flow_analysis** - Analyze network flows and bandwidth usage
5. **connectivity_matrix** - Generate a full connectivity matrix between all checked endpoints
