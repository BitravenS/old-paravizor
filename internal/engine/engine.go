package engine

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/bitravens/paravizor/v1/internal/events"
	"github.com/bitravens/paravizor/v1/internal/items"
	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/bitravens/paravizor/v1/internal/store/db"
	"github.com/bitravens/paravizor/v1/internal/tool"
)

const (
	LIMIT = 150_000
)

// Engine orchestrates pipeline execution.
type Engine struct {
	dag     *DAG
	store   *store.Store
	bus     *events.Bus
	toolReg *tool.Registry
	runner  *tool.Runner
	logger  *slog.Logger

	mu         sync.Mutex
	nodeStatus map[string]NodeStatus
	nodeStats  map[string]*NodeStats
	pendingUp  map[string]int // count of non-completed upstream nodes
	cancel     context.CancelFunc
}

// DAG returns the underlying DAG.
func (e *Engine) DAG() *DAG {
	return e.dag
}

// Bus returns the event bus.
func (e *Engine) Bus() *events.Bus {
	return e.bus
}

// Registry returns the tool registry.
func (e *Engine) Registry() *tool.Registry {
	return e.toolReg
}

// NewEngine creates a new pipeline engine.
func NewEngine(
	dag *DAG,
	st *store.Store,
	bus *events.Bus,
	toolReg *tool.Registry,
	runner *tool.Runner,
	logger *slog.Logger,
) *Engine {
	e := &Engine{
		dag:        dag,
		store:      st,
		bus:        bus,
		toolReg:    toolReg,
		runner:     runner,
		logger:     logger,
		nodeStatus: make(map[string]NodeStatus),
		nodeStats:  make(map[string]*NodeStats),
		pendingUp:  make(map[string]int),
	}

	// Initialize node status and pending upstream counts
	for id := range dag.Nodes {
		e.nodeStatus[id] = NodeStatusIdle
		e.nodeStats[id] = &NodeStats{}
		e.pendingUp[id] = dag.UpstreamCount(id)
	}

	return e
}

// Run starts the pipeline execution.
func (e *Engine) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	defer cancel()

	e.bus.Publish(events.PipelineStarted{
		PipelineID: e.dag.Nodes[e.dag.Order[0]].ID,
		NodeCount:  len(e.dag.Nodes),
		Time:       time.Now(),
	})

	startTime := time.Now()

	// Process nodes in topological order.
	for _, nodeID := range e.dag.Order {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		nodeCfg := e.dag.Nodes[nodeID]
		if err := e.processNode(ctx, nodeCfg); err != nil {
			e.logger.Error("node failed", "node", nodeID, "error", err)
			e.setStatus(nodeID, NodeStatusError)
			e.bus.Publish(events.NodeError{
				NodeID: nodeID,
				Err:    err,
				Fatal:  true,
				Time:   time.Now(),
			})
			continue
		}
		e.setStatus(nodeID, NodeStatusCompleted)
	}

	totalItems := 0
	totalErrors := 0
	for _, stats := range e.nodeStats {
		totalItems += stats.ItemsOut
		totalErrors += stats.Errors
	}

	e.bus.Publish(events.PipelineCompleted{
		Duration:    time.Since(startTime),
		TotalItems:  totalItems,
		TotalErrors: totalErrors,
		Time:        time.Now(),
	})

	return nil
}

