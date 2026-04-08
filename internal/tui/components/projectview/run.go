package projectview

import (
	gocontext "context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	log "charm.land/log/v2"

	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/events"
	proj "github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/bitravens/paravizor/v1/internal/store/db"
	"github.com/bitravens/paravizor/v1/internal/tool"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

// startRun wires up the engine and runs it in a goroutine.
func (m *Model) startRun() tea.Cmd {
	m.running = true
	m.runErr = nil
	// Reset node statuses.
	for i := range m.nodes {
		m.nodes[i].Status = engine.NodeStatusIdle
		m.nodes[i].ItemsIn = 0
		m.nodes[i].ItemsOut = 0
	}

	m.eventCh = make(chan events.Event, 256)
	m.doneCh = make(chan error, 1)

	ctx, cancel := gocontext.WithCancel(gocontext.Background())
	m.cancel = cancel

	eventCh := m.eventCh
	doneCh := m.doneCh
	projectDir := m.projectDir
	pipeline := m.ctx.Pipeline
	cfgDB := m.ctx.Config.DBConfig
	includeTargets := []string{}
	if m.projCfg != nil {
		includeTargets = append(includeTargets, m.projCfg.Scope.Include...)
	}
	configDir, _ := utils.PrvzrConfigDir()

	pipelineName := ""
	if pipeline != nil {
		pipelineName = pipeline.Name
	}

	log.Info("pipeline run requested",
		"project_dir", projectDir,
		"pipeline", pipelineName,
		"targets", len(includeTargets),
	)

	go func() {
		doneCh <- runPipeline(ctx, eventCh, projectDir, pipeline, cfgDB, includeTargets, configDir)
	}()

	m.appendLog("Starting pipeline run...")
	return waitEvent(m.eventCh, m.doneCh)
}

// stopRun cancels the running pipeline.
func (m *Model) stopRun() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.running = false
	m.appendLog("Run cancelled.")
}

