package engine

import (
	"context"
	"time"

	"github.com/bitravens/paravizor/v1/internal/items"
)

// NodeStatus represents the current state of a pipeline node.
type NodeStatus string

const (
	NodeStatusIdle      NodeStatus = "idle"
	NodeStatusActive    NodeStatus = "active"
	NodeStatusDraining  NodeStatus = "draining"
	NodeStatusCompleted NodeStatus = "completed"
	NodeStatusError     NodeStatus = "error"
)

// NodeStats tracks processing statistics for a node.
type NodeStats struct {
	ItemsIn       int
	ItemsOut      int
	ItemsFiltered int
	Errors        int
	StartedAt     time.Time
	CompletedAt   time.Time
	Duration      time.Duration
}

// Node is the runtime representation of a pipeline node.
type Node interface {
	// Config returns the node configuration.
	Config() *NodeConfig
	// Status returns the current node status.
	Status() NodeStatus
	// Stats returns processing statistics.
	Stats() NodeStats
	// Process processes a batch of items and returns output items.
	Process(ctx context.Context, in []items.Item) ([]items.Item, error)
	// SetStatus updates the node status.
	SetStatus(status NodeStatus)
}
