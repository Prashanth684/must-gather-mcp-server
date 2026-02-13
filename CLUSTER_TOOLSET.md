# Cluster Toolset Implementation

## Overview

Implemented a comprehensive cluster toolset that provides access to cluster-level configuration and status information from must-gather cluster-scoped-resources directory.

## Data Sources

The cluster toolset reads from `cluster-scoped-resources/` directory which contains:
- **config.openshift.io/** - Cluster operators, version, infrastructure, network, authentication, etc.
- **core/** - Nodes, persistent volumes
- **operator.openshift.io/** - Operator configurations
- **machineconfiguration.openshift.io/** - Machine configurations
- **rbac.authorization.k8s.io/** - RBAC resources
- And many other cluster-wide API groups

## Implemented Tools (6 total)

### 1. Cluster Version Tools

#### cluster_version_get
Get OpenShift cluster version information

**Parameters**: None

**Returns**:
- Cluster ID
- Current version
- Image reference
- Status conditions (Available, Progressing, Failing, etc.)
- Enabled capabilities
- Version history (most recent 3 versions)

**Example Output**:
```
OpenShift Cluster Version
================================================================================

Cluster ID: 684f4c11-e778-4e90-a562-6d9bf21e35e5
Current Version: 4.21.0-okd-scos.ec.18
Image: registry.build05.ci.openshift.org/ci-op-...

Status Conditions:
--------------------------------------------------------------------------------
✓ Available: True
  Message: Done applying 4.21.0-okd-scos.ec.18

✗ Failing: False

✗ Progressing: False
  Reason: ClusterVersionUpgradeable
  Message: Cluster version is 4.21.0-okd-scos.ec.18

✓ Upgradeable: True

Enabled Capabilities (18):
  - Build
  - CSISnapshot
  - CloudControllerManager
  - CloudCredential
  ...
```

### 2. Cluster Information Tools

#### cluster_info_get
Get cluster infrastructure and network configuration

**Parameters**: None

**Returns**:
- Platform type (AWS, Azure, GCP, Bare Metal, etc.)
- Region/Location
- Infrastructure name
- Control plane topology (HighlyAvailable vs SingleNode)
- Infrastructure topology
- CPU partitioning status
- API server URLs
- Cluster network configuration
- Service network configuration
- Network type (OVNKubernetes, OpenShiftSDN, etc.)

**Example Output**:
```
OpenShift Cluster Information
================================================================================

## Infrastructure

Platform: AWS
Infrastructure Name: ci-op-d6hlpfk5-62c02-x2f57
Control Plane Topology: HighlyAvailable
Infrastructure Topology: HighlyAvailable
CPU Partitioning: None

Platform Details (AWS):
  Region: us-east-2

API Server URL: https://api.ci-op-d6hlpfk5-62c02.XXXXX:6443
API Server Internal URI: https://api-int.ci-op-d6hlpfk5-62c02.XXXXX:6443

## Network Configuration

Cluster Networks:
  1. CIDR: 10.128.0.0/14, Host Prefix: 23

Service Networks:
  1. 172.30.0.0/16

Network Type: OVNKubernetes
```

### 3. Cluster Operator Tools

#### cluster_operators_list
List all cluster operators with status

**Parameters**:
- `status` (optional) - Filter by status: all, degraded, progressing, unavailable (default: all)

**Returns**:
- List of all cluster operators
- Status for each: Available, Progressing, Degraded

**Example Output**:
```
OpenShift Cluster Operators
================================================================================

Total Operators: 35

NAME                                AVAILABLE    PROGRESSING  DEGRADED
--------------------------------------------------------------------------------
authentication                      ✓ True       ✗ False      ✗ False
baremetal                           ✓ True       ✗ False      ✗ False
cloud-controller-manager            ✓ True       ✗ False      ✗ False
cloud-credential                    ✓ True       ✗ False      ✗ False
cluster-autoscaler                  ✓ True       ✗ False      ✗ False
config-operator                     ✓ True       ✗ False      ✗ False
console                             ✓ True       ✗ False      ✗ False
dns                                 ✓ True       ✗ False      ✗ False
etcd                                ✓ True       ✗ False      ✗ False
image-registry                      ✓ True       ✗ False      ✗ False
ingress                             ✓ True       ✗ False      ✗ False
kube-apiserver                      ✓ True       ✗ False      ✗ False
kube-controller-manager             ✓ True       ✗ False      ✗ False
kube-scheduler                      ✓ True       ✗ False      ✗ False
...
```

**With Status Filter**:
```json
{
  "name": "cluster_operators_list",
  "arguments": {
    "status": "degraded"
  }
}
```

#### cluster_operator_get
Get detailed information for a specific cluster operator

**Parameters**:
- `name` (required) - Operator name (e.g., "etcd", "kube-apiserver")

**Returns**:
- Detailed status conditions with messages and reasons
- Component versions
- Related objects (namespaces, resources managed by the operator)

**Example Output**:
```
Cluster Operator: etcd
================================================================================

Status Conditions:
--------------------------------------------------------------------------------
✗ Degraded: False
  Reason: AsExpected
  Last Transition: 2026-01-12T13:52:56Z
  Message:
    NodeControllerDegraded: All master nodes are ready
    EtcdMembersDegraded: No unhealthy members found

✗ Progressing: False
  Reason: AsExpected
  Last Transition: 2026-01-12T14:11:22Z
  Message:
    NodeInstallerProgressing: 3 nodes are at revision 8
    EtcdMembersProgressing: No unstarted etcd members found

✓ Available: True
  Reason: AsExpected
  Last Transition: 2026-01-12T13:47:24Z
  Message:
    StaticPodsAvailable: 3 nodes are active; 3 nodes are at revision 8
    EtcdMembersAvailable: 3 members are available

✓ Upgradeable: True
  Reason: AsExpected
  Last Transition: 2026-01-12T13:44:48Z
  Message: All is well

Versions:
--------------------------------------------------------------------------------
  raw-internal: 4.21.0-okd-scos.ec.18
  etcd: 4.21.0-okd-scos.ec.18
  operator: 4.21.0-okd-scos.ec.18

Related Objects:
--------------------------------------------------------------------------------
  - operator.openshift.io/etcds cluster
  - namespaces openshift-config
  - namespaces openshift-config-managed
  - namespaces openshift-etcd-operator
  - namespaces openshift-etcd
```

### 4. Cluster Node Tools

#### cluster_nodes_list
List all cluster nodes with status and roles

**Parameters**:
- `role` (optional) - Filter by role: all, master, worker (default: all)

**Returns**:
- Node names
- Roles (master, worker, infra, etc.)
- Ready status
- Kubelet version

**Example Output**:
```
Cluster Nodes
================================================================================

NAME                                     ROLES           STATUS     VERSION
--------------------------------------------------------------------------------
ip-10-0-122-129.us-east-2.compute.int... master          Ready      v1.30.9
ip-10-0-124-129.us-east-2.compute.int... master          Ready      v1.30.9
ip-10-0-49-48.us-east-2.compute.inter... worker          Ready      v1.30.9
ip-10-0-6-220.us-east-2.compute.inter... worker          Ready      v1.30.9
ip-10-0-67-249.us-east-2.compute.inte... master          Ready      v1.30.9
ip-10-0-97-146.us-east-2.compute.inte... worker          Ready      v1.30.9

Total Nodes: 6
```

**With Role Filter**:
```json
{
  "name": "cluster_nodes_list",
  "arguments": {
    "role": "master"
  }
}
```

#### cluster_node_get
Get detailed information for a specific node

**Parameters**:
- `name` (required) - Node name

**Returns**:
- Node roles
- Ready status
- System information (OS, kernel, container runtime)
- Resource capacity and allocatable
- Addresses (InternalIP, Hostname, etc.)
- Conditions (Ready, MemoryPressure, DiskPressure, PIDPressure, NetworkUnavailable)
- Taints

**Example Output**:
```
Node: ip-10-0-122-129.us-east-2.compute.internal
================================================================================

Roles: master
Status: Ready

System Information:
--------------------------------------------------------------------------------
OS Image: Fedora CoreOS 41.20250104.3.0
Kernel Version: 6.12.8-200.fc41.x86_64
Container Runtime: cri-o://1.31.4-dev
Kubelet Version: v1.30.9
Kube-Proxy Version: v1.30.9
Machine ID: 8e2c1d9f8a6b4e5c9d7a3b2f1e0c4d5a
System UUID: ec2d6e51-7a8b-9c3d-4e5f-1a2b3c4d5e6f

Resources:
--------------------------------------------------------------------------------
Capacity:
  cpu: 4
  ephemeral-storage: 125831224Ki
  hugepages-1Gi: 0
  hugepages-2Mi: 0
  memory: 16369268Ki
  pods: 250
Allocatable:
  cpu: 3500m
  ephemeral-storage: 115975528800
  hugepages-1Gi: 0
  hugepages-2Mi: 0
  memory: 15218788Ki
  pods: 250

Addresses:
--------------------------------------------------------------------------------
  InternalIP: 10.0.122.129
  Hostname: ip-10-0-122-129.us-east-2.compute.internal
  InternalDNS: ip-10-0-122-129.us-east-2.compute.internal

Conditions:
--------------------------------------------------------------------------------
✓ Ready: True
  Reason: KubeletReady
  Message: kubelet is posting ready status

✗ MemoryPressure: False
  Reason: KubeletHasSufficientMemory

✗ DiskPressure: False
  Reason: KubeletHasNoDiskPressure

✗ PIDPressure: False
  Reason: KubeletHasSufficientPID

✗ NetworkUnavailable: False
  Reason: RouteCreated
```

## Architecture

The cluster toolset uses the existing MustGatherProvider interface to query resources from the indexed data:

```go
// Query cluster operators
gvk := parseGVK("config.openshift.io/v1", "ClusterOperator")
operatorList, err := provider.ListResources(ctx, gvk, "", opts)

// Query nodes
gvk := parseGVK("v1", "Node")
nodeList, err := provider.ListResources(ctx, gvk, "", opts)
```

## Tool Organization

The cluster toolset is organized into separate files by category:

- **version.go** - Cluster version tools
- **info.go** - Infrastructure and network information tools
- **operators.go** - Cluster operator tools
- **nodes.go** - Node tools
- **helpers.go** - Shared utilities (parseGVK)
- **toolset.go** - Toolset registration

## Usage Examples

### Check Cluster Version
```json
{
  "name": "cluster_version_get",
  "arguments": {}
}
```

### List All Degraded Operators
```json
{
  "name": "cluster_operators_list",
  "arguments": {
    "status": "degraded"
  }
}
```

### Get Details for a Specific Operator
```json
{
  "name": "cluster_operator_get",
  "arguments": {
    "name": "etcd"
  }
}
```

### Check Cluster Infrastructure
```json
{
  "name": "cluster_info_get",
  "arguments": {}
}
```

### List All Master Nodes
```json
{
  "name": "cluster_nodes_list",
  "arguments": {
    "role": "master"
  }
}
```

### Get Details for a Specific Node
```json
{
  "name": "cluster_node_get",
  "arguments": {
    "name": "ip-10-0-122-129.us-east-2.compute.internal"
  }
}
```

## Integration with AI Assistants

These tools enable AI assistants to perform cluster-level troubleshooting:

**Example Queries**:
- "What version of OpenShift is this cluster running?"
- "Show me all degraded cluster operators"
- "What platform is this cluster running on?"
- "Which nodes are masters and which are workers?"
- "Show me the details of the etcd operator"
- "Are there any nodes with issues?"
- "What's the cluster network configuration?"

## Build Status

✅ **Build Successful**
```
Registered 5 toolsets
Registering 6 tools from toolset: cluster
Registering 3 tools from toolset: core
Registering 9 tools from toolset: diagnostics
Registering 8 tools from toolset: monitoring
Registering 3 tools from toolset: network
```

**Total Tools**: 30 (6 cluster + 3 core + 9 diagnostics + 8 monitoring + 3 network)

## Future Enhancements

Potential additions to the cluster toolset:
1. **cluster_operators_summary** - Quick health overview of all operators
2. **cluster_health_check** - Comprehensive cluster health report
3. **cluster_capacity_get** - Aggregated cluster capacity across all nodes
4. **cluster_alerts_list** - Parse Prometheus alerts (if available)
5. **cluster_certificates_check** - Certificate expiration warnings
6. **cluster_machineconfig_list** - MachineConfig and MachineConfigPool status
