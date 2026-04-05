package ratelimit

import (
	"sync"
	"time"
)

// AdaptiveThrottle monitors tool responses and adjusts per-node rates.
type AdaptiveThrottle struct {
	mu                     sync.Mutex
	pool                   *CreditPool
	backoffFactor          float64
	recoveryRate           float64 // fraction per minute
	maxConsecutiveTimeouts int

	// Per-node state.
	consecutiveTimeouts map[string]int
	throttledAt         map[string]time.Time
	baseRate            map[string]int // rate before throttle
}

// NewAdaptiveThrottle creates a throttle bound to the given pool.
func NewAdaptiveThrottle(pool *CreditPool, backoffFactor, recoveryRate float64, maxTimeouts int) *AdaptiveThrottle {
	return &AdaptiveThrottle{
		pool:                   pool,
		backoffFactor:          backoffFactor,
		recoveryRate:           recoveryRate,
		maxConsecutiveTimeouts: maxTimeouts,
		consecutiveTimeouts:    make(map[string]int),
		throttledAt:            make(map[string]time.Time),
		baseRate:               make(map[string]int),
	}
}

// OnRateLimit should be called when a 429 response is received for nodeID.
func (a *AdaptiveThrottle) OnRateLimit(nodeID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.applyBackoff(nodeID, a.backoffFactor)
}

// OnTimeout should be called when a request timeout is observed for nodeID.
func (a *AdaptiveThrottle) OnTimeout(nodeID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.consecutiveTimeouts[nodeID]++
	if a.consecutiveTimeouts[nodeID] >= a.maxConsecutiveTimeouts {
		a.applyBackoff(nodeID, 0.75) // reduce by 25%
		a.consecutiveTimeouts[nodeID] = 0
	}
}

// OnConnectionReset should be called on connection reset errors.
func (a *AdaptiveThrottle) OnConnectionReset(nodeID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.applyBackoff(nodeID, 0.75)
}

// OnSuccess should be called on successful responses to reset timeout counter.
func (a *AdaptiveThrottle) OnSuccess(nodeID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.consecutiveTimeouts[nodeID] = 0
}

// applyBackoff reduces nodeID's rate by multiplying by factor.
// Must be called with a.mu held.
func (a *AdaptiveThrottle) applyBackoff(nodeID string, factor float64) {
	currentRate := a.pool.NodeRate(nodeID)
	if currentRate == 0 {
		return
	}
	if _, throttled := a.throttledAt[nodeID]; !throttled {
		a.baseRate[nodeID] = currentRate
	}
	newRate := int(float64(currentRate) * factor)
	if newRate < 1 {
		newRate = 1
	}
	a.throttledAt[nodeID] = time.Now()
	a.pool.SetNodeRate(nodeID, newRate)
}

// Start launches a background recovery goroutine that gradually restores
// throttled node rates at recoveryRate (fraction of base rate per minute).
func (a *AdaptiveThrottle) Start(stop <-chan struct{}) {
	go a.recoveryLoop(stop)
}

func (a *AdaptiveThrottle) recoveryLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			a.recover()
		}
	}
}

func (a *AdaptiveThrottle) recover() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for nodeID, throttledAt := range a.throttledAt {
		_ = throttledAt // throttle time recorded for possible future use
		base, ok := a.baseRate[nodeID]
		if !ok {
			continue
		}
		current := a.pool.NodeRate(nodeID)
		if current >= base {
			delete(a.throttledAt, nodeID)
			delete(a.baseRate, nodeID)
			continue
		}
		increase := int(float64(base) * a.recoveryRate)
		if increase < 1 {
			increase = 1
		}
		newRate := current + increase
		if newRate > base {
			newRate = base
		}
		a.pool.SetNodeRate(nodeID, newRate)
		if newRate >= base {
			delete(a.throttledAt, nodeID)
			delete(a.baseRate, nodeID)
		}
	}
}
