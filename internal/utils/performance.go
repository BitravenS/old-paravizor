package utils

import (
	"os"

	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type PerformanceMetrics struct {
	CPUPercent float64 // Process CPU + Children CPU
	MemPercent float64 // Process RAM + Children RAM over System RAM
}

// GetPerformanceMetrics calculates the total CPU and Memory percentage used by
// the current process and the specified child PIDs.
func GetPerformanceMetrics(childPIDs []int64) PerformanceMetrics {
	var metrics PerformanceMetrics

	// Get system memory
	vmem, err := mem.VirtualMemory()
	if err != nil {
		return metrics
	}

	var totalCPU float64
	var totalRSS uint64

	// Main process
	myPid := int32(os.Getpid())
	if p, err := process.NewProcess(myPid); err == nil {
		if c, err := p.CPUPercent(); err == nil {
			totalCPU += c
		}
		if m, err := p.MemoryInfo(); err == nil {
			totalRSS += m.RSS
		}
	}

	// Child processes
	for _, pid := range childPIDs {
		if cp, err := process.NewProcess(int32(pid)); err == nil {
			if c, err := cp.CPUPercent(); err == nil {
				totalCPU += c
			}
			if m, err := cp.MemoryInfo(); err == nil {
				totalRSS += m.RSS
			}
		}
	}

	metrics.CPUPercent = totalCPU
	if vmem.Total > 0 {
		metrics.MemPercent = float64(totalRSS) / float64(vmem.Total) * 100
	}

	return metrics
}
