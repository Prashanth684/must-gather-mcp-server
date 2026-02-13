package monitoring

// PrometheusAPIResponse wraps Prometheus API responses
type PrometheusAPIResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

// TSDBStatusResponse wraps TSDB status
type TSDBStatusResponse struct {
	Status string     `json:"status"`
	Data   TSDBStatus `json:"data"`
}

// RuntimeInfoResponse wraps runtime info
type RuntimeInfoResponse struct {
	Status string      `json:"status"`
	Data   RuntimeInfo `json:"data"`
}

// ActiveTargetsAPIResponse wraps active targets response
type ActiveTargetsAPIResponse struct {
	Status string                `json:"status"`
	Data   ActiveTargetsResponse `json:"data"`
}

// RuleGroupsAPIResponse wraps rules response
type RuleGroupsAPIResponse struct {
	Status string             `json:"status"`
	Data   RuleGroupsResponse `json:"data"`
}

// TSDBStatus represents Prometheus TSDB status data
type TSDBStatus struct {
	SeriesCountByMetricName    []MetricCount `json:"seriesCountByMetricName"`
	LabelValueCountByLabelName []LabelCount  `json:"labelValueCountByLabelName"`
	MemoryInBytesByLabelName   []LabelMemory `json:"memoryInBytesByLabelName"`
	HeadStats                  HeadStats     `json:"headStats"`
}

// MetricCount represents metric name and series count
type MetricCount struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// LabelCount represents label name and value count
type LabelCount struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// LabelMemory represents label name and memory usage
type LabelMemory struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// HeadStats represents TSDB head statistics
type HeadStats struct {
	NumSeries     int64 `json:"numSeries"`
	NumLabelPairs int64 `json:"numLabelPairs"`
	ChunkCount    int64 `json:"chunkCount"`
	MinTime       int64 `json:"minTime"`
	MaxTime       int64 `json:"maxTime"`
}

// RuntimeInfo represents Prometheus runtime information
type RuntimeInfo struct {
	StartTime           string `json:"startTime"`
	CWD                 string `json:"CWD"`
	ReloadConfigSuccess bool   `json:"reloadConfigSuccess"`
	LastConfigTime      string `json:"lastConfigTime"`
	CorruptionCount     int64  `json:"corruptionCount"`
	GoroutineCount      int64  `json:"goroutineCount"`
	GOMAXPROCS          int64  `json:"GOMAXPROCS"`
	GOGC                string `json:"GOGC"`
	GOMEMLIMIT          int64  `json:"GOMEMLIMIT"`
	StorageRetention    string `json:"storageRetention"`
}

// ActiveTargetsResponse represents the active targets API response
type ActiveTargetsResponse struct {
	ActiveTargets []ActiveTarget `json:"activeTargets"`
}

// ActiveTarget represents a single scrape target
type ActiveTarget struct {
	DiscoveredLabels   map[string]string `json:"discoveredLabels"`
	Labels             map[string]string `json:"labels"`
	ScrapePool         string            `json:"scrapePool"`
	ScrapeURL          string            `json:"scrapeUrl"`
	GlobalURL          string            `json:"globalUrl"`
	LastError          string            `json:"lastError"`
	LastScrape         string            `json:"lastScrape"`
	LastScrapeDuration float64           `json:"lastScrapeDuration"`
	Health             string            `json:"health"`
	ScrapeInterval     string            `json:"scrapeInterval"`
	ScrapeTimeout      string            `json:"scrapeTimeout"`
}

// RuleGroupsResponse represents the rules API response
type RuleGroupsResponse struct {
	Groups []RuleGroup `json:"groups"`
}

// RuleGroup represents a group of rules
type RuleGroup struct {
	Name           string  `json:"name"`
	File           string  `json:"file"`
	Rules          []Rule  `json:"rules"`
	Interval       float64 `json:"interval"`
	Limit          int64   `json:"limit"`
	EvaluationTime float64 `json:"evaluationTime"`
	LastEvaluation string  `json:"lastEvaluation"`
}

// Rule represents a single alerting or recording rule
type Rule struct {
	State          string            `json:"state,omitempty"`
	Name           string            `json:"name"`
	Query          string            `json:"query"`
	Duration       float64           `json:"duration,omitempty"`
	KeepFiringFor  float64           `json:"keepFiringFor,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	Alerts         []Alert           `json:"alerts,omitempty"`
	Health         string            `json:"health"`
	Type           string            `json:"type"`
	LastError      string            `json:"lastError,omitempty"`
	EvaluationTime float64           `json:"evaluationTime,omitempty"`
	LastEvaluation string            `json:"lastEvaluation,omitempty"`
}

// Alert represents an active alert
type Alert struct {
	Labels          map[string]string `json:"labels"`
	Annotations     map[string]string `json:"annotations"`
	State           string            `json:"state"`
	ActiveAt        string            `json:"activeAt"`
	Value           string            `json:"value"`
	KeepFiringSince string            `json:"keepFiringSince,omitempty"`
}

// AlertManagerStatus represents AlertManager status
type AlertManagerStatus struct {
	Cluster     AlertManagerCluster `json:"cluster"`
	VersionInfo VersionInfo         `json:"versionInfo"`
	Config      AlertManagerConfig  `json:"config"`
	Uptime      string              `json:"uptime"`
}

// AlertManagerCluster represents AlertManager cluster info
type AlertManagerCluster struct {
	Status string             `json:"status"`
	Peers  []AlertManagerPeer `json:"peers"`
}

// AlertManagerPeer represents a single AlertManager peer
type AlertManagerPeer struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// VersionInfo represents version information
type VersionInfo struct {
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	Branch    string `json:"branch"`
	BuildUser string `json:"buildUser"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
}

// AlertManagerConfig represents AlertManager configuration
type AlertManagerConfig struct {
	Original string `json:"original"`
}

// PrometheusConfig represents Prometheus configuration
type PrometheusConfig struct {
	YAML string `json:"yaml"`
}

// ConfigResponse wraps the config YAML
type ConfigResponse struct {
	YAML string `json:"yaml"`
}

// FlagsResponse represents Prometheus flags
type FlagsResponse map[string]string

// AlertManagersResponse represents AlertManager targets
type AlertManagersResponse struct {
	ActiveAlertmanagers  []AlertManagerTarget `json:"activeAlertmanagers"`
	DroppedAlertmanagers []AlertManagerTarget `json:"droppedAlertmanagers"`
}

// AlertManagerTarget represents an AlertManager target
type AlertManagerTarget struct {
	URL string `json:"url"`
}
