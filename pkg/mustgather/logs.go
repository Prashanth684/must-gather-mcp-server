package mustgather

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/must-gather-mcp-server/pkg/api"
)

// GetPodLog retrieves pod container logs
func (p *Provider) GetPodLog(opts api.PodLogOptions) (string, error) {
	containerDir, err := findContainerDir(p.path)
	if err != nil {
		containerDir = p.path
	}

	// Construct log path: namespaces/{ns}/pods/{pod}/{container}/{container}/logs/{logtype}.log
	logFile := string(opts.LogType) + ".log"
	logPath := filepath.Join(
		containerDir,
		"namespaces",
		opts.Namespace,
		"pods",
		opts.Pod,
		opts.Container,
		opts.Container, // Container name appears twice in path
		"logs",
		logFile,
	)

	// Check if file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return "", fmt.Errorf("log file not found: %s", logPath)
	}

	// Read the log file
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to read log file: %w", err)
	}

	content := string(data)

	// Apply tail limit if specified
	if opts.TailLines > 0 {
		content = TailLines(content, opts.TailLines)
	}

	return content, nil
}

// ListPodContainers lists all containers for a pod
func (p *Provider) ListPodContainers(namespace, pod string) ([]string, error) {
	containerDir, err := findContainerDir(p.path)
	if err != nil {
		containerDir = p.path
	}

	podsDir := filepath.Join(containerDir, "namespaces", namespace, "pods", pod)

	// Check if pod directory exists
	if _, err := os.Stat(podsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("pod directory not found: %s/%s", namespace, pod)
	}

	// List subdirectories (containers)
	entries, err := os.ReadDir(podsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read pod directory: %w", err)
	}

	containers := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			// Check if it has logs subdirectory
			logsDir := filepath.Join(podsDir, name, name, "logs")
			if _, err := os.Stat(logsDir); err == nil {
				containers = append(containers, name)
			}
		}
	}

	return containers, nil
}

// GetNodeDiagnostics retrieves node diagnostic information
func (p *Provider) GetNodeDiagnostics(nodeName string) (*api.NodeDiagnostics, error) {
	containerDir, err := findContainerDir(p.path)
	if err != nil {
		containerDir = p.path
	}

	nodeDir := filepath.Join(containerDir, "nodes", nodeName)

	// Check if node directory exists
	if _, err := os.Stat(nodeDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("node directory not found: %s", nodeName)
	}

	diag := &api.NodeDiagnostics{
		NodeName: nodeName,
	}

	// Read kubelet log (gzipped)
	kubeletLogPath := filepath.Join(nodeDir, nodeName+"_logs_kubelet.gz")
	if content, err := readGzipFile(kubeletLogPath); err == nil {
		diag.KubeletLog = content
	}

	// Read sysinfo.log
	if content, err := readTextFile(filepath.Join(nodeDir, "sysinfo.log")); err == nil {
		diag.SysInfo = content
	}

	// Read JSON files
	if content, err := readTextFile(filepath.Join(nodeDir, "cpu_affinities.json")); err == nil {
		diag.CPUAffinities = content
	}

	if content, err := readTextFile(filepath.Join(nodeDir, "irq_affinities.json")); err == nil {
		diag.IRQAffinities = content
	}

	if content, err := readTextFile(filepath.Join(nodeDir, "pods_info.json")); err == nil {
		diag.PodsInfo = content
	}

	if content, err := readTextFile(filepath.Join(nodeDir, "podresources.json")); err == nil {
		diag.PodResources = content
	}

	// Read system info files
	if content, err := readTextFile(filepath.Join(nodeDir, "lscpu")); err == nil {
		diag.Lscpu = content
	}

	if content, err := readTextFile(filepath.Join(nodeDir, "lspci")); err == nil {
		diag.Lspci = content
	}

	if content, err := readTextFile(filepath.Join(nodeDir, "dmesg")); err == nil {
		diag.Dmesg = content
	}

	if content, err := readTextFile(filepath.Join(nodeDir, "proc_cmdline")); err == nil {
		diag.ProcCmdline = content
	}

	return diag, nil
}

// ListNodes lists all nodes in the must-gather
func (p *Provider) ListNodes() ([]string, error) {
	containerDir, err := findContainerDir(p.path)
	if err != nil {
		containerDir = p.path
	}

	nodesDir := filepath.Join(containerDir, "nodes")

	// Check if nodes directory exists
	if _, err := os.Stat(nodesDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(nodesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read nodes directory: %w", err)
	}

	nodes := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			nodes = append(nodes, entry.Name())
		}
	}

	return nodes, nil
}