// runPipeline is the blocking goroutine function that sets up and runs the engine.
// It must not touch the Model directly; all communication goes through eventCh / doneCh.
func runPipeline(
	ctx gocontext.Context,
	eventCh chan<- events.Event,
	projectDir string,
	pipeline *engine.PipelineConfig,
	dbCfg *store.DBConfig,
	includeTargets []string,
	configDir string,
) error {
	pipelineName := ""
	if pipeline != nil {
		pipelineName = pipeline.Name
	}

	log.Debug("runPipeline begin",
		"project_dir", projectDir,
		"pipeline", pipelineName,
		"targets", len(includeTargets),
	)
	if pipeline == nil {
		return fmt.Errorf("pipeline is nil")
	}

	cfg := store.DBConfig{}
	if dbCfg != nil {
		cfg = *dbCfg
	}

	dbPath := proj.DBPath(projectDir)
	st, err := store.Open(ctx, dbPath, cfg)
	if err != nil {
		log.Error("store open failed", "db_path", dbPath, "err", err)
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()
	log.Debug("store opened", "db_path", dbPath)

	bus := events.NewBus()
	bus.SubscribeAll(func(e events.Event) {
		select {
		case eventCh <- e:
		default:
			// Drop if consumer is too slow (non-blocking).
		}
	})

	toolsDir := configDir + "/tools"
	reg := tool.NewRegistry()
	if err := reg.LoadDir(toolsDir); err != nil {
		log.Error("tool registry load failed", "dir", toolsDir, "err", err)
		return fmt.Errorf("load tools: %w", err)
	}
	reg.CheckAvailability(nil)
	log.Info("tool registry loaded",
		"dir", toolsDir,
		"total", len(reg.All()),
		"available", len(reg.Available()),
		"missing", len(reg.Missing()),
	)

	if err := engine.ValidatePipelineAgainstRegistry(pipeline, reg); err != nil {
		log.Error("pipeline/registry mismatch", "pipeline", pipeline.Name, "err", err)
		return fmt.Errorf("pipeline validation against registry: %w", err)
	}

	logsDir := projectDir + "/logs"
	runner := tool.NewRunner(bus, logsDir)

	dag, err := engine.BuildDAG(pipeline)
	if err != nil {
		log.Error("dag build failed", "pipeline", pipeline.Name, "err", err)
		return fmt.Errorf("build dag: %w", err)
	}
	log.Debug("dag built", "nodes", len(dag.Nodes), "roots", len(dag.RootNodes()))

	if err := seedRootInputs(ctx, st, dag, pipeline, includeTargets); err != nil {
		log.Error("seed inputs failed", "err", err)
		return fmt.Errorf("seed initial inputs: %w", err)
	}
	log.Debug("seed inputs complete")

	// Keep engine's internal slog output out of the terminal UI.
	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	eng := engine.NewEngine(dag, st, bus, reg, runner, silentLogger)
	err = eng.Run(ctx)
	if err != nil {
		log.Error("engine run failed", "err", err)
		return err
	}
	log.Info("engine run completed")
	return nil
}

func seedRootInputs(
	ctx gocontext.Context,
	st *store.Store,
	dag *engine.DAG,
	pipeline *engine.PipelineConfig,
	includeTargets []string,
) error {
	if len(includeTargets) == 0 {
		return nil
	}

	rootNodes := dag.RootNodes()
	initRulesByScope := make(map[string][]engine.InitConfig)
	for _, initCfg := range pipeline.Init {
		initRulesByScope[initCfg.Scope] = append(initRulesByScope[initCfg.Scope], initCfg)
	}

	for _, raw := range includeTargets {
		scopeType, normalized, expanded := normalizeScopeTarget(raw)
		if len(expanded) == 0 {
			continue
		}
		log.Debug("scope target normalized", "raw", raw, "scope", scopeType, "normalized", normalized, "expanded", len(expanded))

		rules := selectInitRules(initRulesByScope, scopeType)
		if len(rules) == 0 {
			fallbackType := "domain"
			if scopeType == "path" {
				fallbackType = "url"
			}
			for _, nodeID := range rootNodes {
				node, ok := dag.Nodes[nodeID]
				if !ok || node.Consumes != fallbackType {
					continue
				}
				rules = append(rules, engine.InitConfig{Scope: scopeType, Node: nodeID, ItemType: fallbackType})
			}
		}

		for _, val := range expanded {
			var (
				itemID   int64
				inserted bool
			)

			for _, rule := range rules {
				node, ok := dag.Nodes[rule.Node]
				if !ok {
					continue
				}
				if node.Consumes != rule.ItemType {
					continue
				}

				if !inserted {
					switch rule.ItemType {
					case "domain":
						id, err := st.InsertDomain(ctx, val, "seed", nil)
						if err != nil {
							return fmt.Errorf("insert seed domain %q: %w", val, err)
						}
						itemID = id
					case "url":
						id, err := st.InsertURL(ctx, val, "seed", nil, nil)
						if err != nil {
							return fmt.Errorf("insert seed url %q: %w", val, err)
						}
						itemID = id
					default:
						continue
					}
					inserted = true
				}

				err := st.SetPipelineState(ctx, &db.PipelineState{
					ItemType: rule.ItemType,
					ItemID:   itemID,
					NodeID:   rule.Node,
					Status:   "pending",
				})
				if err != nil {
					return fmt.Errorf("seed pipeline state node %q item %q: %w", rule.Node, val, err)
				}
				log.Debug("seeded pending state", "node_id", rule.Node, "item", val, "item_type", rule.ItemType, "item_id", itemID)
			}
		}
	}

	return nil
}

func selectInitRules(index map[string][]engine.InitConfig, scopeType string) []engine.InitConfig {
	if rules, ok := index[scopeType]; ok {
		return rules
	}
	return nil
}

func normalizeScopeTarget(raw string) (scopeType string, normalized string, expanded []string) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", "", nil
	}

	if strings.HasPrefix(target, "*.") {
		normalized = strings.TrimPrefix(target, "*.")
		return "wildcard", normalized, []string{normalized}
	}

	if strings.Contains(target, "://") || strings.Contains(target, "/") {
		expanded = expandBracePattern(target)
		for i := range expanded {
			expanded[i] = strings.TrimSpace(expanded[i])
		}
		if len(expanded) == 0 {
			return "path", target, []string{target}
		}
		return "path", target, expanded
	}

	return "exact", target, []string{target}
}