func (e *Engine) processNode(ctx context.Context, nodeCfg *NodeConfig) error {
	nodeID := nodeCfg.ID
	e.setStatus(nodeID, NodeStatusActive)

	e.bus.Publish(events.NodeStarted{
		NodeID: nodeID,
		Time:   time.Now(),
	})

	startTime := time.Now()
	consumes := nodeCfg.Consumes
	totalIn := 0
	totalOut := 0

	for {
		states, err := e.store.GetPendingItems(ctx, nodeID, consumes, LIMIT)
		if err != nil {
			return fmt.Errorf("get pending items: %w", err)
		}
		if len(states) == 0 {
			break
		}

		totalIn += len(states)

		for _, state := range states {
			now := time.Now()
			st := sql.NullTime{Time: now, Valid: true}
			err := e.store.SetPipelineState(ctx, &db.PipelineState{
				ItemType:  state.ItemType,
				ItemID:    state.ItemID,
				NodeID:    nodeID,
				Status:    "processing",
				StartedAt: st,
			})
			if err != nil {
				return fmt.Errorf("mark item processing: %w", err)
			}
		}

		e.logger.Info("processing node", "node", nodeID, "items", len(states), "tool", nodeCfg.Tool)

		inputStrings := e.resolveInputStrings(ctx, states, consumes)
		var outputItems []items.Item

		if nodeCfg.Tool != "" {
			// Run external tool
			outputItems, err = e.runSingle(ctx, nodeCfg, inputStrings)
			if err != nil {
				e.mu.Lock()
				e.nodeStats[nodeID].Errors++
				e.mu.Unlock()
				return err
			}
		} else {
			// Passthrough node (e.g. router or filter)
			outputItems, err = e.statesAsItems(ctx, states, consumes)
			if err != nil {
				e.mu.Lock()
				e.nodeStats[nodeID].Errors++
				e.mu.Unlock()
				return err
			}
		}

		// Apply node-level filter regex (if any)
		if nodeCfg.Filter != "" {
			outputItems = filterItemsRegex(outputItems, nodeCfg.Filter)
		}

		// Mark input items as completed
		for _, item := range states {
			now := time.Now()
			ct := sql.NullTime{Time: now, Valid: true}
			e.store.SetPipelineState(ctx, &db.PipelineState{
				ItemType:    item.ItemType,
				ItemID:      item.ItemID,
				NodeID:      nodeID,
				Status:      "completed",
				CompletedAt: ct,
			})
		}

		// Store output items and enqueue for downstream nodes
		stored, err := e.storeAndRoute(ctx, nodeCfg, outputItems)
		if err != nil {
			return fmt.Errorf("store and route: %w", err)
		}
		totalOut += stored
	}

	if totalIn == 0 {
		e.logger.Info("no pending items", "node", nodeID)
	}

	e.mu.Lock()
	e.nodeStats[nodeID].ItemsIn = totalIn
	e.nodeStats[nodeID].ItemsOut = totalOut
	e.nodeStats[nodeID].Duration = time.Since(startTime)
	e.mu.Unlock()

	e.bus.Publish(events.NodeCompleted{
		NodeID:   nodeID,
		ItemsIn:  totalIn,
		ItemsOut: totalOut,
		Duration: time.Since(startTime),
		Time:     time.Now(),
	})

	return nil
}

func (e *Engine) runSingle(ctx context.Context, nodeCfg *NodeConfig, input []string) ([]items.Item, error) {
	toolName := nodeCfg.Tool
	def, ok := e.toolReg.Get(toolName)
	if !ok {
		return nil, fmt.Errorf("tool %s not found in registry", toolName)
	}
	if !def.Available {
		e.logger.Warn("tool not available, skipping", "tool", toolName, "node", nodeCfg.ID)
		e.bus.Publish(events.LogMessage{
			Level:   "warn",
			Message: fmt.Sprintf("tool %s is not installed or not on PATH; skipping node %s", toolName, nodeCfg.ID),
			Fields: map[string]string{
				"tool":   toolName,
				"binary": def.Binary,
				"node":   nodeCfg.ID,
			},
			Time: time.Now(),
		})
		return nil, nil
	}

	result, err := e.runner.Run(ctx, def, input, nodeCfg.ID)
	if err != nil {
		return nil, fmt.Errorf("run %s: %w", toolName, err)
	}
	if result.Error != nil {
		e.logger.Warn("tool returned error", "tool", toolName, "error", result.Error)
	}

	return result.Items, nil
}

func (e *Engine) resolveInputStrings(ctx context.Context, states []db.PipelineState, itemType string) []string {
	var result []string

	switch items.ItemType(itemType) {
	case items.TypeDomain:
		for _, item := range states {
			var name string
			err := e.store.DB().QueryRowContext(ctx,
				`SELECT name FROM domains WHERE id = ?`, item.ItemID,
			).Scan(&name)
			if err == nil && name != "" {
				result = append(result, name)
			}
		}
	case items.TypeURL:
		for _, item := range states {
			var url string
			err := e.store.DB().QueryRowContext(ctx,
				`SELECT full_url FROM urls WHERE id = ?`, item.ItemID,
			).Scan(&url)
			if err == nil && url != "" {
				result = append(result, url)
			}
		}
	case items.TypeIP:
		for _, item := range states {
			var addr string
			err := e.store.DB().QueryRowContext(ctx,
				`SELECT address FROM ips WHERE id = ?`, item.ItemID,
			).Scan(&addr)
			if err == nil && addr != "" {
				result = append(result, addr)
			}
		}
	}

	return result
}

