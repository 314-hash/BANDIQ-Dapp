package system

import (
	"testing"
)

func TestCollectorSystemMetrics(t *testing.T) {
	collector := NewCollector()
	metrics, err := collector.GetSystemMetrics()
	if err != nil {
		t.Fatalf("failed to get system metrics: %v", err)
	}

	if metrics.MemoryTotalMB == 0 {
		t.Logf("warning: memory total is 0")
	}

	t.Logf("System Metrics: Uptime=%ds, AgentUptime=%ds, CPU=%.2f%%, MemoryTotal=%dMB, MemoryUsed=%dMB",
		metrics.UptimeSec, metrics.AgentUptimeSec, metrics.CPUPercent, metrics.MemoryTotalMB, metrics.MemoryUsedMB)
}

func TestCollectorInterfaceCounters(t *testing.T) {
	collector := NewCollector()
	counters, err := collector.GetInterfaceCounters()
	if err != nil {
		t.Fatalf("failed to get interface counters: %v", err)
	}

	t.Logf("Found %d non-loopback network interfaces", len(counters))
	for _, iface := range counters {
		t.Logf("Interface: %s (MAC: %s, In: %d bytes, Out: %d bytes, Status: %s)",
			iface.InterfaceName, iface.MACAddress, iface.BytesRecv, iface.BytesSent, iface.OperationalStat)
	}

	primary, err := collector.GetPrimaryInterfaceCounters()
	if err != nil {
		t.Fatalf("failed to get primary interface counters: %v", err)
	}
	t.Logf("Primary interface: %s (BytesIn=%d, BytesOut=%d)", primary.InterfaceName, primary.BytesRecv, primary.BytesSent)
}