func expandBracePattern(s string) []string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return []string{s}
	}
	end := strings.IndexByte(s[start:], '}')
	if end < 0 {
		return []string{s}
	}
	end += start

	inside := s[start+1 : end]
	parts := strings.Split(inside, ",")
	prefix := s[:start]
	suffix := s[end+1:]

	var out []string
	for _, part := range parts {
		expanded := expandBracePattern(prefix + part + suffix)
		out = append(out, expanded...)
	}
	return out
}

// waitEvent is the bridge cmd between goroutine and bubbletea: waits for either
// an event on eventCh or a completion signal on doneCh.
func waitEvent(eventCh <-chan events.Event, doneCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case e, ok := <-eventCh:
			if !ok {
				return msgRunCompleted{}
			}
			return msgEngineEvent{event: e}
		case err := <-doneCh:
			return msgRunCompleted{err: err}
		}
	}
}

// applyEvent updates node statuses and log lines from engine events.
func (m *Model) applyEvent(e events.Event) {
	ts := e.Timestamp().Format("15:04:05")

	switch v := e.(type) {
	case events.PipelineStarted:
		log.Info("event pipeline.started", "nodes", v.NodeCount, "pipeline_id", v.PipelineID)
		m.appendLog(fmt.Sprintf("[%s] Pipeline started (%d nodes)", ts, v.NodeCount))

	case events.PipelineCompleted:
		log.Info("event pipeline.completed",
			"duration", v.Duration,
			"total_items", v.TotalItems,
			"total_errors", v.TotalErrors,
		)
		m.appendLog(fmt.Sprintf("[%s] Pipeline done — %d items, %d errors, %s",
			ts, v.TotalItems, v.TotalErrors, v.Duration.Round(time.Millisecond)))

	case events.NodeStarted:
		log.Info("event node.started", "node_id", v.NodeID)
		if i := m.nodeIndex(v.NodeID); i >= 0 {
			m.nodes[i].Status = engine.NodeStatusActive
		}
		m.appendLog(fmt.Sprintf("[%s] Node started: %s", ts, v.NodeID))

	case events.NodeCompleted:
		log.Info("event node.completed",
			"node_id", v.NodeID,
			"items_in", v.ItemsIn,
			"items_out", v.ItemsOut,
			"duration", v.Duration,
		)
		if i := m.nodeIndex(v.NodeID); i >= 0 {
			m.nodes[i].Status = engine.NodeStatusCompleted
			m.nodes[i].ItemsIn = v.ItemsIn
			m.nodes[i].ItemsOut = v.ItemsOut
		}
		m.appendLog(fmt.Sprintf("[%s] Node done: %s  in=%d out=%d  (%s)",
			ts, v.NodeID, v.ItemsIn, v.ItemsOut, v.Duration.Round(time.Millisecond)))

	case events.NodeError:
		log.Error("event node.error", "node_id", v.NodeID, "fatal", v.Fatal, "err", v.Err)
		if i := m.nodeIndex(v.NodeID); i >= 0 {
			m.nodes[i].Status = engine.NodeStatusError
		}
		m.appendLog(fmt.Sprintf("[%s] Node error: %s — %v", ts, v.NodeID, v.Err))

	case events.DomainDiscovered:
		_ = ts
		_ = v

	case events.URLDiscovered:
		_ = ts
		_ = v

	case events.FindingDiscovered:
		log.Info("event item.finding.discovered", "severity", v.Severity, "title", v.Title, "node_id", v.NodeID)
		m.appendLog(fmt.Sprintf("[%s] finding [%s] %s  (by: %s)", ts, v.Severity, v.Title, v.Scanner))

	case events.ProcessStarted:
		log.Info("event process.started", "tool", v.ToolName, "pid", v.PID)
		m.appendLog(fmt.Sprintf("[%s] exec: %s (pid %d)", ts, v.ToolName, v.PID))

	case events.ProcessOutput:
		log.Debug("event process.output", "stream", v.Stream, "line", v.Line)
		if v.Stream == "stderr" {
			m.appendLog(fmt.Sprintf("[%s] stderr: %s", ts, v.Line))
		}

	case events.ProcessCompleted:
		log.Info("event process.completed", "tool", v.ToolName, "exit_code", v.ExitCode, "duration", v.Duration)

	default:
		log.Debug("event other", "type", e.EventType())
	}
}
