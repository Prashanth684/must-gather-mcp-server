package diagnostics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openshift/must-gather-mcp-server/pkg/api"
)

func etcdExtendedTools() []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: api.Tool{
				Name:        "etcd_members_list",
				Description: "Get ETCD cluster member information including IDs, peer URLs, and client URLs",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: etcdMembersList,
		},
		{
			Tool: api.Tool{
				Name:        "etcd_endpoint_status",
				Description: "Get detailed ETCD endpoint status including DB size, leader info, raft state, and quota usage",
				InputSchema: &jsonschema.Schema{
					Type: "object",
				},
			},
			Handler: etcdEndpointStatus,
		},
	}
}

func etcdMembersList(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	// Find container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	memberFile := filepath.Join(containerDir, "etcd_info", "member_list.json")

	// Check if file exists
	if _, err := os.Stat(memberFile); os.IsNotExist(err) {
		return api.NewToolCallResult("", fmt.Errorf("ETCD member list not found")), nil
	}

	// Read and parse JSON
	data, err := os.ReadFile(memberFile)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to read ETCD member list: %w", err)), nil
	}

	var memberList struct {
		Header struct {
			ClusterID uint64 `json:"cluster_id"`
			MemberID  uint64 `json:"member_id"`
			RaftTerm  int    `json:"raft_term"`
		} `json:"header"`
		Members []struct {
			ID         uint64   `json:"ID"`
			Name       string   `json:"name"`
			PeerURLs   []string `json:"peerURLs"`
			ClientURLs []string `json:"clientURLs"`
		} `json:"members"`
	}

	if err := json.Unmarshal(data, &memberList); err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to parse ETCD member list: %w", err)), nil
	}

	output := "ETCD Cluster Members\n"
	output += strings.Repeat("=", 80) + "\n\n"

	output += fmt.Sprintf("Cluster ID: %d\n", memberList.Header.ClusterID)
	output += fmt.Sprintf("Current Member ID: %d\n", memberList.Header.MemberID)
	output += fmt.Sprintf("Raft Term: %d\n", memberList.Header.RaftTerm)
	output += fmt.Sprintf("Total Members: %d\n\n", len(memberList.Members))

	output += strings.Repeat("-", 80) + "\n"

	for i, member := range memberList.Members {
		output += fmt.Sprintf("\nMember %d:\n", i+1)
		output += fmt.Sprintf("  Name: %s\n", member.Name)
		output += fmt.Sprintf("  ID: %d\n", member.ID)
		output += fmt.Sprintf("  Peer URLs: %s\n", strings.Join(member.PeerURLs, ", "))
		output += fmt.Sprintf("  Client URLs: %s\n", strings.Join(member.ClientURLs, ", "))
	}

	return api.NewToolCallResult(output, nil), nil
}

