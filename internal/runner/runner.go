// Package runner provides the engine execution bridge between the TUI and the pipeline engine.
package runner

import (
	gocontext "context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	log "charm.land/log/v2"

	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/events"
	"github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/bitravens/paravizor/v1/internal/store/db"
	"github.com/bitravens/paravizor/v1/internal/tool"
)

// Run is the blocking goroutine function that sets up and runs the engine.
// It must not touch any TUI model directly; all communication goes through eventCh / doneCh.
func Run(
	ctx gocontext.Context,
	eventCh chan<- events.Event,
	projectDir string,
	pipeline *engine.PipelineConfig,
	dbCfg *store.DBConfig,
	includeTargets []string,
	configDir string,
) error {
	if pipeline == nil {
		return fmt.Errorf("pipeline is nil")
	}

	log.Debug("runner.Run begin",
		"project_dir", projectDir,
		"pipeline", pipeline.Name,
		"targets", len(includeTargets),
	)

	cfg := store.DBConfig{}
	if dbCfg != nil {
		cfg = *dbCfg
	}

	dbPath := project.DBPath(projectDir)
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

	if err := SeedRootInputs(ctx, st, dag, pipeline, includeTargets); err != nil {
		log.Error("seed inputs failed", "err", err)
		return fmt.Errorf("seed initial inputs: %w", err)
	}
	log.Debug("seed inputs complete")

	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// In a real run, project config scope should be parsed and passed to NewScopeEngine.
	// For now, we will pass a nil ScopeEngine or an empty one if we don't have project scope in Runner args.
	var scopeEngine *engine.ScopeEngine // this will be populated if project config is available later
	eng := engine.NewEngine(dag, st, bus, reg, runner, silentLogger, scopeEngine)
	err = eng.Run(ctx)
	if err != nil {
		log.Error("engine run failed", "err", err)
		return err
	}
	log.Info("engine run completed")
	return nil
}

// SeedRootInputs seeds the pipeline state table with the initial targets.
func SeedRootInputs(
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
		scopeType, _, expanded := normalizeScopeTarget(raw)
		if len(expanded) == 0 {
			continue
		}

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
