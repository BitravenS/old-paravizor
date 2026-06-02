package engine

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
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

	scopeInclude []string
	scopeExclude []string

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

// SetScope configures in-scope and out-of-scope filters for newly produced items.
func (e *Engine) SetScope(include, exclude []string) {
	e.scopeInclude = append([]string(nil), include...)
	e.scopeExclude = append([]string(nil), exclude...)
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

	if len(e.dag.Order) == 0 {
		return fmt.Errorf("pipeline DAG has no nodes")
	}

	e.bus.Publish(events.PipelineStarted{
		PipelineID: e.dag.Nodes[e.dag.Order[0]].ID,
		NodeCount:  len(e.dag.Nodes),
		Time:       time.Now(),
	})

	startTime := time.Now()
	type nodeResult struct {
		nodeID string
		err    error
	}

	pending := make(map[string]int, len(e.pendingUp))
	for nodeID, count := range e.pendingUp {
		pending[nodeID] = count
	}

	ready := e.dag.RootNodes()
	doneCh := make(chan nodeResult, len(e.dag.Nodes))
	running := 0
	completed := 0
	launch := func(nodeID string) {
		running++
		nodeCfg := e.dag.Nodes[nodeID]
		go func() {
			doneCh <- nodeResult{nodeID: nodeID, err: e.processNode(ctx, nodeCfg)}
		}()
	}

	for completed < len(e.dag.Nodes) {
		for len(ready) > 0 {
			nodeID := ready[0]
			ready = ready[1:]
			launch(nodeID)
		}

		if running == 0 {
			return fmt.Errorf("pipeline scheduler stalled after %d/%d nodes", completed, len(e.dag.Nodes))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-doneCh:
			running--
			completed++
			if result.err != nil {
				e.logger.Error("node failed", "node", result.nodeID, "error", result.err)
				e.setStatus(result.nodeID, NodeStatusError)
				e.bus.Publish(events.NodeError{
					NodeID: result.nodeID,
					Err:    result.err,
					Fatal:  true,
					Time:   time.Now(),
				})
			} else {
				e.setStatus(result.nodeID, NodeStatusCompleted)
			}

			for _, child := range e.dag.Edges[result.nodeID] {
				pending[child]--
				if pending[child] == 0 {
					ready = append(ready, child)
				}
			}
		}
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
		limit := LIMIT
		if nodeCfg.Batch.Size > 0 && nodeCfg.Batch.Size < limit {
			limit = nodeCfg.Batch.Size
		}
		states, err := e.store.GetPendingItems(ctx, nodeID, consumes, limit)
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
			if err := e.store.SetPipelineState(ctx, &db.PipelineState{
				ItemType:    item.ItemType,
				ItemID:      item.ItemID,
				NodeID:      nodeID,
				Status:      "completed",
				CompletedAt: ct,
			}); err != nil {
				return fmt.Errorf("mark item completed: %w", err)
			}
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
		return nil, nil
	}

	if timeout := parseNodeTimeout(nodeCfg.Batch.Timeout); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	result, err := e.runner.Run(ctx, def, input, nodeCfg.ID)
	if err != nil {
		return nil, fmt.Errorf("run %s: %w", toolName, err)
	}
	if result.Error != nil {
		e.logger.Warn("tool returned error", "tool", toolName, "error", result.Error)
		e.bus.Publish(events.LogMessage{
			Level:   "warn",
			Message: fmt.Sprintf("tool %s on node %s returned error: %v", toolName, nodeCfg.ID, result.Error),
			Time:    time.Now(),
		})
	}

	return result.Items, nil
}

func parseNodeTimeout(value string) time.Duration {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	timeout, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return timeout
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
		if item == nil {
			continue
		}
		if !e.itemAllowedByScope(item) {
			e.bus.Publish(events.OutOfScopeFiltered{
				ItemType: string(item.Type()),
				ItemID:   item.ItemID(),
				NodeID:   nodeCfg.ID,
				Reason:   "project scope",
				Time:     time.Now(),
			})
			continue
		}

		itemID, err := e.storeItem(ctx, nodeCfg, item)
		if err != nil {
			e.logger.Warn("failed to store item", "type", item.Type(), "error", err)
			continue
		}

		for _, route := range nodeCfg.Routes {
			if route.Condition != "" && !EvalCondition(route.Condition, item) {
				continue
			}
			if err := e.store.SetPipelineState(ctx, &db.PipelineState{
				ItemType: string(item.Type()),
				ItemID:   itemID,
				NodeID:   route.To,
				Status:   "pending",
			}); err != nil {
				return stored, fmt.Errorf("enqueue item %d for node %q: %w", itemID, route.To, err)
			}
		}

		stored++
	}

	return stored, nil
}

func (e *Engine) storeItem(ctx context.Context, nodeCfg *NodeConfig, item items.Item) (int64, error) {
	switch v := item.(type) {
	case *items.DomainItem:
		return e.storeDomain(ctx, nodeCfg, v.Name, v.SourceName)
	case items.DomainItem:
		return e.storeDomain(ctx, nodeCfg, v.Name, v.SourceName)
	case *items.URLItem:
		return e.storeURL(ctx, nodeCfg, v.FullURL, v.SourceName)
	case items.URLItem:
		return e.storeURL(ctx, nodeCfg, v.FullURL, v.SourceName)
	case *items.IPItem:
		return e.store.UpsertIP(ctx, v.Address)
	case items.IPItem:
		return e.store.UpsertIP(ctx, v.Address)
	case *items.PortItem:
		return e.storePort(ctx, v)
	case items.PortItem:
		return e.storePort(ctx, &v)
	case *items.DNSRecordItem:
		return e.storeDNSRecord(ctx, v)
	case items.DNSRecordItem:
		return e.storeDNSRecord(ctx, &v)
	case *items.FindingItem:
		return e.storeFinding(ctx, nodeCfg, v)
	case items.FindingItem:
		return e.storeFinding(ctx, nodeCfg, &v)
	case *items.FileItem:
		return e.storeFile(ctx, v)
	case items.FileItem:
		return e.storeFile(ctx, &v)
	default:
		return 0, fmt.Errorf("unsupported item type %T", item)
	}
}

