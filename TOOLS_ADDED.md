# New Tools Added to Must-Gather MCP Server

## Summary

**Total Tools: 57** (was 30, added 27 new tools)

## Toolset Breakdown

### Cluster Toolset: 23 tools (was 6, +17)

#### Machine Configuration (4 tools)
1. `machineconfig_list` - List all MachineConfigs (OS and systemd configuration)
2. `machineconfig_get` - Get detailed MachineConfig including ignition configuration
3. `machineconfigpool_status` - Check pool degradation and update status
4. `machineconfignode_status` - Per-node config application status

#### Storage Analysis (4 tools)
5. `storage_classes_list` - List StorageClasses with provisioners and defaults
6. `csi_drivers_status` - CSI driver capabilities and status
7. `volume_attachments_list` - Find stuck or failing volume mounts
8. `persistent_volumes_status` - PV status (available, bound, failed)

#### Security & RBAC (2 tools)
9. `security_scc_list` - SecurityContextConstraints with privilege analysis
10. `rbac_clusterroles_list` - ClusterRoles with dangerous permission detection

#### OLM/Operator Management (3 tools)
11. `olm_subscriptions_status` - Operator subscription and upgrade status
12. `olm_catalogsources_status` - Catalog source connectivity and health
13. `olm_installplans_status` - Pending operator installations

#### Admission Control (2 tools)
14. `admission_webhooks_list` - Validating and mutating webhooks
15. `admission_policies_list` - CEL-based admission policies

#### Configuration Resources (2 tools)
16. `cluster_config_list` - List all config.openshift.io resources
17. `cluster_config_get` - Get detailed cluster configuration

### Core Toolset: 6 tools (was 3, +3)

#### Events Analysis (3 tools)
18. `events_list` - Filter events by type/namespace/reason (7,730 events found)
19. `events_timeline` - Chronological event sequence for incident analysis
20. `events_by_resource` - All events for specific pods/nodes/deployments

### Diagnostics Toolset: 17 tools (was 10, +7)

#### Host Service Logs (3 tools)
21. `host_service_logs_list` - List systemd services (11 services: kubelet, crio, NetworkManager, etc.)
22. `host_service_logs_get` - Get specific service logs with tail support
23. `host_service_logs_grep` - Search across all host service logs

#### Static Pod Termination Logs (1 tool)
24. `static_pod_termination_logs` - Control plane pod crash logs (kube-apiserver, etcd, etc.)

#### Extended Node Diagnostics (3 tools)
25. `node_hardware_info` - CPU topology, PCI devices, network interfaces
26. `node_dmesg_errors` - Parse kernel logs for errors, OOM kills, hardware issues
27. `node_kernel_info` - Kernel boot parameters and configuration

### Monitoring Toolset: 8 tools (unchanged)
- Prometheus, AlertManager, alerts, rules, targets, TSDB, config

### Network Toolset: 3 tools (unchanged)
- Network scale, OVN resources, connectivity checks

## Data Sources Accessed

### New Directories Mined
1. **host_service_logs/masters/** - Systemd service logs
   - kubelet, crio, NetworkManager, openvswitch, machine-config-daemon

2. **static-pods/** - Static pod termination logs
   - kube-apiserver, etcd, kube-controller-manager, kube-scheduler

3. **nodes/{node}/*** - Extended node diagnostics
   - lscpu, lspci, dmesg, proc_cmdline, ethtool_*

### New Resource Types
1. **machineconfiguration.openshift.io**
   - MachineConfig, MachineConfigPool, MachineConfigNode

2. **storage.k8s.io**
   - StorageClass, CSIDriver, VolumeAttachment, PersistentVolume

3. **security.openshift.io**
   - SecurityContextConstraints

4. **rbac.authorization.k8s.io**
   - ClusterRole

5. **operators.coreos.com**
   - Subscription, CatalogSource, InstallPlan

6. **admissionregistration.k8s.io**
   - ValidatingWebhookConfiguration, MutatingWebhookConfiguration
   - ValidatingAdmissionPolicy, ValidatingAdmissionPolicyBinding

7. **config.openshift.io**
   - Authentication, Console, DNS, FeatureGate, OAuth, Proxy, Network, etc.

8. **v1/Event**
   - All Kubernetes events (7,730 in test cluster)

## Key Features

### Filtering & Search
- Events: Filter by type, namespace, resource, reason
- Host logs: Search with case-insensitive grep across all services
- Storage: Filter by attachment status, phase
- OLM: Filter by state, approval status
- Security: Identify dangerous permissions automatically

### Health Analysis
- MachineConfigPool degradation detection
- OLM subscription upgrade failures
- Volume attachment failures
- dmesg error parsing (OOM, hardware errors)
- Admission webhook failure policies

### Debugging Capabilities
- Static pod crash logs for control plane debugging
- Host-level systemd service logs
- Kernel boot parameters and dmesg analysis
- Hardware inventory (CPU, PCI, network)
- Event timeline for incident reconstruction

## Usage Examples

### Debug Control Plane Crashes
```
static_pod_termination_logs --pod kube-apiserver --node master-0
```

### Find OOM Kills
```
node_dmesg_errors --severity oom
```

### Investigate Failed Deployments
```
events_by_resource --name my-deployment --namespace default
events_timeline --hours 2 --type Warning
```

### Debug Storage Issues
```
volume_attachments_list --attached false
persistent_volumes_status --phase Failed
```

### Check Operator Health
```
olm_subscriptions_status --state UpgradeFailed
olm_installplans_status --approved false
```

### Analyze Security
```
security_scc_list
rbac_clusterroles_list --role admin
```

### Host-Level Debugging
```
host_service_logs_list
host_service_logs_grep --filter "error" --caseInsensitive true
node_hardware_info --node master-0
```

## Testing Results

All tools tested successfully with must-gather-Prashanth-Testcase-failure:
- ✅ 11 host service logs detected
- ✅ 7,730 events indexed and searchable
- ✅ Static pod termination logs accessible
- ✅ Node diagnostics including dmesg, lscpu, lspci
- ✅ All Kubernetes resource types accessible

## Future Enhancements

Potential additions:
1. API service status
2. Custom Resource Definitions (CRD) analysis
3. Network policy analysis
4. Resource quota and limit analysis
5. Image pull errors analysis
6. Certificate expiration checking
7. Must-gather collection metadata
