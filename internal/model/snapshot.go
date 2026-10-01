// Fields are declared in sorted JSON-tag order so marshaled output matches the map-based generator byte for byte (T3a proof). Do not reorder.

// Package model holds the marvin/v1 snapshot types (docs/SCHEMA.md) and the
// shared derivation rules: color bands, severity, Fahrenheit, display names and the
// fixed-capacity ring buffer.
package model

// SchemaVersion is the value of Snapshot.Schema on every snapshot.
const SchemaVersion = "marvin/v1"

// Snapshot is one marvin/v1 document as served at GET /snapshot.json.
type Snapshot struct {
	Connections Connections `json:"connections"`
	CPU         CPU         `json:"cpu"`
	DiskIO      DiskIO      `json:"disk_io"`
	Fans        []Fan       `json:"fans"`
	GeneratedAt string      `json:"generated_at"`
	GPULine     string      `json:"gpu_line"`
	GPUs        []GPU       `json:"gpus"`
	Host        Host        `json:"host"`
	Memory      Memory      `json:"memory"`
	Mood        string      `json:"mood"`
	Network     []Net       `json:"network"`
	PanicCount  *int        `json:"panic_count"`
	Phrase      Phrase      `json:"phrase"`
	Scenario    string      `json:"scenario"`
	Schema      string      `json:"schema"`
	Smart       Smart       `json:"smart"`
	Storage     []Mount     `json:"storage"`
	Temps       Temps       `json:"temps"`
}

// Connections is the established TCP connection count (D-036).
type Connections struct {
	Established int `json:"established"`
}

// CPU is the PROCESSOR box.
type CPU struct {
	Band          Band       `json:"band"`
	FreqGHz       float64    `json:"freq_ghz"`
	HistPct       []float64  `json:"hist_pct"`
	IowaitPct     *float64   `json:"iowait_pct"`
	Load1         float64    `json:"load1"`
	Load15        float64    `json:"load15"`
	Load5         float64    `json:"load5"`
	Model         string     `json:"model"`
	ModelDisplay  string     `json:"model_display"`
	PerThreadPct  []*float64 `json:"per_thread_pct"`
	PhysicalCores int        `json:"physical_cores"`
	TempC         *float64   `json:"temp_c"`
	ThermalBand   Band       `json:"thermal_band"`
	Threads       int        `json:"threads"`
	TotalPct      *float64   `json:"total_pct"`
}

// DiskIO is the STORAGE · I/O cell for the physical NVMe (D-035).
type DiskIO struct {
	Device    string   `json:"device"`
	InFlight  int      `json:"in_flight"`
	QueueAvg  *float64 `json:"queue_avg"`
	ReadBps   *int64   `json:"read_bps"`
	ReadIOPS  *int     `json:"read_iops"`
	WriteBps  *int64   `json:"write_bps"`
	WriteIOPS *int     `json:"write_iops"`
}

// Fan is one FAN BANK entry; rpm null is real telemetry (D-027).
type Fan struct {
	Bank    int    `json:"bank"`
	Label   string `json:"label"`
	MaxRPM  *int   `json:"max_rpm"`
	RPM     *int   `json:"rpm"`
	Verdict string `json:"verdict"`
}

// GPU is one Nvidia card from nvidia-smi, matched by UUID.
type GPU struct {
	DisplayName string    `json:"display_name"`
	HistUtilPct []float64 `json:"hist_util_pct"`
	MemTotalMiB int       `json:"mem_total_mib"`
	MemUsedMiB  int       `json:"mem_used_mib"`
	Name        string    `json:"name"`
	PowerLimitW float64   `json:"power_limit_w"`
	PowerW      *float64  `json:"power_w"`
	TempC       *float64  `json:"temp_c"`
	ThermalBand Band      `json:"thermal_band"`
	UtilPct     *float64  `json:"util_pct"`
	UUID        string    `json:"uuid"`
}

// Host identifies the machine.
type Host struct {
	Hostname      string `json:"hostname"`
	Kernel        string `json:"kernel"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// Memory is the MEMORY cell (D-034).
type Memory struct {
	CacheBytes     int64   `json:"cache_bytes"`
	SwapTotalBytes int64   `json:"swap_total_bytes"`
	SwapUsedBytes  int64   `json:"swap_used_bytes"`
	TotalBytes     int64   `json:"total_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	UsedPct        float64 `json:"used_pct"`
}

// Net is the single NETWORK entry (wlp14s0, D-018).
type Net struct {
	If    string `json:"if"`
	RxBps *int64 `json:"rx_bps"`
	RxErr int    `json:"rx_err"`
	TxBps *int64 `json:"tx_bps"`
	TxErr int    `json:"tx_err"`
}

// Phrase is the phrase-box text, already wrapped by marvind.
type Phrase struct {
	Lines []string `json:"lines"`
}

// Smart is the SMART status from the root-owned handoff file (D-012).
type Smart struct {
	AgeSeconds      *int   `json:"age_seconds"`
	PercentageUsed  *int   `json:"percentage_used"`
	State           string `json:"state"`
	UnsafeShutdowns *int   `json:"unsafe_shutdowns"`
}

// Mount is one SPACE row (D-019).
type Mount struct {
	Device     string  `json:"device"`
	FreeBytes  int64   `json:"free_bytes"`
	FS         string  `json:"fs"`
	Mount      string  `json:"mount"`
	State      Band    `json:"state"`
	TotalBytes int64   `json:"total_bytes"`
	UsedBytes  int64   `json:"used_bytes"`
	UsedPct    float64 `json:"used_pct"`
}

// Temps holds the THERMALS readings not owned by CPU or GPUs.
type Temps struct {
	NvmeC           *float64 `json:"nvme_c"`
	NvmeMaxC        float64  `json:"nvme_max_c"`
	NvmeSensor      string   `json:"nvme_sensor"`
	NvmeThermalBand Band     `json:"nvme_thermal_band"`
}
