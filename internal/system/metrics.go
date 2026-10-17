package system

import (
	"time"
)

// InterfaceCounters tracks raw network device octets and packet counters.
type InterfaceCounters struct {
	InterfaceName   string `json:"interface_name"`
	MACAddress      string `json:"mac_address"`
	BytesSent       uint64 `json:"bytes_sent"`
	BytesRecv       uint64 `json:"bytes_recv"`
	PacketsSent     uint64 `json:"packets_sent"`
	PacketsRecv     uint64 `json:"packets_recv"`
	ErrorsIn        uint64 `json:"errors_in"`
	ErrorsOut       uint64 `json:"errors_out"`
	DropsIn         uint64 `json:"drops_in"`
	DropsOut        uint64 `json:"drops_out"`
	IsDefaultRoute  bool   `json:"is_default_route"`
	OperationalStat string `json:"operational_status"`
}

// SystemMetrics tracks host resources.
type SystemMetrics struct {
	Timestamp      time.Time `json:"timestamp"`
	UptimeSec      uint64    `json:"uptime_sec"`
	AgentUptimeSec uint64    `json:"agent_uptime_sec"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryTotalMB  uint64    `json:"memory_total_mb"`
	MemoryUsedMB   uint64    `json:"memory_used_mb"`
	MemoryPercent  float64   `json:"memory_percent"`
}

// Collector defines the interface for collecting system telemetry.
type Collector interface {
	GetSystemMetrics() (SystemMetrics, error)
	GetInterfaceCounters() ([]InterfaceCounters, error)
	GetPrimaryInterfaceCounters() (InterfaceCounters, error)
}
