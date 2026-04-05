package ratelimit

import (
	"sync"
	"time"
)

// CreditPool manages the global per-target request rate budget across all active nodes.
type CreditPool struct {
	mu           sync.Mutex
	totalBudget  int // req/s
	burstReserve int // req/s held for new activations
	burstMin     int // minimum burst reserve
	burstPercent int // burst reserve percentage
	overdrive    bool
	activeNodes  map[string]*Bucket // nodeID -> bucket
	nodeWeights  map[string]int     // nodeID -> DAG weight
	onRebalance  func(allocations map[string]int)
}

// NewCreditPool creates a pool with the given budget settings.
func NewCreditPool(budget, burstReservePercent, burstReserveMin int) *CreditPool {
	burstReserve := budget * burstReservePercent / 100
	if burstReserve < burstReserveMin {
		burstReserve = burstReserveMin
	}
	return &CreditPool{
		totalBudget:  budget,
		burstReserve: burstReserve,
		burstMin:     burstReserveMin,
		burstPercent: burstReservePercent,
		activeNodes:  make(map[string]*Bucket),
		nodeWeights:  make(map[string]int),
	}
}

// SetWeights initialises the weight table from the computed DAG weights.
func (p *CreditPool) SetWeights(weights map[string]int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, w := range weights {
		p.nodeWeights[id] = w
	}
}

// Activate registers a node as active and returns its initial token bucket.
// If no unallocated budget exists the burst reserve is used temporarily.
func (p *CreditPool) Activate(nodeID string) *Bucket {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.overdrive {
		b := newBucket(p.totalBudget)
		p.activeNodes[nodeID] = b
		return b
	}

	// Assign burst reserve so the node starts immediately.
	b := newBucket(p.burstReserve)
	p.activeNodes[nodeID] = b
	p.rebalanceLocked()
	return b
}

// Complete removes a node from active tracking and redistributes its tokens.
func (p *CreditPool) Complete(nodeID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.activeNodes, nodeID)
	p.rebalanceLocked()
}

// Rebalance triggers an immediate rebalance (e.g. after adaptive throttle).
func (p *CreditPool) Rebalance() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rebalanceLocked()
}

// Allocations returns the current token allocation per active node.
func (p *CreditPool) Allocations() map[string]int {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[string]int, len(p.activeNodes))
	for id, b := range p.activeNodes {
		out[id] = b.Rate()
	}
	return out
}

// EnableOverdrive bypasses all rate limiting.
func (p *CreditPool) EnableOverdrive() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.overdrive = true
	for _, b := range p.activeNodes {
		b.SetRate(p.totalBudget * 100) // effectively unlimited
	}
}

// DisableOverdrive restores normal rate limiting.
func (p *CreditPool) DisableOverdrive() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.overdrive = false
	p.rebalanceLocked()
}

// IsOverdrive returns the current overdrive state.
func (p *CreditPool) IsOverdrive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.overdrive
}

// rebalanceLocked recalculates and applies allocations.
// Must be called with p.mu held.
func (p *CreditPool) rebalanceLocked() {
	if len(p.activeNodes) == 0 {
		return
	}

	// Sum weights of active nodes.
	totalWeight := 0
	for id := range p.activeNodes {
		w := p.nodeWeights[id]
		if w == 0 {
			w = 1
		}
		totalWeight += w
	}

	// Effective budget = total - burst reserve.
	effectiveBudget := p.totalBudget - p.burstReserve
	if effectiveBudget < 1 {
		effectiveBudget = 1
	}

	allocated := 0
	for id, b := range p.activeNodes {
		w := p.nodeWeights[id]
		if w == 0 {
			w = 1
		}
		tokens := effectiveBudget * w / totalWeight
		if tokens < 1 {
			tokens = 1
		}
		b.SetRate(tokens)
		allocated += tokens
	}

	// Recalculate burst reserve from rounding remainder.
	remainder := p.totalBudget - allocated
	p.burstReserve = remainder
	if p.burstReserve < p.burstMin {
		p.burstReserve = p.burstMin
	}

	if p.onRebalance != nil {
		alloc := make(map[string]int, len(p.activeNodes))
		for id, b := range p.activeNodes {
			alloc[id] = b.Rate()
		}
		p.onRebalance(alloc)
	}
}

// OnRebalance registers a callback invoked after every rebalance.
func (p *CreditPool) OnRebalance(fn func(allocations map[string]int)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onRebalance = fn
}

// Wait blocks until the node's token bucket allows one request.
// Returns immediately in overdrive mode.
func (p *CreditPool) Wait(nodeID string) {
	p.mu.Lock()
	b, ok := p.activeNodes[nodeID]
	p.mu.Unlock()
	if !ok || p.IsOverdrive() {
		return
	}
	b.Wait()
}

// SetNodeRate overrides a specific node's rate (used by adaptive throttle).
func (p *CreditPool) SetNodeRate(nodeID string, rate int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if b, ok := p.activeNodes[nodeID]; ok {
		b.SetRate(rate)
	}
}

// NodeRate returns the current rate for a node (0 if not active).
func (p *CreditPool) NodeRate(nodeID string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if b, ok := p.activeNodes[nodeID]; ok {
		return b.Rate()
	}
	return 0
}

// tickerRefill runs in a background goroutine, refilling all buckets at their
// configured rate every second.
func (p *CreditPool) tickerRefill(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			p.mu.Lock()
			for _, b := range p.activeNodes {
				b.refill()
			}
			p.mu.Unlock()
		}
	}
}

// Start launches the background token refill goroutine. Call once.
func (p *CreditPool) Start(stop <-chan struct{}) {
	go p.tickerRefill(stop)
}
