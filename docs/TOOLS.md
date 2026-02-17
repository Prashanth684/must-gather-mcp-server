# MCP Tools Reference

Complete reference for all 61 tools across 5 toolsets in must-gather-mcp-server.

## Overview

**Total Tools: 61** across 5 toolsets
- Cluster Toolset: 27 tools
- Core Toolset: 6 tools
- Diagnostics Toolset: 17 tools
- Network Toolset: 3 tools
- Monitoring Toolset: 8 tools

## Cluster Toolset (27 tools)

### Version & Info (6 tools)

#### cluster_version_get
Get OpenShift cluster version information.

**Returns:**
- Cluster ID
- Current version
- Image reference
- Status conditions (Available, Progressing, Failing)
- Enabled capabilities
- Version history

#### cluster_info_get
Get cluster infrastructure and network configuration.

**Returns:**
- Platform type (AWS, Azure, GCP, Bare Metal, etc.)
- Region/Location
- Control plane topology (HighlyAvailable vs SingleNode)
- API server URLs
- Cluster network configuration
- Service network configuration
- Network type (OVNKubernetes, OpenShiftSDN)

#### cluster_operators_list
List all cluster operators with their status.

**Returns:**
- Operator names
- Available/Progressing/Degraded status
- Version information

#### cluster_operator_get
Get detailed information about a specific cluster operator.

**Parameters:**
- `name` (string, required) - Operator name

**Returns:**
- Detailed conditions
- Versions
- Related objects

#### cluster_nodes_list
List all cluster nodes.

**Returns:**
- Node names
- Roles (master, worker)
- Status (Ready, NotReady)
- Kubelet version

#### cluster_node_get
Get detailed information about a specific node.

**Parameters:**
- `name` (string, required) - Node name

**Returns:**
- Capacity and allocatable resources
- Conditions
- Taints and labels
- System info

### Machine Configuration (4 tools)

#### machineconfig_list
List all MachineConfigs (OS and systemd configuration).

**Returns:**
- MachineConfig names
- Configuration metadata

#### machineconfig_get
Get detailed MachineConfig including ignition configuration.

**Parameters:**
- `name` (string, required) - MachineConfig name

**Returns:**
- Ignition configuration
- Systemd units
- File configurations

#### machineconfigpool_status
Check MachineConfigPool degradation and update status.

**Returns:**
- Pool names
- Degraded status
- Update progress
- Machine counts

#### machineconfignode_status
Per-node config application status.

**Parameters:**
- `node` (string, optional) - Filter by node name

**Returns:**
- Node name
- Current config
- Desired config
- Degraded status

### Storage (4 tools)

#### storage_classes_list
List StorageClasses with provisioners and defaults.

**Returns:**
- StorageClass names
- Provisioners
- Default status
- Volume binding mode

#### csi_drivers_status
Get CSI driver capabilities and status.

**Returns:**
- Driver names
- Capabilities (attach, volume lifecycle)
- Storage capacity

#### volume_attachments_list
Find stuck or failing volume mounts.

**Parameters:**
- `attached` (boolean, optional) - Filter by attachment status

**Returns:**
- Volume names
- Attached status
- Node attachments
- Error details

#### persistent_volumes_status
Get PersistentVolume status (available, bound, failed).

**Parameters:**
- `phase` (string, optional) - Filter by phase (Available, Bound, Failed, Released)

**Returns:**
- PV names
- Phase
- Capacity
- Storage class
- Claim reference

### Security & RBAC (2 tools)

#### security_scc_list
List SecurityContextConstraints with privilege analysis.

**Returns:**
- SCC names
- Allow privileged
- Capabilities
- SELinux context
- Users and groups

#### rbac_clusterroles_list
List ClusterRoles with dangerous permission detection.

**Parameters:**
- `role` (string, optional) - Filter by role name

**Returns:**
- ClusterRole names
- Rules
- Dangerous permissions (wildcards, escalate, bind)

### OLM/Operators (3 tools)