func (e *Engine) storeDomain(ctx context.Context, nodeCfg *NodeConfig, name, source string) (int64, error) {
	itemID, err := e.store.InsertDomain(ctx, name, source, nil)
	if err == nil {
		e.updateDomainLivenessFromSource(ctx, itemID, source)
		e.bus.Publish(events.DomainDiscovered{
			DomainName: name,
			DomainID:   itemID,
			Source:     source,
			NodeID:     nodeCfg.ID,
			Time:       time.Now(),
		})
	}
	return itemID, err
}

func (e *Engine) storeURL(ctx context.Context, nodeCfg *NodeConfig, fullURL, source string) (int64, error) {
	itemID, err := e.store.InsertURL(ctx, fullURL, source, nil, nil)
	if err == nil {
		e.bus.Publish(events.URLDiscovered{
			FullURL: fullURL,
			URLID:   itemID,
			Source:  source,
			NodeID:  nodeCfg.ID,
			Time:    time.Now(),
		})
	}
	return itemID, err
}

func (e *Engine) storePort(ctx context.Context, item *items.PortItem) (int64, error) {
	ipID, err := e.store.UpsertIP(ctx, item.Host)
	if err != nil {
		return 0, err
	}
	if item.Protocol == "" {
		item.Protocol = "tcp"
	}
	if err := e.store.UpsertPort(ctx, ipID, item.Port, item.Protocol, nil, nil, item.SourceName); err != nil {
		return 0, err
	}
	var id int64
	err = e.store.DB().QueryRowContext(ctx, `SELECT id FROM ports WHERE ip_id = ? AND port = ? AND protocol = ?`, ipID, item.Port, item.Protocol).Scan(&id)
	return id, err
}

func (e *Engine) storeDNSRecord(ctx context.Context, item *items.DNSRecordItem) (int64, error) {
	domainID, err := e.store.InsertDomain(ctx, item.Name, item.SourceName, nil)
	if err != nil {
		return 0, err
	}
	if err := e.store.UpsertDNSRecord(ctx, domainID, item.RecordType, item.RecordValue, nil, item.SourceName); err != nil {
		return 0, err
	}
	var id int64
	err = e.store.DB().QueryRowContext(ctx, `SELECT id FROM dns_records WHERE domain_id = ? AND record_type = ? AND value = ?`, domainID, item.RecordType, item.RecordValue).Scan(&id)
	return id, err
}

func (e *Engine) storeFinding(ctx context.Context, nodeCfg *NodeConfig, item *items.FindingItem) (int64, error) {
	severity := normalizeSeverity(item.Severity)
	f := &db.Finding{
		Scanner:  item.SourceName,
		Severity: &severity,
		Title:    item.Title,
	}
	itemID, err := e.store.InsertFinding(ctx, f)
	if err == nil {
		e.bus.Publish(events.FindingDiscovered{
			FindingID: itemID,
			Title:     item.Title,
			Severity:  severity,
			Scanner:   item.SourceName,
			NodeID:    nodeCfg.ID,
			Time:      time.Now(),
		})
	}
	return itemID, err
}

func (e *Engine) storeFile(ctx context.Context, item *items.FileItem) (int64, error) {
	originURL := strings.TrimSpace(item.URL)
	if originURL == "" {
		originURL = "file://" + strings.TrimLeft(item.Path, "/")
	}
	urlID, err := e.store.InsertURL(ctx, originURL, item.SourceName, nil, nil)
	if err != nil {
		return 0, err
	}
	fileType := "unknown"
	return e.store.InsertDownloadedFile(ctx, urlID, item.Path, fileType, nil, nil)
}

func normalizeSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical", "high", "medium", "low", "info":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "info"
	}
}

func (e *Engine) itemAllowedByScope(item items.Item) bool {
	target := strings.TrimSpace(string(item.ScopeTarget()))
	if target == "" {
		return true
	}
	for _, pattern := range e.scopeExclude {
		if scopePatternMatches(pattern, target) {
			return false
		}
	}
	if len(e.scopeInclude) == 0 {
		return true
	}
	for _, pattern := range e.scopeInclude {
		if scopePatternMatches(pattern, target) {
			return true
		}
	}
	return false
}

func scopePatternMatches(pattern, target string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	target = strings.ToLower(strings.TrimSpace(target))
	if pattern == "" || target == "" {
		return false
	}
	host := targetHost(target)
	if strings.HasPrefix(pattern, "regex:") {
		re, err := regexp.Compile(strings.TrimPrefix(pattern, "regex:"))
		return err == nil && (re.MatchString(target) || re.MatchString(host))
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*.")
		return host == suffix || strings.HasSuffix(host, "."+suffix)
	}
	if strings.HasPrefix(pattern, "*") && len(pattern) > 1 {
		suffix := strings.TrimLeft(strings.TrimPrefix(pattern, "*"), ".")
		return suffix != "" && (host == suffix || strings.HasSuffix(host, "."+suffix))
	}
	return target == pattern || host == pattern || strings.Contains(target, pattern)
}

func targetHost(target string) string {
	if u, err := url.Parse(target); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return strings.Trim(target, " []")
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
