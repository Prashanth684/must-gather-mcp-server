# Running with Goose (or other HTTP-based MCP clients)

## Quick Start

### 1. Build the Server

```bash
cd /path/to/must-gather-mcp-server
make build
```

### 2. Start the MCP Server in HTTP Mode

```bash
./must-gather-mcp-server \
  --must-gather-path /path/to/your/must-gather \
  --http \
  --http-addr localhost:8080
```

You should see:
```
Loading must-gather from: /path/to/your/must-gather
Loaded 11100 resources from 69 namespaces
Building resource index...
Index built with 10655 resources
Registered 4 toolsets
Registering 6 tools from toolset: cluster
Registering 3 tools from toolset: core
Registering 9 tools from toolset: diagnostics
Registering 3 tools from toolset: network
Starting must-gather MCP server in HTTP/SSE mode...
Starting MCP server on http://localhost:8080
SSE endpoint: http://localhost:8080/sse (GET request to establish connection)
Message endpoint: http://localhost:8080/messages/<session-id> (POST for sending messages)
```

### 3. Configure Goose

Add to your Goose configuration file (typically `~/.config/goose/config.yaml` or project-specific config):

```yaml
mcp_servers:
  must-gather:
    url: http://localhost:8080/sse
```

### 4. Start Goose

```bash
goose session start
```

Goose will automatically connect to the MCP server and load the 21 available tools.

## Using with Goose

Once connected, you can ask Goose to analyze the must-gather:

### Cluster Analysis
```
Ask Goose: "What version of OpenShift is this cluster running?"
Ask Goose: "Show me all degraded cluster operators"
Ask Goose: "Analyze the cluster health"
```

### ETCD Monitoring
```
Ask Goose: "Check ETCD cluster health and database usage"
Ask Goose: "Are there any ETCD issues?"
Ask Goose: "Show me ETCD member list and endpoint status"
```

### Network Troubleshooting
```
Ask Goose: "Are there any network connectivity failures?"
Ask Goose: "What's the network scale of this cluster?"
Ask Goose: "Which OVN components are using excessive resources?"
```

### Pod & Node Diagnostics
```
Ask Goose: "Get logs for the etcd pod in openshift-etcd namespace"
Ask Goose: "Show me kubelet logs for the master nodes"
Ask Goose: "Analyze node health and capacity"
```

## Alternative: Using curl to Test

You can test the HTTP/SSE endpoint with curl:

```bash
# Establish SSE connection
curl -N -H "Accept: text/event-stream" http://localhost:8080/sse
```

This will show the session ID in the SSE endpoint response.

## Troubleshooting

### Port Already in Use
```
Error: listen tcp :8080: bind: address already in use
```

Solution: Use a different port:
```bash
./must-gather-mcp-server \
  --must-gather-path /path/to/must-gather \
  --http \
  --http-addr localhost:8888
```

Then update Goose config to use `http://localhost:8888/sse`.

### Connection Refused
```
Error: connection refused
```

Solution: Ensure the MCP server is running and listening on the correct address. Check that:
1. The server started successfully
2. No firewall is blocking the port
3. You're using the correct host:port in Goose config

### Server Not Loading Must-Gather
```
Error: must-gather path does not exist
```

Solution: Verify the path is correct and points to the extracted must-gather directory:
```bash
ls -la /path/to/must-gather
# Should show: quay-io-okd-scos-content-sha256-.../ directory
```

## Advanced Configuration

### Running on a Different Host

To allow connections from other machines:

```bash
./must-gather-mcp-server \
  --must-gather-path /path/to/must-gather \
  --http \
  --http-addr 0.0.0.0:8080
```

Then configure Goose to use `http://<server-ip>:8080/sse`.

**Security Note**: Only do this on trusted networks. The server does not have authentication.

### Running as a Background Service

Using systemd (Linux):

```ini
# /etc/systemd/system/must-gather-mcp.service
[Unit]
Description=Must-Gather MCP Server
After=network.target

[Service]
Type=simple
ExecStart=/path/to/must-gather-mcp-server \
  --must-gather-path /path/to/must-gather \
  --http \
  --http-addr localhost:8080
Restart=on-failure
User=your-user

[Install]
WantedBy=multi-user.target
```

Then:
```bash
sudo systemctl daemon-reload
sudo systemctl start must-gather-mcp
sudo systemctl enable must-gather-mcp
```

### Using Docker

```dockerfile
FROM golang:1.25 as builder
WORKDIR /build
COPY . .
RUN make build

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /build/_output/bin/must-gather-mcp-server /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/must-gather-mcp-server"]
```

Build and run:
```bash
docker build -t must-gather-mcp-server .
docker run -p 8080:8080 \
  -v /path/to/must-gather:/must-gather:ro \
  must-gather-mcp-server \
  --must-gather-path /must-gather \
  --http \
  --http-addr 0.0.0.0:8080
```

## Available Tools

When connected, Goose has access to 21 tools across 4 toolsets:

### Cluster Tools (6)
- cluster_version_get
- cluster_info_get
- cluster_operators_list
- cluster_operator_get
- cluster_nodes_list
- cluster_node_get

### Core Tools (3)
- resources_get
- resources_list
- namespaces_list

### Diagnostics Tools (9)
- pod_logs_get
- pod_containers_list
- nodes_list
- node_diagnostics_get
- node_kubelet_logs
- etcd_health
- etcd_object_count
- etcd_members_list
- etcd_endpoint_status

### Network Tools (3)
- network_scale_get
- network_ovn_resources
- network_connectivity_check

## Example Workflow

Complete troubleshooting workflow with Goose:

```
1. Start MCP server:
   ./must-gather-mcp-server --must-gather-path /path/to/must-gather --http

2. Start Goose:
   goose session start

3. Ask Goose to analyze:
   "Analyze this OpenShift cluster's health. Start by checking the version,
    then look at operator status, ETCD health, and any network connectivity issues.
    If you find any problems, investigate the related logs."

4. Goose will automatically:
   - Call cluster_version_get
   - Call cluster_operators_list
   - Call etcd_health and etcd_endpoint_status
   - Call network_connectivity_check
   - Based on findings, may call pod_logs_get or node_kubelet_logs
   - Provide a comprehensive analysis report
```

## Performance Tips

- **Startup time**: The server takes 5-10 seconds to load and index resources. Wait for "Starting must-gather MCP server..." before connecting Goose.
- **Query performance**: Most queries respond in <50ms. Log retrievals may take up to 500ms.
- **Memory usage**: Expect ~100-200MB for the indexed resources.

## Stopping the Server

- **Ctrl+C**: Gracefully shuts down the HTTP server
- **Kill signal**: `kill <pid>` or `pkill must-gather-mcp-server`

The server handles shutdown gracefully and closes all connections properly.
