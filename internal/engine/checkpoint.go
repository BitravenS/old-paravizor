package engine

import (
	"context"
	"time"

	"github.com/bitravens/paravizor/v1/internal/events"
	"github.com/bitravens/paravizor/v1/internal/store"
)

// CheckpointManager handles interrupted session detection and periodic checkpoint events.
type CheckpointManager struct {
	store  *store.Store
	bus    *events.Bus
	period time.Duration
}

func NewCheckpointManager(st *store.Store, bus *events.Bus, period time.Duration) *CheckpointManager {
	if period <= 0 {
		period = 30 * time.Second
	}
	return &CheckpointManager{store: st, bus: bus, period: period}
}

func (c *CheckpointManager) HasInterruptedSession(ctx context.Context) (bool, error) {
	return c.store.HasInterruptedSession(ctx)
}

func (c *CheckpointManager) RecoverInterruptedSession(ctx context.Context) (int64, error) {
	return c.store.ResetProcessingItems(ctx)
}

func (c *CheckpointManager) Start(ctx context.Context) {
	ticker := time.NewTicker(c.period)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.bus.Publish(events.CheckpointSaved{Time: time.Now()})
		}
	}
}
