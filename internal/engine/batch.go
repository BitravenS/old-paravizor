package engine

import (
	"context"
	"time"

	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/bitravens/paravizor/v1/internal/store/db"
)

// BatchAccumulator collects pending pipeline states until a batch policy fires.
// It is used by processNode to decide when to release a batch for processing.
type BatchAccumulator struct {
	cfg    BatchConfig
	nodeID string
}

// NewBatchAccumulator creates a batch accumulator for the given node.
func NewBatchAccumulator(nodeID string, cfg BatchConfig) *BatchAccumulator {
	return &BatchAccumulator{cfg: cfg, nodeID: nodeID}
}

// parseBatchTimeout parses the timeout string from BatchConfig.
// Returns 0 if empty or invalid (no timeout).
func (b *BatchAccumulator) parseBatchTimeout() time.Duration {
	if b.cfg.Timeout == "" {
		return 0
	}
	d, err := time.ParseDuration(b.cfg.Timeout)
	if err != nil {
		return 0
	}
	return d
}

// ShouldProcess returns true if the batch is ready to be processed.
// Rules:
//  1. If size >= cfg.Size (and cfg.Size > 0), process immediately.
//  2. If timeout elapsed, process whatever we have (if >= min_size).
//  3. If wait_for_peers = true, all upstream nodes must be draining/completed.
func (b *BatchAccumulator) ShouldProcess(
	states []db.PipelineState,
	upstreamAllCompleted bool,
	elapsed time.Duration,
) bool {
	n := len(states)
	if n == 0 {
		return false
	}

	// If wait_for_peers is set, hold until upstream is completely done
	if b.cfg.WaitForPeers {
		if !upstreamAllCompleted {
			return false
		}
		return true // upstream done, we can process what we have
	}

	// Always process remaining items if upstream is completed
	if upstreamAllCompleted {
		return true
	}

	// If neither size nor timeout configured, process immediately
	if b.cfg.Size == 0 && b.parseBatchTimeout() == 0 {
		return true
	}

	// size threshold
	if b.cfg.Size > 0 && n >= b.cfg.Size {
		return true
	}

	// timeout threshold
	timeout := b.parseBatchTimeout()
	if timeout > 0 && elapsed >= timeout {
		minSize := b.cfg.MinSize
		if minSize <= 0 {
			minSize = 1
		}
		return n >= minSize
	}

	return false
}

// WaitAndCollect polls the store for pending items until the batch policy fires,
// context is cancelled, or timeout elapses. Returns the collected batch.
// This is used in a poll-based model (no channels) compatible with the DB-backed engine.
func (b *BatchAccumulator) WaitAndCollect(
	ctx context.Context,
	st *store.Store,
	itemType string,
	maxItems int,
	upstreamDone func() bool,
) ([]db.PipelineState, error) {
	if b.cfg.Size > 0 && maxItems > b.cfg.Size {
		maxItems = b.cfg.Size
	}

	timeout := b.parseBatchTimeout()
	if timeout == 0 {
		timeout = 60 * time.Second // default max wait
	}

	deadline := time.Now().Add(timeout)
	poll := 500 * time.Millisecond

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		states, err := st.GetPendingItems(ctx, b.nodeID, itemType, maxItems)
		if err != nil {
			return nil, err
		}

		upDone := upstreamDone()

		// If upstream is done and there are no more items, return immediately
		if len(states) == 0 && upDone {
			return states, nil
		}

		elapsed := time.Since(deadline.Add(-timeout))
		if b.ShouldProcess(states, upDone, elapsed) {
			return states, nil
		}

		// Nothing ready yet; wait a bit before next poll.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(poll):
		}

		// Hard deadline
		if time.Now().After(deadline) && !b.cfg.WaitForPeers {
			minSize := b.cfg.MinSize
			if minSize <= 0 {
				minSize = 1
			}
			if len(states) >= minSize {
				return states, nil
			}
			deadline = time.Now().Add(timeout)
		}
	}
}
