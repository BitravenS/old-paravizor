package runtime

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitravens/paravizor/v1/internal/config"
	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/events"
	proj "github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/ratelimit"
	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/bitravens/paravizor/v1/internal/store/db"
	"github.com/bitravens/paravizor/v1/internal/tool"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

type Options struct {
	ProjectDir     string
	Pipeline       *engine.PipelineConfig
	Config         config.Config
	Project        *proj.ProjectConfig
	InstallMissing bool
	EventCh        chan<- events.Event
	Logger         *slog.Logger
}

func RunPipeline(ctx context.Context, opts Options) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if strings.TrimSpace(opts.ProjectDir) == "" {
		return fmt.Errorf("project directory is required")
	}

	projectCfg := opts.Project
	if projectCfg == nil {
		loaded, err := proj.LoadProject(opts.ProjectDir)
		if err != nil {
			return fmt.Errorf("load project: %w", err)
		}
		projectCfg = &loaded
	}

	pipeline := opts.Pipeline
	if pipeline == nil {
		pipelineName := opts.Config.DefaultPipeline
		if projectCfg.Pipeline != "" {
			pipelineName = projectCfg.Pipeline
		}
		loaded, err := engine.LoadExternalPipeline(pipelineName)
		if err != nil && loaded == nil {
			return fmt.Errorf("load pipeline: %w", err)
		}
		pipeline = loaded
	}
	if pipeline == nil {
		return fmt.Errorf("pipeline is nil")
	}

	dbCfg := store.DBConfig{}
	if opts.Config.DBConfig != nil {
		dbCfg = *opts.Config.DBConfig
	}
	st, err := store.Open(ctx, proj.DBPath(opts.ProjectDir), dbCfg)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	bus := events.NewBus()
	if opts.EventCh != nil {
		bus.SubscribeAll(func(e events.Event) {
			select {
			case opts.EventCh <- e:
			default:
			}
		})
	}

	configDir, err := utils.PrvzrConfigDir()
	if err != nil {
		return fmt.Errorf("resolve config dir: %w", err)
	}
	reg := tool.NewRegistry()
	if err := reg.LoadDir(filepath.Join(configDir, "tools")); err != nil {
		return fmt.Errorf("load tools: %w", err)
	}
	reg.CheckAvailability(nil)
	bus.Publish(events.LogMessage{Level: "info", Message: "checking required pipeline tools", Time: time.Now()})

	missing := requiredMissingTools(pipeline, reg)
	if len(missing) > 0 && opts.InstallMissing {
		bus.Publish(events.LogMessage{Level: "info", Message: "installing missing tools", Time: time.Now()})
		if err := tool.InstallMissing(ctx, missing); err != nil {
			logger.Warn("tool installation failed", "error", err)
			bus.Publish(events.LogMessage{Level: "warn", Message: fmt.Sprintf("tool installation failed: %v", err), Time: time.Now()})
		}
		reg.CheckAvailability(nil)
		missing = requiredMissingTools(pipeline, reg)
	}
	if len(missing) > 0 {
		bus.Publish(events.LogMessage{Level: "warn", Message: "missing tools will be skipped: " + missingToolList(missing), Time: time.Now()})
	} else {
		bus.Publish(events.LogMessage{Level: "info", Message: "all required pipeline tools are available", Time: time.Now()})
	}

	if err := engine.ValidatePipelineAgainstRegistry(pipeline, reg); err != nil {
		return fmt.Errorf("pipeline validation against registry: %w", err)
	}

	dag, err := engine.BuildDAG(pipeline)
	if err != nil {
		return fmt.Errorf("build dag: %w", err)
	}

	if count, err := st.ResetProcessingItems(ctx); err == nil && count > 0 {
		bus.Publish(events.LogMessage{Level: "info", Message: fmt.Sprintf("recovered %d interrupted items", count), Time: time.Now()})
	}

	if err := seedRootInputs(ctx, st, dag, pipeline, projectCfg.Scope.Include); err != nil {
		return fmt.Errorf("seed initial inputs: %w", err)
	}

	maxProcesses := opts.Config.MaxProcesses
	if maxProcesses <= 0 {
		maxProcesses = 10
	}
	healthInterval := time.Duration(opts.Config.HealthCheckInterval) * time.Second
	if healthInterval <= 0 {
		healthInterval = 10 * time.Second
	}
	pm := tool.NewProcessManager(maxProcesses, healthInterval, 5*time.Second)
	pm.StartHealthChecks()
	defer pm.StopHealthChecks()
	defer pm.KillAll()

	runner := tool.NewRunner(bus, filepath.Join(opts.ProjectDir, "logs")).WithProcessManager(pm)
	stopRates := make(chan struct{})
	if pool := buildRateLimiter(projectCfg, dag, bus); pool != nil {
		pool.Start(stopRates)
		defer close(stopRates)
		runner = runner.WithRateLimiter(pool)
	}

	eng := engine.NewEngine(dag, st, bus, reg, runner, logger)
	eng.SetScope(projectCfg.Scope.Include, projectCfg.Scope.Exclude)
	return eng.Run(ctx)
}

