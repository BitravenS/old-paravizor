package utils

import (
	"os"
	"time"

	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type PerformanceMetrics struct {
	CPUPercent float64 // Process CPU + Children CPU
	MemPercent float64 // Process RAM + Children RAM over System RAM
	MemMB      float64 // Process RAM + Children RAM in MB
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

	seen := map[int32]struct{}{}

	// Aggregate Paravizor process and all descendant tool processes.
	myPid := int32(os.Getpid())
	if p, err := process.NewProcess(myPid); err == nil {
		aggregateProcess(p, seen, &totalCPU, &totalRSS)
	}

	// Also include any explicit PIDs from store tracking as fallback/supplement.
	for _, pid := range childPIDs {
		cpid := int32(pid)
		if _, ok := seen[cpid]; ok {
			continue
		}
		if p, err := process.NewProcess(cpid); err == nil {
			aggregateProcess(p, seen, &totalCPU, &totalRSS)
		}
	}

	metrics.CPUPercent = totalCPU
	metrics.MemMB = float64(totalRSS) / 1024 / 1024
	if vmem.Total > 0 {
		metrics.MemPercent = float64(totalRSS) / float64(vmem.Total) * 100
	}

	return metrics
}

func aggregateProcess(p *process.Process, seen map[int32]struct{}, totalCPU *float64, totalRSS *uint64) {
	if p == nil {
		return
	}
	pid := p.Pid
	if _, ok := seen[pid]; ok {
		return
	}
	seen[pid] = struct{}{}

	if c, err := p.Percent(200 * time.Millisecond); err == nil {
		*totalCPU += c
	}
	if m, err := p.MemoryInfo(); err == nil {
		*totalRSS += m.RSS
	}

	children, err := p.Children()
	if err != nil {
		return
	}
	for _, child := range children {
		aggregateProcess(child, seen, totalCPU, totalRSS)
	}
}