func etcdEndpointStatus(params api.ToolHandlerParams) (*api.ToolCallResult, error) {
	// Find container directory
	containerDir, err := findContainerDir(params.MustGatherProvider.GetMetadata().Path)
	if err != nil {
		containerDir = params.MustGatherProvider.GetMetadata().Path
	}

	statusFile := filepath.Join(containerDir, "etcd_info", "endpoint_status.json")

	// Check if file exists
	if _, err := os.Stat(statusFile); os.IsNotExist(err) {
		return api.NewToolCallResult("", fmt.Errorf("ETCD endpoint status not found")), nil
	}

	// Read and parse JSON
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to read ETCD endpoint status: %w", err)), nil
	}

	var statuses []struct {
		Endpoint string `json:"Endpoint"`
		Status   struct {
			Header struct {
				ClusterID uint64 `json:"cluster_id"`
				MemberID  uint64 `json:"member_id"`
				Revision  int64  `json:"revision"`
				RaftTerm  int    `json:"raft_term"`
			} `json:"header"`
			Version          string `json:"version"`
			DBSize           int64  `json:"dbSize"`
			Leader           uint64 `json:"leader"`
			RaftIndex        int64  `json:"raftIndex"`
			RaftTerm         int    `json:"raftTerm"`
			RaftAppliedIndex int64  `json:"raftAppliedIndex"`
			DBSizeInUse      int64  `json:"dbSizeInUse"`
			StorageVersion   string `json:"storageVersion"`
			DBSizeQuota      int64  `json:"dbSizeQuota"`
		} `json:"Status"`
	}

	if err := json.Unmarshal(data, &statuses); err != nil {
		return api.NewToolCallResult("", fmt.Errorf("failed to parse ETCD endpoint status: %w", err)), nil
	}

	output := "ETCD Endpoint Status\n"
	output += strings.Repeat("=", 80) + "\n\n"

	output += fmt.Sprintf("Total Endpoints: %d\n\n", len(statuses))

	// Find leader
	var leaderID uint64
	if len(statuses) > 0 {
		leaderID = statuses[0].Status.Leader
	}

	for i, status := range statuses {
		isLeader := (status.Status.Header.MemberID == leaderID)
		leaderMarker := ""
		if isLeader {
			leaderMarker = " (LEADER)"
		}

		output += fmt.Sprintf("Endpoint %d: %s%s\n", i+1, status.Endpoint, leaderMarker)
		output += strings.Repeat("-", 80) + "\n"

		output += fmt.Sprintf("  Member ID: %d\n", status.Status.Header.MemberID)
		output += fmt.Sprintf("  Version: %s\n", status.Status.Version)
		output += fmt.Sprintf("  Storage Version: %s\n", status.Status.StorageVersion)
		output += "\n"

		// Database info
		dbSizeMB := float64(status.Status.DBSize) / (1024 * 1024)
		dbSizeInUseMB := float64(status.Status.DBSizeInUse) / (1024 * 1024)
		dbQuotaGB := float64(status.Status.DBSizeQuota) / (1024 * 1024 * 1024)
		usagePercent := float64(status.Status.DBSizeInUse) / float64(status.Status.DBSizeQuota) * 100

		output += "  Database:\n"
		output += fmt.Sprintf("    Size: %.2f MB\n", dbSizeMB)
		output += fmt.Sprintf("    In Use: %.2f MB\n", dbSizeInUseMB)
		output += fmt.Sprintf("    Quota: %.2f GB\n", dbQuotaGB)
		output += fmt.Sprintf("    Usage: %.2f%%\n", usagePercent)

		if usagePercent > 80 {
			output += "    ⚠ WARNING: Database usage is above 80%\n"
		}
		output += "\n"

		// Raft info
		output += "  Raft:\n"
		output += fmt.Sprintf("    Term: %d\n", status.Status.RaftTerm)
		output += fmt.Sprintf("    Index: %d\n", status.Status.RaftIndex)
		output += fmt.Sprintf("    Applied Index: %d\n", status.Status.RaftAppliedIndex)
		output += fmt.Sprintf("    Revision: %d\n", status.Status.Header.Revision)

		lagBehind := status.Status.RaftIndex - status.Status.RaftAppliedIndex
		if lagBehind > 0 {
			output += fmt.Sprintf("    ⚠ Lag: %d (index - applied index)\n", lagBehind)
		}

		output += "\n"
	}

	// Summary
	output += strings.Repeat("=", 80) + "\n"
	output += "Summary\n"
	output += strings.Repeat("=", 80) + "\n\n"

	// Calculate total DB size
	var totalDBSize int64
	var totalDBInUse int64
	for _, status := range statuses {
		totalDBSize += status.Status.DBSize
		totalDBInUse += status.Status.DBSizeInUse
	}

	avgDBSizeMB := float64(totalDBSize) / float64(len(statuses)) / (1024 * 1024)
	avgDBInUseMB := float64(totalDBInUse) / float64(len(statuses)) / (1024 * 1024)

	output += fmt.Sprintf("Average DB Size: %.2f MB\n", avgDBSizeMB)
	output += fmt.Sprintf("Average DB In Use: %.2f MB\n", avgDBInUseMB)
	output += fmt.Sprintf("Leader ID: %d\n", leaderID)

	return api.NewToolCallResult(output, nil), nil
}

// Helper function
func findContainerDir(basePath string) (string, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, "quay") || strings.Contains(name, "sha256") {
				return filepath.Join(basePath, name), nil
			}
		}
	}

	return "", fmt.Errorf("container directory not found")
}
