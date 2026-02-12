package api

// LogType represents the type of log
type LogType string

const (
	LogTypeCurrent          LogType = "current"
	LogTypePrevious         LogType = "previous"
	LogTypePreviousInsecure LogType = "previous.insecure"
)

// PodLogOptions contains options for retrieving pod logs
type PodLogOptions struct {
	Namespace string
	Pod       string
	Container string
	LogType   LogType
	TailLines int  // Number of lines from end (0 = all)
	Follow    bool // Not applicable for must-gather (always false)
}

// NodeDiagnostics contains node diagnostic information
type NodeDiagnostics struct {
	NodeName      string
	KubeletLog    string // Decompressed kubelet log
	SysInfo       string
	CPUAffinities string
	IRQAffinities string
	PodsInfo      string
	PodResources  string
	Lscpu         string
	Lspci         string
	Dmesg         string
	ProcCmdline   string
}
