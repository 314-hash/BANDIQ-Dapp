//go:build windows
// +build windows

package system

import (
	"fmt"
	"net"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modiphlpapi     = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetIfEntry2 = modiphlpapi.NewProc("GetIfEntry2")

	modKernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGetTickCount64       = modKernel32.NewProc("GetTickCount64")
	procGetSystemTimes       = modKernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = modKernel32.NewProc("GlobalMemoryStatusEx")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

type filetime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

// MIB_IF_ROW2 structure corresponding to Windows IP Helper API.
type mibIfRow2 struct {
	InterfaceLuid               uint64
	InterfaceIndex              uint32
	InterfaceGuid               windows.GUID
	Alias                       [257]uint16
	Description                 [257]uint16
	PhysicalAddressLength       uint32
	PhysicalAddress             [32]byte
	PermanentPhysicalAddress    [32]byte
	Mtu                         uint32
	Type                        uint32
	TunnelType                  uint32
	MediaType                   uint32
	PhysicalMediumType          uint32
	AccessType                  uint32
	DirectionType               uint32
	InterfaceAndOperStatusFlags uint32
	OperStatus                  uint32
	AdminStatus                 uint32
	MediaConnectState           uint32
	NetworkGuid                 windows.GUID
	ConnectionType              uint32
	_pad                        uint32
	TransmitLinkSpeed           uint64
	ReceiveLinkSpeed            uint64
	InOctets                    uint64
	InUcastPkts                 uint64
	InNUcastPkts                uint64
	InDiscards                  uint64
	InErrors                    uint64
	InUnknownProtos             uint64
	InUcastOctets               uint64
	InMulticastOctets           uint64
	InBroadcastOctets           uint64
	OutOctets                   uint64
	OutUcastPkts                uint64
	OutNUcastPkts               uint64
	OutDiscards                 uint64
	OutErrors                   uint64
	OutUcastOctets              uint64
	OutMulticastOctets          uint64
	OutBroadcastOctets          uint64
	OutQLen                     uint64
}

// WindowsCollector implements Collector for Windows systems.
type WindowsCollector struct {
	mu           sync.Mutex
	startTime    time.Time
	prevIdle     filetime
	prevKernel   filetime
	prevUser     filetime
	hasSample    bool
	lastCPUUsage float64
}

// NewCollector creates a Windows-native metrics collector.
func NewCollector() Collector {
	c := &WindowsCollector{
		startTime: time.Now(),
	}
	// Initial sample for CPU diffing
	ret, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&c.prevIdle)),
		uintptr(unsafe.Pointer(&c.prevKernel)),
		uintptr(unsafe.Pointer(&c.prevUser)),
	)
	if ret != 0 {
		c.hasSample = true
	}
	return c
}

func filetimeToUint64(ft filetime) uint64 {
	return (uint64(ft.HighDateTime) << 32) | uint64(ft.LowDateTime)
}

func (c *WindowsCollector) GetSystemMetrics() (SystemMetrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	metrics := SystemMetrics{
		Timestamp:      time.Now().UTC(),
		AgentUptimeSec: uint64(time.Since(c.startTime).Seconds()),
	}

	// Host uptime from GetTickCount64
	r1, _, _ := procGetTickCount64.Call()
	if r1 > 0 {
		metrics.UptimeSec = uint64(r1) / 1000
	}

	// Memory usage via GlobalMemoryStatusEx
	var mem memoryStatusEx
	mem.Length = uint32(unsafe.Sizeof(mem))
	rMem, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))
	if rMem != 0 {
		metrics.MemoryTotalMB = mem.TotalPhys / (1024 * 1024)
		metrics.MemoryUsedMB = (mem.TotalPhys - mem.AvailPhys) / (1024 * 1024)
		metrics.MemoryPercent = float64(mem.MemoryLoad)
	}

	// CPU calculation via GetSystemTimes delta
	var idle, kernel, user filetime
	rCpu, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if rCpu != 0 && c.hasSample {
		idleNow := filetimeToUint64(idle)
		kernelNow := filetimeToUint64(kernel)
		userNow := filetimeToUint64(user)

		idlePrev := filetimeToUint64(c.prevIdle)
		kernelPrev := filetimeToUint64(c.prevKernel)
		userPrev := filetimeToUint64(c.prevUser)

		idleDiff := idleNow - idlePrev
		kernelDiff := kernelNow - kernelPrev
		userDiff := userNow - userPrev
		totalDiff := kernelDiff + userDiff

		if totalDiff > 0 && totalDiff >= idleDiff {
			busy := totalDiff - idleDiff
			metrics.CPUPercent = (float64(busy) / float64(totalDiff)) * 100.0
			c.lastCPUUsage = metrics.CPUPercent
		} else {
			metrics.CPUPercent = c.lastCPUUsage
		}

		c.prevIdle = idle
		c.prevKernel = kernel
		c.prevUser = user
	}

	return metrics, nil
}

func (c *WindowsCollector) GetInterfaceCounters() ([]InterfaceCounters, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	var results []InterfaceCounters
	for _, iface := range ifaces {
		// Ignore loopback interfaces
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		var row mibIfRow2
		row.InterfaceIndex = uint32(iface.Index)

		ret, _, _ := procGetIfEntry2.Call(uintptr(unsafe.Pointer(&row)))
		if ret != 0 {
			results = append(results, InterfaceCounters{
				InterfaceName:   iface.Name,
				MACAddress:      iface.HardwareAddr.String(),
				OperationalStat: iface.Flags.String(),
			})
			continue
		}

		operStatus := "UNKNOWN"
		switch row.OperStatus {
		case 1:
			operStatus = "UP"
		case 2:
			operStatus = "DOWN"
		case 3:
			operStatus = "TESTING"
		}

		counters := InterfaceCounters{
			InterfaceName:   iface.Name,
			MACAddress:      iface.HardwareAddr.String(),
			BytesSent:       row.OutOctets,
			BytesRecv:       row.InOctets,
			PacketsSent:     row.OutUcastPkts + row.OutNUcastPkts,
			PacketsRecv:     row.InUcastPkts + row.InNUcastPkts,
			ErrorsIn:        row.InErrors,
			ErrorsOut:       row.OutErrors,
			DropsIn:         row.InDiscards,
			DropsOut:        row.OutDiscards,
			OperationalStat: operStatus,
		}

		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				counters.IsDefaultRoute = true
				break
			}
		}

		results = append(results, counters)
	}

	return results, nil
}

func (c *WindowsCollector) GetPrimaryInterfaceCounters() (InterfaceCounters, error) {
	all, err := c.GetInterfaceCounters()
	if err != nil {
		return InterfaceCounters{}, err
	}

	for _, iface := range all {
		if iface.IsDefaultRoute && (iface.BytesRecv > 0 || iface.BytesSent > 0) {
			return iface, nil
		}
	}

	for _, iface := range all {
		if iface.BytesRecv > 0 || iface.BytesSent > 0 {
			return iface, nil
		}
	}

	if len(all) > 0 {
		return all[0], nil
	}

	return InterfaceCounters{InterfaceName: "default"}, nil
}