#### olm_subscriptions_status
Get operator subscription and upgrade status.

**Parameters:**
- `namespace` (string, optional) - Filter by namespace
- `state` (string, optional) - Filter by state (UpgradeFailed, etc.)

**Returns:**
- Subscription names
- Current CSV
- Installed CSV
- State
- Upgrade availability

#### olm_catalogsources_status
Get catalog source connectivity and health.

**Returns:**
- Catalog names
- Connection state
- Registry server
- Last update time

#### olm_installplans_status
Get pending operator installations.

**Parameters:**
- `namespace` (string, optional) - Filter by namespace
- `approved` (boolean, optional) - Filter by approval status

**Returns:**
- InstallPlan names
- Approval status
- Phase
- ClusterServiceVersion

### Admission Control (2 tools)

#### admission_webhooks_list
List validating and mutating webhooks.

**Returns:**
- Webhook names
- Type (Validating, Mutating)
- Rules (resources, operations)
- Failure policy
- Client config

#### admission_policies_list
List CEL-based admission policies.

**Returns:**
- Policy names
- Match constraints
- Validations
- Failure policy

### Configuration (2 tools)

#### cluster_config_list
List all config.openshift.io resources.

**Returns:**
- Resource types
- Names
- Counts

#### cluster_config_get
Get detailed cluster configuration.

**Parameters:**
- `kind` (string, required) - Resource kind (OAuth, FeatureGate, etc.)
- `name` (string, optional) - Resource name (default: "cluster")

**Returns:**
- Full configuration YAML

### Ingress & Routing (4 tools)

#### routes_list
List OpenShift Routes with hosts, services, TLS config, and admission status.

**Parameters:**
- `namespace` (string, optional) - Filter by namespace

**Returns:**
- Route names
- Host
- Service
- TLS termination
- Admission status

#### route_get
Get detailed Route information including backend weights and TLS termination.

**Parameters:**
- `name` (string, required) - Route name
- `namespace` (string, required) - Namespace

**Returns:**
- Complete route configuration
- Backend services with weights
- TLS certificate details

#### ingress_list
List Kubernetes Ingress resources with rules and backends.

**Parameters:**
- `namespace` (string, optional) - Filter by namespace

**Returns:**
- Ingress names
- Rules
- Backends
- TLS configuration

#### ingresscontroller_status
Get IngressController (router) status, replicas, and conditions.

**Returns:**
- IngressController name
- Replicas (desired/available)
- Conditions
- Domain

## Core Toolset (6 tools)

### Resources (3 tools)

#### resources_get
Get any Kubernetes resource by kind/name/namespace.

**Parameters:**
- `kind` (string, required) - Resource kind (Pod, Deployment, etc.)
- `name` (string, required) - Resource name
- `namespace` (string, optional) - Namespace (for namespaced resources)

**Returns:**
- Full resource YAML

#### resources_list
List resources with label/field selectors.

**Parameters:**
- `kind` (string, required) - Resource kind
- `namespace` (string, optional) - Namespace filter
- `labels` (map, optional) - Label selectors

**Returns:**
- List of matching resources

#### namespaces_list
List all namespaces.

**Returns:**
- Namespace names
- Status
- Labels

### Events (3 tools)

#### events_list
Filter events by type/namespace/reason.

**Parameters:**
- `namespace` (string, optional) - Filter by namespace
- `type` (string, optional) - Filter by type (Normal, Warning)
- `reason` (string, optional) - Filter by reason

**Returns:**
- Event details
- Resource references
- Messages
- Timestamps

#### events_timeline
Chronological event sequence for incident analysis.

**Parameters:**
- `namespace` (string, optional) - Filter by namespace
- `hours` (int, optional) - Look back hours (default: 1)
- `type` (string, optional) - Filter by type

**Returns:**
- Time-ordered events
- Incident timeline

#### events_by_resource
All events for specific pods/nodes/deployments.

