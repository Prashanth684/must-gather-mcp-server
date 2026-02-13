#!/bin/bash

# Test script to verify monitoring toolset functionality
# This tests each tool manually using JSON files

set -e

MG_PATH="/home/psundara/Downloads/must-gather-Prashanth-Testcase-failure"
CONTAINER_DIR="$MG_PATH/quay-io-okd-scos-content-sha256-02875b0c9bd3440d2fa919c89bd21b15fd2cccdc9539f0caa9240003d5cffc57"
MONITORING_DIR="$CONTAINER_DIR/monitoring"

echo "Testing Monitoring Toolset Data Access"
echo "======================================="
echo ""

# Test 1: AlertManager Status
echo "Test 1: AlertManager Status File"
if [ -f "$MONITORING_DIR/alertmanager/status.json" ]; then
    echo "✓ AlertManager status.json exists"
    SIZE=$(stat -c%s "$MONITORING_DIR/alertmanager/status.json")
    echo "  File size: $SIZE bytes"
else
    echo "✗ AlertManager status.json NOT FOUND"
fi
echo ""

# Test 2: Prometheus TSDB Status (both replicas)
echo "Test 2: Prometheus TSDB Status Files"
for replica in 0 1; do
    TSDB_FILE="$MONITORING_DIR/prometheus/prometheus-k8s-$replica/status/tsdb.json"
    if [ -f "$TSDB_FILE" ]; then
        echo "✓ Prometheus replica $replica tsdb.json exists"
        SIZE=$(stat -c%s "$TSDB_FILE")
        echo "  File size: $SIZE bytes"
        # Extract series count
        SERIES=$(jq -r '.headStats.numSeries' "$TSDB_FILE" 2>/dev/null || echo "N/A")
        echo "  Series count: $SERIES"
    else
        echo "✗ Prometheus replica $replica tsdb.json NOT FOUND"
    fi
done
echo ""

# Test 3: Prometheus Runtime Info
echo "Test 3: Prometheus Runtime Info Files"
for replica in 0 1; do
    RUNTIME_FILE="$MONITORING_DIR/prometheus/prometheus-k8s-$replica/status/runtimeinfo.json"
    if [ -f "$RUNTIME_FILE" ]; then
        echo "✓ Prometheus replica $replica runtimeinfo.json exists"
        RETENTION=$(jq -r '.storageRetention' "$RUNTIME_FILE" 2>/dev/null || echo "N/A")
        echo "  Storage retention: $RETENTION"
    else
        echo "✗ Prometheus replica $replica runtimeinfo.json NOT FOUND"
    fi
done
echo ""

# Test 4: Active Targets
echo "Test 4: Prometheus Active Targets Files"
for replica in 0 1; do
    TARGETS_FILE="$MONITORING_DIR/prometheus/prometheus-k8s-$replica/active-targets.json"
    if [ -f "$TARGETS_FILE" ]; then
        echo "✓ Prometheus replica $replica active-targets.json exists"
        SIZE=$(stat -c%s "$TARGETS_FILE")
        echo "  File size: $SIZE bytes"
        TARGET_COUNT=$(jq '.activeTargets | length' "$TARGETS_FILE" 2>/dev/null || echo "N/A")
        echo "  Target count: $TARGET_COUNT"
    else
        echo "✗ Prometheus replica $replica active-targets.json NOT FOUND"
    fi
done
echo ""

# Test 5: Rules
echo "Test 5: Prometheus Rules File"
RULES_FILE="$MONITORING_DIR/prometheus/rules.json"
if [ -f "$RULES_FILE" ]; then
    echo "✓ Prometheus rules.json exists"
    SIZE=$(stat -c%s "$RULES_FILE")
    echo "  File size: $SIZE bytes"
    GROUP_COUNT=$(jq '.groups | length' "$RULES_FILE" 2>/dev/null || echo "N/A")
    echo "  Rule groups: $GROUP_COUNT"
else
    echo "✗ Prometheus rules.json NOT FOUND"
fi
echo ""

# Test 6: Config and Flags
echo "Test 6: Prometheus Config and Flags"
CONFIG_FILE="$MONITORING_DIR/prometheus/status/config.json"
FLAGS_FILE="$MONITORING_DIR/prometheus/status/flags.json"

if [ -f "$CONFIG_FILE" ]; then
    echo "✓ Prometheus config.json exists"
    SIZE=$(stat -c%s "$CONFIG_FILE")
    echo "  File size: $SIZE bytes"
else
    echo "✗ Prometheus config.json NOT FOUND"
fi

if [ -f "$FLAGS_FILE" ]; then
    echo "✓ Prometheus flags.json exists"
    SIZE=$(stat -c%s "$FLAGS_FILE")
    echo "  File size: $SIZE bytes"
else
    echo "✗ Prometheus flags.json NOT FOUND"
fi
echo ""

echo "======================================="
echo "Verification complete!"
echo ""
echo "To test the actual MCP tools, run:"
echo "./_output/bin/must-gather-mcp-server --must-gather-path $MG_PATH"
