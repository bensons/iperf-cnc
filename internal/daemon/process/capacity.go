package process

import (
	"fmt"
	"net"
	"runtime"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// Capacity represents system resource capacity
type Capacity struct {
	CPUCores           int
	AvailableMemory    uint64
	NetworkInterfaces  []string
	MaxProcesses       int
	AvailableProcesses int
}

// CapacityCalculator calculates system resource capacity
type CapacityCalculator struct {
	maxProcesses int
	usedSlots    int
}

// NewCapacityCalculator creates a new capacity calculator
func NewCapacityCalculator(maxProcesses int) *CapacityCalculator {
	return &CapacityCalculator{
		maxProcesses: maxProcesses,
		usedSlots:    0,
	}
}

// DetectCapacity detects current system capacity
func (c *CapacityCalculator) DetectCapacity() (*Capacity, error) {
	cpuCores := runtime.NumCPU()

	// Get memory info
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}

	// Get network interfaces
	interfaces, err := getNetworkInterfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %w", err)
	}

	// Calculate max processes if not configured
	maxProcs := c.maxProcesses
	if maxProcs == 0 {
		maxProcs = calculateMaxProcesses(cpuCores)
	}

	return &Capacity{
		CPUCores:           cpuCores,
		AvailableMemory:    vmStat.Available,
		NetworkInterfaces:  interfaces,
		MaxProcesses:       maxProcs,
		AvailableProcesses: maxProcs - c.usedSlots,
	}, nil
}

// ReserveSlots reserves process slots
func (c *CapacityCalculator) ReserveSlots(count int) error {
	if c.usedSlots+count > c.maxProcesses {
		return fmt.Errorf("insufficient capacity: need %d slots, have %d available",
			count, c.maxProcesses-c.usedSlots)
	}
	c.usedSlots += count
	return nil
}

// ReleaseSlots releases process slots
func (c *CapacityCalculator) ReleaseSlots(count int) {
	c.usedSlots -= count
	if c.usedSlots < 0 {
		c.usedSlots = 0
	}
}

// GetAvailableSlots returns the number of available process slots
func (c *CapacityCalculator) GetAvailableSlots() int {
	return c.maxProcesses - c.usedSlots
}

// GetUsedSlots returns the number of used process slots
func (c *CapacityCalculator) GetUsedSlots() int {
	return c.usedSlots
}

// getNetworkInterfaces returns a list of active network interface names
func getNetworkInterfaces() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var result []string
	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		result = append(result, iface.Name)
	}

	return result, nil
}

// calculateMaxProcesses calculates max processes based on CPU cores
// For small clusters (N < 100): processes = N * 2
// For large clusters: processes = min(CPU_cores * 4, N)
func calculateMaxProcesses(cpuCores int) int {
	// Conservative default: 4 processes per core
	return cpuCores * 4
}

// GetCPUUsage returns current CPU usage percentage
func GetCPUUsage() (float64, error) {
	percentages, err := cpu.Percent(0, false)
	if err != nil {
		return 0, err
	}
	if len(percentages) > 0 {
		return percentages[0], nil
	}
	return 0, nil
}
