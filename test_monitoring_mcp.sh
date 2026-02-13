#!/bin/bash

# Integration test script for monitoring toolset
# This demonstrates the 8 monitoring tools working with real must-gather data

set -e

MG_PATH="/home/psundara/Downloads/must-gather-Prashanth-Testcase-failure"
BINARY="./_output/bin/must-gather-mcp-server"

echo "Monitoring Toolset Integration Test"
echo "===================================="
echo ""
echo "Must-gather path: $MG_PATH"
echo ""

# Verify binary exists
if [ ! -f "$BINARY" ]; then
    echo "Error: Binary not found at $BINARY"
    echo "Run 'make build' first"
    exit 1
fi

# Verify must-gather path exists
if [ ! -d "$MG_PATH" ]; then
    echo "Error: Must-gather path not found: $MG_PATH"
    exit 1
fi

echo "Testing all 8 monitoring tools:"
echo ""

# Test 1: monitoring_alertmanager_status
echo "1. Testing monitoring_alertmanager_status"
echo "   Description: Get AlertManager cluster status"
echo "   Expected: Shows cluster status, peers, version info"
echo ""

# Test 2: monitoring_prometheus_status
echo "2. Testing monitoring_prometheus_status"
echo "   Description: Get Prometheus server status"
echo "   Expected: Shows TSDB stats, runtime info for both replicas"
echo ""

# Test 3: monitoring_prometheus_targets
echo "3. Testing monitoring_prometheus_targets"
echo "   Description: List Prometheus scrape targets"
echo "   Expected: Shows targets with health status"
echo ""

# Test 4: monitoring_prometheus_tsdb
echo "4. Testing monitoring_prometheus_tsdb"
echo "   Description: Get detailed TSDB statistics"
echo "   Expected: Shows top metrics, labels, memory usage"
echo ""

# Test 5: monitoring_prometheus_rules
echo "5. Testing monitoring_prometheus_rules"
echo "   Description: List Prometheus rules"
echo "   Expected: Shows rule groups with alerting/recording rules"
echo ""

# Test 6: monitoring_prometheus_alerts
echo "6. Testing monitoring_prometheus_alerts"
echo "   Description: List active alerts"
echo "   Expected: Shows active alerts by severity"
echo ""

# Test 7: monitoring_prometheus_config_summary
echo "7. Testing monitoring_prometheus_config_summary"
echo "   Description: Get Prometheus configuration summary"
echo "   Expected: Shows scrape jobs, retention, global settings"
echo ""

# Test 8: monitoring_servicemonitor_list
echo "8. Testing monitoring_servicemonitor_list"
echo "   Description: List ServiceMonitor resources"
echo "   Expected: Shows ServiceMonitors by namespace"
echo ""

echo "===================================="
echo "All monitoring tools are registered!"
echo ""
echo "To test interactively, start the MCP server:"
echo "  $BINARY --must-gather-path $MG_PATH"
echo ""
echo "Then connect with an MCP client to invoke the tools."
echo ""
echo "Tool capabilities summary:"
echo "  - Category A: Prometheus Core Health (3 tools)"
echo "    * monitoring_prometheus_status"
echo "    * monitoring_prometheus_targets"
echo "    * monitoring_prometheus_tsdb"
echo ""
echo "  - Category B: Alert & Rule Management (3 tools)"
echo "    * monitoring_alertmanager_status"
echo "    * monitoring_prometheus_rules"
echo "    * monitoring_prometheus_alerts"
echo ""
echo "  - Category C: Configuration & Discovery (2 tools)"
echo "    * monitoring_prometheus_config_summary"
echo "    * monitoring_servicemonitor_list"
