package tool

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type ProcessEntry struct {
	cmd       *exec.Cmd
	startedAt time.Time
	nodeID    string
	toolName  string
	done      chan struct{}
}

// ProcessManager limits concurrent tool processes, monitors health,
// and provides graceful shutdown (SIGTERM → wait grace → SIGKILL).
type ProcessManager struct {
	maxConcurrent   int
	healthInterval  time.Duration
	gracePeriod     time.Duration
	sem             chan struct{}
	mu              sync.Mutex
	running         map[int]*ProcessEntry // pid → entry
	stopHealthCheck chan struct{}
}

// NewProcessManager creates a process manager.
//
//	maxConcurrent:  maximum simultaneous processes (≤0 = unlimited).
//	healthInterval: how often the background goroutine checks liveness.
//	gracePeriod:    time between SIGTERM and SIGKILL on forced shutdown.
func NewProcessManager(maxConcurrent int, healthInterval, gracePeriod time.Duration) *ProcessManager {
	sem := make(chan struct{}, maxConcurrent)
	if maxConcurrent <= 0 {
		sem = make(chan struct{}, 65536)
	}
	return &ProcessManager{
		maxConcurrent:   maxConcurrent,
		healthInterval:  healthInterval,
		gracePeriod:     gracePeriod,
		sem:             sem,
		running:         make(map[int]*ProcessEntry),
		stopHealthCheck: make(chan struct{}),
	}
}

// Acquire blocks until a process slot is available or the context is cancelled.
// Call before starting a tool process.
func (pm *ProcessManager) Acquire(ctx context.Context) error {
	select {
	case pm.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees a process slot. Call after a tool process completes.
func (pm *ProcessManager) Release() {
	select {
	case <-pm.sem:
	default:
	}
}

// Register tracks a started process for health monitoring.
func (pm *ProcessManager) Register(cmd *exec.Cmd, nodeID, toolName string) {
	if cmd.Process == nil {
		return
	}
	entry := &ProcessEntry{
		cmd:       cmd,
		startedAt: time.Now(),
		nodeID:    nodeID,
		toolName:  toolName,
		done:      make(chan struct{}),
	}
	pm.mu.Lock()
	pm.running[cmd.Process.Pid] = entry
	pm.mu.Unlock()
}

// Unregister removes a process from active tracking.
func (pm *ProcessManager) Unregister(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pm.mu.Lock()
	entry, ok := pm.running[cmd.Process.Pid]
	if ok {
		select {
		case <-entry.done:
		default:
			close(entry.done)
		}
		delete(pm.running, cmd.Process.Pid)
	}
	pm.mu.Unlock()
}

// StartHealthChecks starts a background goroutine that removes dead processes
// from the tracking map at the configured interval.
func (pm *ProcessManager) StartHealthChecks() {
	go func() {
		ticker := time.NewTicker(pm.healthInterval)
		defer ticker.Stop()
		for {
			select {
			case <-pm.stopHealthCheck:
				return
			case <-ticker.C:
				pm.checkProcessHealth()
			}
		}
	}()
}

// StopHealthChecks signals the health check goroutine to exit.
func (pm *ProcessManager) StopHealthChecks() {
	select {
	case <-pm.stopHealthCheck:
	default:
		close(pm.stopHealthCheck)
	}
}

// checkProcessHealth removes dead processes from the running map.
func (pm *ProcessManager) checkProcessHealth() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var dead []int
	for pid, entry := range pm.running {
		if !isProcessAlive(entry.cmd.Process) {
			dead = append(dead, pid)
		}
	}
	for _, pid := range dead {
		entry := pm.running[pid]
		select {
		case <-entry.done:
		default:
			close(entry.done)
		}
		delete(pm.running, pid)
	}
}

// KillAll sends SIGTERM to all tracked processes, then SIGKILL after the grace
// period to any that are still alive.
func (pm *ProcessManager) KillAll() {
	pm.mu.Lock()
	entries := make([]*ProcessEntry, 0, len(pm.running))
	for _, e := range pm.running {
		entries = append(entries, e)
	}
	pm.mu.Unlock()

	for _, e := range entries {
		if e.cmd.Process != nil {
			_ = e.cmd.Process.Signal(syscall.SIGTERM)
		}
	}

	if len(entries) == 0 {
		return
	}

	time.Sleep(pm.gracePeriod)

	for _, e := range entries {
		if e.cmd.Process != nil && isProcessAlive(e.cmd.Process) {
			_ = e.cmd.Process.Kill()
		}
	}
}

// ActiveCount returns the number of currently tracked processes.
func (pm *ProcessManager) ActiveCount() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return len(pm.running)
}

// isProcessAlive sends signal 0 to check whether a process is still running.
func isProcessAlive(p *os.Process) bool {
	if p == nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