func (e *Engine) statesAsItems(ctx context.Context, states []db.PipelineState, itemType string) ([]items.Item, error) {
	var result []items.Item
	for _, state := range states {
		switch items.ItemType(itemType) {
		case items.TypeURL:
			var rawURL string
			err := e.store.DB().QueryRowContext(ctx,
				`SELECT full_url FROM urls WHERE id = ?`, state.ItemID,
			).Scan(&rawURL)
			if err == nil && rawURL != "" {
				result = append(result, &items.URLItem{
					ID:         state.ItemID,
					FullURL:    rawURL,
					SourceName: "builtin",
				})
			}
		case items.TypeDomain:
			var name string
			err := e.store.DB().QueryRowContext(ctx,
				`SELECT name FROM domains WHERE id = ?`, state.ItemID,
			).Scan(&name)
			if err == nil && name != "" {
				result = append(result, &items.DomainItem{
					ID:         state.ItemID,
					Name:       name,
					SourceName: "builtin",
				})
			}
		}
	}
	return result, nil
}

func (e *Engine) storeAndRoute(ctx context.Context, nodeCfg *NodeConfig, out []items.Item) (int, error) {
	stored := 0

	for _, item := range out {
		var itemID int64
		var err error

		switch v := item.(type) {
		case *items.DomainItem:
			itemID, err = e.store.InsertDomain(ctx, v.Name, v.SourceName, nil)
			if err == nil {
				e.updateDomainLivenessFromSource(ctx, itemID, v.SourceName)
			}
		case items.DomainItem:
			itemID, err = e.store.InsertDomain(ctx, v.Name, v.SourceName, nil)
			if err == nil {
				e.updateDomainLivenessFromSource(ctx, itemID, v.SourceName)
			}
		case *items.URLItem:
			itemID, err = e.store.InsertURL(ctx, v.FullURL, v.SourceName, nil, nil)
		case items.URLItem:
			itemID, err = e.store.InsertURL(ctx, v.FullURL, v.SourceName, nil, nil)
		case *items.FindingItem:
			f := &db.Finding{
				Scanner:  v.SourceName,
				Severity: &v.Severity,
				Title:    v.Title,
			}
			itemID, err = e.store.InsertFinding(ctx, f)
			if err == nil {
				e.bus.Publish(events.FindingDiscovered{
					FindingID: itemID,
					Title:     v.Title,
					Severity:  v.Severity,
					Scanner:   v.SourceName,
					NodeID:    nodeCfg.ID,
					Time:      time.Now(),
				})
			}
		case items.FindingItem:
			f := &db.Finding{
				Scanner:  v.SourceName,
				Severity: &v.Severity,
				Title:    v.Title,
			}
			itemID, err = e.store.InsertFinding(ctx, f)
			if err == nil {
				e.bus.Publish(events.FindingDiscovered{
					FindingID: itemID,
					Title:     v.Title,
					Severity:  v.Severity,
					Scanner:   v.SourceName,
					NodeID:    nodeCfg.ID,
					Time:      time.Now(),
				})
			}
		default:
			continue
		}

		if err != nil {
			e.logger.Warn("failed to store item", "type", item.Type(), "error", err)
			continue
		}

		// Enqueue for downstream nodes
		for _, route := range nodeCfg.Routes {
			if route.Condition != "" && !EvalCondition(route.Condition, item) {
				continue
			}
			e.store.SetPipelineState(ctx, &db.PipelineState{
				ItemType: string(item.Type()),
				ItemID:   itemID,
				NodeID:   route.To,
				Status:   "pending",
			})
		}

		stored++
	}

	return stored, nil
}

func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
}

func (e *Engine) setStatus(nodeID string, status NodeStatus) {
	e.mu.Lock()
	e.nodeStatus[nodeID] = status
	e.mu.Unlock()
}

func (e *Engine) GetNodeStatus(nodeID string) NodeStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.nodeStatus[nodeID]
}

func (e *Engine) GetNodeStats(nodeID string) NodeStats {
	e.mu.Lock()
	defer e.mu.Unlock()
	if stats, ok := e.nodeStats[nodeID]; ok {
		return *stats
	}
	return NodeStats{}
}

// filterItemsRegex filters items by applying a regex to their string value.
func filterItemsRegex(in []items.Item, pattern string) []items.Item {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return in
	}
	var out []items.Item
	for _, item := range in {
		if re.MatchString(item.Value()) {
			out = append(out, item)
		}
	}
	return out
}

func (e *Engine) updateDomainLivenessFromSource(ctx context.Context, domainID int64, source string) {
	if source != "dnsx-live" {
		return
	}

	isLive := true
	if err := e.store.WriteTx(ctx, func(q *db.Queries) error {
		return q.UpdateDomainLiveness(ctx, db.UpdateDomainLivenessParams{
			ID:     domainID,
			IsLive: &isLive,
			Ip:     nil,
		})
	}); err != nil {
		e.logger.Warn("failed to update domain liveness", "domain_id", domainID, "source", source, "error", err)
	}
}
