//go:build !windows
// +build !windows

package system

import (
	"net"
	"runtime"
	"sync"
	"time"
)

type OtherCollector struct {
	mu        sync.Mutex
	startTime time.Time
}

func NewCollector() Collector {
	return &OtherCollector{
		startTime: time.Now(),
	}
}

func (c *OtherCollector) GetSystemMetrics() (SystemMetrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemMetrics{
		Timestamp:      time.Now().UTC(),
		UptimeSec:      uint64(time.Since(c.startTime).Seconds()),
		AgentUptimeSec: uint64(time.Since(c.startTime).Seconds()),
		CPUPercent:     0.0,
		MemoryTotalMB:  m.Sys / (1024 * 1024),
		MemoryUsedMB:   m.Alloc / (1024 * 1024),
		MemoryPercent:  float64(m.Alloc) / float64(m.Sys+1) * 100.0,
	}, nil
}

func (c *OtherCollector) GetInterfaceCounters() ([]InterfaceCounters, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var results []InterfaceCounters
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		counters := InterfaceCounters{
			InterfaceName:   iface.Name,
			MACAddress:      iface.HardwareAddr.String(),
			OperationalStat: iface.Flags.String(),
		}
		addrs, _ := iface.Addrs()
		if len(addrs) > 0 {
			counters.IsDefaultRoute = true
		}
		results = append(results, counters)
	}

	return results, nil
}

func (c *OtherCollector) GetPrimaryInterfaceCounters() (InterfaceCounters, error) {
	all, err := c.GetInterfaceCounters()
	if err != nil {
		return InterfaceCounters{}, err
	}
	if len(all) > 0 {
		return all[0], nil
	}
	return InterfaceCounters{InterfaceName: "default"}, nil
}