// Helper functions

func readTextFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func readGzipFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, gz); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// TailLines returns the last n lines from the content
func TailLines(content string, n int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= n {
		return content
	}

	return strings.Join(lines[len(lines)-n:], "\n")
}

// tailLinesFromGzip reads the last n lines from a gzipped file efficiently
func tailLinesFromGzip(path string, n int) (string, error) {
	// For simplicity, decompress entire file and tail
	// In production, you might want to optimize this
	content, err := readGzipFile(path)
	if err != nil {
		return "", err
	}

	if n == 0 {
		return content, nil
	}

	// Get last n lines
	scanner := bufio.NewScanner(strings.NewReader(content))
	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}

	return strings.Join(lines, "\n"), nil
}

// ListHostServiceLogs lists all available host service log files
func (p *Provider) ListHostServiceLogs() ([]string, error) {
	containerDir, err := findContainerDir(p.path)
	if err != nil {
		containerDir = p.path
	}

	// Check for masters directory (primary location)
	mastersDir := filepath.Join(containerDir, "host_service_logs", "masters")
	services := make([]string, 0)

	if _, err := os.Stat(mastersDir); err == nil {
		entries, err := os.ReadDir(mastersDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read host_service_logs directory: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
				// Remove _service.log suffix to get service name
				serviceName := strings.TrimSuffix(entry.Name(), "_service.log")
				services = append(services, serviceName)
			}
		}
	}

	return services, nil
}

// GetHostServiceLog retrieves a specific host service log
func (p *Provider) GetHostServiceLog(serviceName string, tailLines int) (string, error) {
	containerDir, err := findContainerDir(p.path)
	if err != nil {
		containerDir = p.path
	}

	// Construct log path
	logFile := serviceName + "_service.log"
	logPath := filepath.Join(containerDir, "host_service_logs", "masters", logFile)

	// Check if file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return "", fmt.Errorf("service log not found: %s", serviceName)
	}

	// Read the log file
	content, err := readTextFile(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to read service log: %w", err)
	}

	// Apply tail limit if specified
	if tailLines > 0 {
		content = TailLines(content, tailLines)
	}

	return content, nil
}

// ListStaticPodTerminationLogs lists available static pod termination logs
func (p *Provider) ListStaticPodTerminationLogs() (map[string][]string, error) {
	containerDir, err := findContainerDir(p.path)
	if err != nil {
		containerDir = p.path
	}

	staticPodsDir := filepath.Join(containerDir, "static-pods")
	result := make(map[string][]string)

	// Check if directory exists
	if _, err := os.Stat(staticPodsDir); os.IsNotExist(err) {
		return result, nil
	}

	// List pod type directories (e.g., kube-apiserver, etcd)
	podTypes, err := os.ReadDir(staticPodsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read static-pods directory: %w", err)
	}

	for _, podType := range podTypes {
		if !podType.IsDir() {
			continue
		}

		podTypeDir := filepath.Join(staticPodsDir, podType.Name())
		logs, err := os.ReadDir(podTypeDir)
		if err != nil {
			continue
		}

		nodeTerminationLogs := []string{}
		for _, log := range logs {
			if !log.IsDir() && strings.HasSuffix(log.Name(), "-termination.log.gz") {
				// Extract node name from filename
				nodeName := strings.TrimSuffix(log.Name(), "-termination.log.gz")
				nodeTerminationLogs = append(nodeTerminationLogs, nodeName)
			}
		}

		if len(nodeTerminationLogs) > 0 {
			result[podType.Name()] = nodeTerminationLogs
		}
	}

	return result, nil
}

// GetStaticPodTerminationLog retrieves termination log for a static pod
func (p *Provider) GetStaticPodTerminationLog(podType, nodeName string) (string, error) {
	containerDir, err := findContainerDir(p.path)
	if err != nil {
		containerDir = p.path
	}

	logFile := nodeName + "-termination.log.gz"
	logPath := filepath.Join(containerDir, "static-pods", podType, logFile)

	// Check if file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return "", fmt.Errorf("termination log not found for %s on node %s", podType, nodeName)
	}

	// Read and decompress
	content, err := readGzipFile(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to read termination log: %w", err)
	}

	return content, nil
}
