package ratelimit

import (
	"sync"
	"time"
)

// Bucket is a token bucket for a single node.
// Tokens are replenished at a configurable rate (tokens/second).
type Bucket struct {
	mu      sync.Mutex
	tokens  int
	maxRate int // tokens added per second
	avail   chan struct{}
}

func newBucket(rate int) *Bucket {
	if rate < 1 {
		rate = 1
	}
	b := &Bucket{
		tokens:  rate,
		maxRate: rate,
		avail:   make(chan struct{}, 1),
	}
	b.signal()
	return b
}

// Rate returns the current configured rate.
func (b *Bucket) Rate() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxRate
}

// SetRate updates the configured rate; also refills to avoid instant starvation.
func (b *Bucket) SetRate(rate int) {
	if rate < 1 {
		rate = 1
	}
	b.mu.Lock()
	b.maxRate = rate
	if b.tokens < rate {
		b.tokens = rate // give immediate credit on rate increase
	}
	b.mu.Unlock()
	b.signal()
}

// Wait blocks until at least one token is available, then consumes it.
func (b *Bucket) Wait() {
	for {
		b.mu.Lock()
		if b.tokens > 0 {
			b.tokens--
			b.mu.Unlock()
			return
		}
		b.mu.Unlock()
		// Block until signalled or poll.
		select {
		case <-b.avail:
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// TryAcquire consumes a token if available and returns true; else false.
func (b *Bucket) TryAcquire() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

// refill adds tokens up to maxRate (called every second by credit pool ticker).
func (b *Bucket) refill() {
	b.mu.Lock()
	b.tokens += b.maxRate
	if b.tokens > b.maxRate*2 {
		b.tokens = b.maxRate * 2 // cap at 2x to allow short bursts
	}
	b.mu.Unlock()
	b.signal()
}

// signal notifies waiters that tokens may be available.
func (b *Bucket) signal() {
	select {
	case b.avail <- struct{}{}:
	default:
	}
}