**Parameters:**
- `name` (string, required) - Resource name
- `kind` (string, optional) - Resource kind (default: Pod)
- `namespace` (string, optional) - Namespace (for namespaced resources)

**Returns:**
- Events related to the resource

## Diagnostics Toolset (17 tools)

### Pod Logs (2 tools)

#### pod_logs_get
Get container logs with tail support.

**Parameters:**
- `namespace` (string, required) - Pod namespace
- `pod` (string, required) - Pod name
- `container` (string, optional) - Container name
- `tail` (int, optional) - Number of lines to tail
- `previous` (boolean, optional) - Get previous logs

**Returns:**
- Container logs

#### pod_containers_list
List containers with logs.

**Parameters:**
- `namespace` (string, required) - Pod namespace
- `pod` (string, required) - Pod name

**Returns:**
- Container names

### Node Diagnostics (4 tools)

#### nodes_list
List nodes with diagnostic data available.

**Returns:**
- Node names with diagnostic data

#### node_diagnostics_get
Get comprehensive node diagnostics.

**Parameters:**
- `node` (string, required) - Node name
- `include` (string, optional) - Comma-separated: kubelet,sysinfo,cpu,irq,hardware
- `kubeletTail` (int, optional) - Kubelet log tail lines

**Returns:**
- Requested diagnostic data

#### node_kubelet_logs
Get kubelet logs (auto-decompressed from .gz).

**Parameters:**
- `node` (string, required) - Node name
- `tail` (int, optional) - Number of lines to tail

**Returns:**
- Kubelet logs

#### node_kubelet_logs_grep
Filter kubelet logs by string.

**Parameters:**
- `node` (string, optional) - Node name (or all nodes)
- `filter` (string, required) - Search string
- `caseInsensitive` (boolean, optional) - Case-insensitive search

**Returns:**
- Matching log lines

### Extended Node Diagnostics (3 tools)

#### node_hardware_info
Get CPU topology, PCI devices, network interfaces.

**Parameters:**
- `node` (string, required) - Node name

**Returns:**
- CPU info (lscpu)
- PCI devices (lspci)
- Network interfaces (ethtool)

#### node_dmesg_errors
Parse kernel logs for errors, OOM kills, hardware issues.

**Parameters:**
- `node` (string, optional) - Node name (or all nodes)
- `severity` (string, optional) - Filter by severity (error, warn, oom)

**Returns:**
- dmesg errors and warnings

#### node_kernel_info
Get kernel boot parameters and configuration.

**Parameters:**
- `node` (string, required) - Node name

**Returns:**
- Kernel command line parameters

### Host Service Logs (3 tools)

#### host_service_logs_list
List systemd services.

**Returns:**
- Service names (kubelet, crio, NetworkManager, etc.)

#### host_service_logs_get
Get specific service logs with tail support.

**Parameters:**
- `service` (string, required) - Service name
- `node` (string, optional) - Node name
- `tail` (int, optional) - Number of lines to tail

**Returns:**
- Service logs

#### host_service_logs_grep
Search across all host service logs.

**Parameters:**
- `filter` (string, required) - Search string
- `caseInsensitive` (boolean, optional) - Case-insensitive search
- `service` (string, optional) - Filter by service

**Returns:**
- Matching log lines across all services

### Static Pods (1 tool)

#### static_pod_termination_logs
Get control plane pod crash logs.

**Parameters:**
- `pod` (string, optional) - Pod name (kube-apiserver, etcd, etc.)
- `node` (string, optional) - Node name

**Returns:**
- Termination logs for crashed static pods

### ETCD (4 tools)

#### etcd_health
Get cluster health, endpoint status, alarms.

**Returns:**
- Cluster health status
- Endpoint health
- Active alarms

#### etcd_object_count
Get resource type object counts.

**Parameters:**
- `sortBy` (string, optional) - Sort by count or name
- `top` (int, optional) - Limit results

**Returns:**
- Object counts by resource type

#### etcd_members_list
Get member IDs, peer/client URLs.

**Returns:**
- Member list
- Peer URLs
- Client URLs