func requiredMissingTools(pipeline *engine.PipelineConfig, reg *tool.Registry) map[string]*tool.ToolConfig {
	missing := make(map[string]*tool.ToolConfig)
	for _, node := range pipeline.Nodes {
		if node.Tool == "" {
			continue
		}
		def, ok := reg.Get(node.Tool)
		if !ok || def == nil {
			missing[node.Tool] = &tool.ToolConfig{Name: node.Tool, Binary: node.Tool}
			continue
		}
		if !def.Available {
			missing[node.Tool] = def
		}
	}
	return missing
}

func missingToolList(missing map[string]*tool.ToolConfig) string {
	var names []string
	for name, def := range missing {
		if def.Binary != "" && def.Binary != name {
			names = append(names, fmt.Sprintf("%s(%s)", name, def.Binary))
		} else {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func buildRateLimiter(projectCfg *proj.ProjectConfig, dag *engine.DAG, bus *events.Bus) *ratelimit.CreditPool {
	if projectCfg == nil || projectCfg.RateLimitMode == "overdrive" || len(projectCfg.RateLimit) == 0 {
		return nil
	}
	budget := 0
	burstPct := 10
	burstMin := 1
	for _, rl := range projectCfg.RateLimit {
		budget += rl.Budget
		if rl.BurstReservePercentage > 0 {
			burstPct = rl.BurstReservePercentage
		}
		if rl.BurstReserveMin > 0 {
			burstMin = rl.BurstReserveMin
		}
	}
	if budget <= 0 {
		return nil
	}
	pool := ratelimit.NewCreditPool(budget, burstPct, burstMin)
	pool.SetWeights(dag.Weights)
	pool.OnRebalance(func(allocations map[string]int) {
		converted := make(map[string]float64, len(allocations))
		for nodeID, value := range allocations {
			converted[nodeID] = float64(value)
		}
		bus.Publish(events.RateLimitRebalanced{Allocations: converted, TotalBudget: float64(budget), Time: time.Now()})
	})
	return pool
}

func seedRootInputs(ctx context.Context, st *store.Store, dag *engine.DAG, pipeline *engine.PipelineConfig, includeTargets []string) error {
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
		rules := append([]engine.InitConfig(nil), initRulesByScope[scopeType]...)
		if scopeType == "exact" {
			for _, wildcardRule := range initRulesByScope["wildcard"] {
				if wildcardRule.ItemType == "domain" {
					rules = append(rules, engine.InitConfig{Scope: scopeType, Node: wildcardRule.Node, ItemType: wildcardRule.ItemType})
				}
			}
		}
		if len(rules) == 0 {
			fallbackType := "domain"
			if scopeType == "path" {
				fallbackType = "url"
			}
			for _, nodeID := range rootNodes {
				node, ok := dag.Nodes[nodeID]
				if ok && node.Consumes == fallbackType {
					rules = append(rules, engine.InitConfig{Scope: scopeType, Node: nodeID, ItemType: fallbackType})
				}
			}
		}

		for _, val := range expanded {
			var itemID int64
			inserted := false
			for _, rule := range rules {
				node, ok := dag.Nodes[rule.Node]
				if !ok || node.Consumes != rule.ItemType {
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
				if err := st.SetPipelineState(ctx, &db.PipelineState{ItemType: rule.ItemType, ItemID: itemID, NodeID: rule.Node, Status: "pending"}); err != nil {
					return fmt.Errorf("seed pipeline state node %q item %q: %w", rule.Node, val, err)
				}
			}
		}
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
	if strings.HasPrefix(target, "*") && len(target) > 1 && !strings.ContainsAny(target, ":/") {
		normalized = strings.TrimLeft(strings.TrimPrefix(target, "*"), ".")
		if normalized != "" {
			return "wildcard", normalized, []string{normalized}
		}
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
		out = append(out, expandBracePattern(prefix+part+suffix)...)
	}
	return out
}