#### etcd_endpoint_status
Get DB size, quota usage, raft state, leader info.

**Returns:**
- Database size
- Quota usage
- Raft index
- Leader information

## Network Toolset (3 tools)

#### network_scale_get
Get network resource counts (services, pods, policies).

**Returns:**
- Service count
- Pod count
- NetworkPolicy count
- Endpoints count

#### network_ovn_resources
Get OVN Kubernetes component resource usage.

**Returns:**
- OVN component CPU/memory usage
- Resource limits

#### network_connectivity_check
Get pod connectivity test results with failure analysis.

**Returns:**
- Connectivity check results
- Failed checks with details
- Success rate

## Monitoring Toolset (8 tools)

### Prometheus Core Health

#### monitoring_prometheus_status
Get Prometheus server status with TSDB statistics.

**Returns:**
- Server status
- TSDB stats
- Runtime information

#### monitoring_prometheus_targets
Get scrape targets with health filtering.

**Parameters:**
- `health` (string, optional) - Filter by health (up, down, unknown)

**Returns:**
- Target endpoints
- Health status
- Last scrape time
- Errors

#### monitoring_prometheus_tsdb
Get detailed TSDB statistics with top metrics and label cardinality.

**Returns:**
- Series count
- Chunk count
- Top metrics by series
- Label cardinality

### Alert & Rule Management

#### monitoring_alertmanager_status
Get AlertManager cluster status and version info.

**Returns:**
- Cluster status
- Version
- Uptime
- Configuration

#### monitoring_prometheus_rules
Get recording and alerting rules with health status.

**Returns:**
- Rule names
- Type (recording, alerting)
- Health status
- Evaluation time

#### monitoring_prometheus_alerts
Get active alerts with severity filtering.

**Parameters:**
- `severity` (string, optional) - Filter by severity (critical, warning, info)
- `state` (string, optional) - Filter by state (firing, pending)

**Returns:**
- Alert names
- Severity
- State
- Annotations

### Configuration & Discovery

#### monitoring_prometheus_config_summary
Get configuration overview with scrape jobs and global settings.

**Returns:**
- Global configuration
- Scrape configs
- Alerting configuration

#### monitoring_servicemonitor_list
List ServiceMonitor CRDs for scrape target discovery.

**Parameters:**
- `namespace` (string, optional) - Filter by namespace

**Returns:**
- ServiceMonitor names
- Selector labels
- Endpoints

## Usage Examples

### Cluster Analysis
```
"What version of OpenShift is this cluster running?"
"Show me all degraded cluster operators"
"List all master nodes and their status"
```

### Storage Troubleshooting
```
"Which is the default StorageClass?"
"Show me all failing volume attachments"
"List all PersistentVolumes that are in Failed state"
```

### Security Analysis
```
"Which SecurityContextConstraints allow privileged containers?"
"Show me ClusterRoles with wildcard permissions"
```

### Events & Incidents
```
"Show me all Warning events from the last hour"
"Timeline of events for namespace openshift-etcd"
"Find all events with reason 'BackOff' or 'Failed'"
```

### ETCD Monitoring
```
"Check ETCD cluster health"
"What's the ETCD database size and quota usage?"
"Is there any raft lag in the ETCD cluster?"
```

### Network Troubleshooting
```
"Show me all failing network connectivity checks"
"What's the network scale of this cluster?"
"Which OVN components are using the most resources?"
```

### Pod & Node Diagnostics
```
"Get logs for pod X in namespace Y"
"Show me kubelet logs for node Z"
"Search for 'OOM' in kubelet logs for all nodes"
"Get comprehensive diagnostics for node A"
```

### Control Plane Debugging
```
"Show me kube-apiserver termination logs for master-0"
"Are there any control plane pod crashes?"
```

### Monitoring & Observability
```
"What's the Prometheus server status and TSDB statistics?"
"Show me all failing Prometheus scrape targets"
"List all critical alerts currently firing"
```
