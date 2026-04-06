// Package projectview provides the TUI page for creating and running recon projects.
package projectview

import (
	gocontext "context"
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	log "charm.land/log/v2"

	"github.com/bitravens/paravizor/v1/internal/config"
	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/events"
	proj "github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/bitravens/paravizor/v1/internal/store/db"
	"github.com/bitravens/paravizor/v1/internal/tool"
	tuictx "github.com/bitravens/paravizor/v1/internal/tui/context"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

// ── Messages ──────────────────────────────────────────────────────────────────

// MsgBack is sent when the user wants to return to the home screen.
type MsgBack struct{}

// msgRunCompleted is sent (internally) when the engine goroutine finishes.
type msgRunCompleted struct{ err error }

// msgEngineEvent carries a single engine event forwarded to the TUI.
type msgEngineEvent struct{ event events.Event }

// ── Internal page state ───────────────────────────────────────────────────────

type pageState int

const (
	stateCreate  pageState = iota // new-project form
	stateProject                  // project view with pipeline nodes + log
)

// nodeRow is the UI representation of a single pipeline node.
type nodeRow struct {
	ID       string
	Name     string
	Tool     string
	Status   engine.NodeStatus
	ItemsIn  int
	ItemsOut int
}

const maxLogLines = 200

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the bubbletea model for the project page.
type Model struct {
	ctx   *tuictx.ProgramContext
	state pageState

	// ── Create form ──
	inputs  []textinput.Model
	focus   int
	formErr error

	// ── Project state ──
	projectDir string
	projCfg    *proj.ProjectConfig
	nodes      []nodeRow
	logLines   []string
	running    bool
	runErr     error

	// channels shared with goroutine (allocated once per run)
	eventCh chan events.Event
	doneCh  chan error
	cancel  gocontext.CancelFunc

	// layout
	width  int
	height int
}

// ProjectDir returns the active project directory for this view.
func (m Model) ProjectDir() string {
	return m.projectDir
}

// ── Construction ──────────────────────────────────────────────────────────────

// NewModel creates a project page. If existingDir is non-empty the form is
// skipped and the project is loaded directly.
func NewModel(ctx *tuictx.ProgramContext, existingDir string) Model {
	m := Model{
		ctx:    ctx,
		state:  stateCreate,
		width:  ctx.Window.Width,
		height: ctx.Window.Height,
	}

	m.inputs = buildInputs(ctx)

	if existingDir != "" {
		m.loadExistingProject(existingDir)
	}

	return m
}

// buildInputs constructs the four form text inputs.
func buildInputs(ctx *tuictx.ProgramContext) []textinput.Model {
	type fieldSpec struct {
		prompt, placeholder, value string
	}

	defaultBase := os.Getenv("HOME") + "/recon"
	pipeline := ctx.Config.DefaultPipeline
	if pipeline == "" {
		pipeline = "default"
	}

	specs := []fieldSpec{
		{"Project Name: ", "my-target", ""},
		{"Target Domains: ", "example.com, *.example.com", ""},
		{"Base Directory: ", defaultBase, defaultBase},
		{"Pipeline: ", "default", pipeline},
	}

	inputs := make([]textinput.Model, len(specs))
	for i, s := range specs {
		t := textinput.New()

		styles := textinput.DefaultDarkStyles()
		styles.Focused.Prompt = lipgloss.NewStyle().Foreground(ctx.Theme.WarningText)
		styles.Focused.Text = lipgloss.NewStyle().Foreground(ctx.Theme.PrimaryText)
		styles.Blurred.Prompt = lipgloss.NewStyle().Foreground(ctx.Theme.SecondaryText)
		styles.Blurred.Text = lipgloss.NewStyle().Foreground(ctx.Theme.PrimaryText)
		styles.Cursor.Color = ctx.Theme.WarningText

		t.SetStyles(styles)
		t.Prompt = s.prompt
		t.Placeholder = s.placeholder
		t.SetValue(s.value)
		if i == 0 {
			t.Focus()
		}
		inputs[i] = t
	}

	return inputs
}

// loadExistingProject transitions directly to stateProject using an existing dir.
func (m *Model) loadExistingProject(dir string) {
	pcfg, err := proj.LoadProject(dir)
	if err != nil {
		m.formErr = fmt.Errorf("load project: %w", err)
		return
	}
	m.projectDir = dir
	m.projCfg = &pcfg
	m.state = stateProject
	m.rebuildNodes()
}

// rebuildNodes syncs m.nodes from ctx.Pipeline.Nodes.
func (m *Model) rebuildNodes() {
	if m.ctx.Pipeline == nil {
		return
	}
	m.nodes = make([]nodeRow, len(m.ctx.Pipeline.Nodes))
	for i, n := range m.ctx.Pipeline.Nodes {
		m.nodes[i] = nodeRow{
			ID:     n.ID,
			Name:   n.Name,
			Tool:   n.Tool,
			Status: engine.NodeStatusIdle,
		}
	}
}

// nodeIndex returns the index into m.nodes for the given ID, or -1.
func (m *Model) nodeIndex(id string) int {
	for i, n := range m.nodes {
		if n.ID == id {
			return i
		}
	}
	return -1
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		cmds = append(cmds, m.handleKey(msg)...)

	case msgEngineEvent:
		m.applyEvent(msg.event)
		// Re-arm the listener so the next event is forwarded.
		cmds = append(cmds, waitEvent(m.eventCh, m.doneCh))

	case msgRunCompleted:
		m.running = false
		if msg.err != nil {
			m.runErr = msg.err
			m.appendLog("ERROR: " + msg.err.Error())
		} else {
			m.appendLog("Pipeline completed.")
		}
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
	}

	// Forward to text inputs when in create form.
	if m.state == stateCreate {
		for i := range m.inputs {
			t, cmd := m.inputs[i].Update(msg)
			m.inputs[i] = t
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleKey(msg tea.KeyMsg) []tea.Cmd {
	switch m.state {
	case stateCreate:
		return m.handleCreateKey(msg)
	case stateProject:
		return m.handleProjectKey(msg)
	}
	return nil
}

func (m *Model) handleCreateKey(msg tea.KeyMsg) []tea.Cmd {
	switch msg.String() {
	case "esc":
		return []tea.Cmd{func() tea.Msg { return MsgBack{} }}

	case "tab", "down":
		m.nextInput()

	case "shift+tab", "up":
		m.prevInput()

	case "enter":
		if m.focus < len(m.inputs)-1 {
			m.nextInput()
		} else {
			// Last field: create the project.
			return []tea.Cmd{m.submitCreate()}
		}

	case "ctrl+r":
		return []tea.Cmd{m.submitCreate()}
	}
	return nil
}

func (m *Model) handleProjectKey(msg tea.KeyMsg) []tea.Cmd {
	switch msg.String() {
	case "esc":
		if m.running {
			m.stopRun()
			return nil
		}
		return []tea.Cmd{func() tea.Msg { return MsgBack{} }}

	case "ctrl+r":
		if !m.running {
			return []tea.Cmd{m.startRun()}
		}
	}
	return nil
}

// ── Create project ────────────────────────────────────────────────────────────

// submitCreate validates the form, creates the project on disk, and transitions
// to stateProject. Returns a tea.Cmd that drives the transition message.
func (m *Model) submitCreate() tea.Cmd {
	name := strings.TrimSpace(m.inputs[0].Value())
	if name == "" {
		m.formErr = fmt.Errorf("project name is required")
		return nil
	}

	baseDir := strings.TrimSpace(m.inputs[2].Value())
	if baseDir == "" {
		baseDir = os.Getenv("HOME") + "/recon"
	}

	domainsRaw := strings.TrimSpace(m.inputs[1].Value())
	var includes []string
	for _, d := range strings.FieldsFunc(domainsRaw, func(r rune) bool { return r == ',' || r == ' ' }) {
		if d = strings.TrimSpace(d); d != "" {
			includes = append(includes, d)
		}
	}

	pipeline := strings.TrimSpace(m.inputs[3].Value())
	if pipeline == "" {
		pipeline = "default"
	}

	scope := proj.ScopeConfig{Include: includes}
	cfg, err := proj.CreateProject(name, "", baseDir, pipeline, "", nil, scope)
	if err != nil {
		m.formErr = err
		return nil
	}

	dir, err := proj.InitProject(baseDir, *cfg)
	if err != nil {
		m.formErr = err
		return nil
	}

	m.projectDir = dir
	m.projCfg = cfg
	m.formErr = nil
	m.state = stateProject
	m.rebuildNodes()
	m.appendLog(fmt.Sprintf("Project created at %s", dir))

	// Persist as a recent project.
	m.addRecentProject(dir)

	return nil
}

// addRecentProject prepends dir to ctx.Config.RecentProjects (max 10).
func (m *Model) addRecentProject(dir string) {
	cfg := m.ctx.Config
	// Deduplicate.
	filtered := make([]string, 0, len(cfg.RecentProjects))
	for _, p := range cfg.RecentProjects {
		if p != dir {
			filtered = append(filtered, p)
		}
	}
	cfg.RecentProjects = append([]string{dir}, filtered...)
	if len(cfg.RecentProjects) > 10 {
		cfg.RecentProjects = cfg.RecentProjects[:10]
	}
	// Best-effort save; ignore errors here.
	if path, err := config.GetGlobalConfigPath(); err == nil {
		_ = config.WriteConfig(path, *cfg)
	}
}

// ── Run pipeline ──────────────────────────────────────────────────────────────

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

// appendLog adds a line to the log, capping at maxLogLines.
func (m *Model) appendLog(line string) {
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > maxLogLines {
		m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
	}
}

// ── Focus helpers ─────────────────────────────────────────────────────────────

func (m *Model) nextInput() {
	m.focus = (m.focus + 1) % len(m.inputs)
	m.updateFocus()
}

func (m *Model) prevInput() {
	m.focus--
	if m.focus < 0 {
		m.focus = len(m.inputs) - 1
	}
	m.updateFocus()
}

func (m *Model) updateFocus() {
	for i := range m.inputs {
		if i == m.focus {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	switch m.state {
	case stateCreate:
		return m.viewCreate()
	case stateProject:
		return m.viewProject()
	}
	return ""
}

// ── Create form view ──────────────────────────────────────────────────────────

func (m Model) viewCreate() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.PrimaryText).
		Background(m.ctx.Theme.SelectedBackground).
		Padding(0, 1).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("New Project"))
	b.WriteString("\n\n")

	for i, inp := range m.inputs {
		b.WriteString(inp.View())
		if i < len(m.inputs)-1 {
			b.WriteString("\n\n")
		}
	}

	helpStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("↑/↓ Tab: Navigate  •  Enter/ctrl+r: Create  •  Esc: Back"))

	if m.formErr != nil {
		errStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText)
		b.WriteString("\n\n")
		b.WriteString(errStyle.Render(fmt.Sprintf("Error: %v", m.formErr)))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.ctx.Theme.PrimaryBorder).
		Padding(1, 4).
		Render(b.String())
}

// ── Project view ──────────────────────────────────────────────────────────────

func (m Model) viewProject() string {
	// Calculate panel widths: 40% nodes, 60% log.
	totalW := m.width
	if totalW < 10 {
		totalW = 80
	}
	leftW := totalW * 40 / 100
	rightW := totalW - leftW - 3 // 3 = lipgloss join spacing
	if rightW < 10 {
		rightW = 10
	}

	// Inner heights: subtract border (2) + title (1) + padding (2) + status bar (2).
	innerH := m.height - 7
	if innerH < 4 {
		innerH = 4
	}

	left := m.renderNodes(leftW, innerH)
	right := m.renderLog(rightW, innerH)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	status := m.renderStatus(totalW)

	return lipgloss.JoinVertical(lipgloss.Left, panels, status)
}

func (m Model) renderNodes(w, h int) string {
	borderColor := m.ctx.Theme.PrimaryBorder
	if m.running {
		borderColor = m.ctx.Theme.SuccessText
	}

	title := "Pipeline Nodes"
	if m.projCfg != nil {
		title = m.projCfg.Name + " — nodes"
	}

	var b strings.Builder
	for _, n := range m.nodes {
		icon, color := nodeStatusIcon(n.Status, m.ctx)
		iconStyle := lipgloss.NewStyle().Foreground(color)
		nameStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText)
		dimStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)

		line := fmt.Sprintf("%s %-16s %-12s",
			iconStyle.Render(icon),
			nameStyle.Render(n.Name),
			dimStyle.Render(n.Tool),
		)
		if n.Status == engine.NodeStatusCompleted {
			line += dimStyle.Render(fmt.Sprintf(" (%d→%d)", n.ItemsIn, n.ItemsOut))
		}
		b.WriteString(line + "\n")
	}

	if len(m.nodes) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
			Render("No pipeline nodes loaded."))
	}

	inner := b.String()
	// Trim to fit height.
	lines := strings.Split(strings.TrimRight(inner, "\n"), "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	inner = strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Width(w).
		Height(h+2). // +2 for border
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(lipgloss.NewStyle().
			Foreground(m.ctx.Theme.PrimaryText).
			Background(m.ctx.Theme.SelectedBackground).
			Padding(0, 1).
			Render(title) + "\n" + inner)
}

func (m Model) renderLog(w, h int) string {
	var lines []string
	if len(m.logLines) > h {
		lines = m.logLines[len(m.logLines)-h:]
	} else {
		lines = m.logLines
	}

	logText := strings.Join(lines, "\n")
	if logText == "" {
		logText = lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
			Render("No output yet.")
	}

	return lipgloss.NewStyle().
		Width(w).
		Height(h+2). // +2 for border
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.ctx.Theme.PrimaryBorder).
		Padding(0, 1).
		Render(lipgloss.NewStyle().
			Foreground(m.ctx.Theme.PrimaryText).
			Background(m.ctx.Theme.SelectedBackground).
			Padding(0, 1).
			Render("Log") + "\n" + logText)
}

func (m Model) renderStatus(w int) string {
	var parts []string

	if m.projCfg != nil {
		parts = append(parts, fmt.Sprintf("Project: %s", m.projCfg.Name))
		if len(m.projCfg.Scope.Include) > 0 {
			parts = append(parts, fmt.Sprintf("Scope: %s", strings.Join(m.projCfg.Scope.Include, ", ")))
		}
	}

	statusPart := "Ready"
	if m.running {
		statusPart = lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText).Render("Running...")
	}
	if m.runErr != nil {
		statusPart = lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText).
			Render("Error: " + m.runErr.Error())
	}
	parts = append(parts, statusPart)

	left := strings.Join(parts, "  •  ")

	helpStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	var helpParts []string
	if !m.running {
		helpParts = append(helpParts, "ctrl+r: Run")
	} else {
		helpParts = append(helpParts, "esc: Stop")
	}
	helpParts = append(helpParts, "esc: Back")
	right := helpStyle.Render(strings.Join(helpParts, "  "))

	// Pad to fill width.
	leftStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.SecondaryText)
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return leftStyle.Render(left) + strings.Repeat(" ", gap) + right
}

// nodeStatusIcon returns the display icon and themed colour for a node status.
func nodeStatusIcon(s engine.NodeStatus, ctx *tuictx.ProgramContext) (icon string, c color.Color) {
	switch s {
	case engine.NodeStatusActive:
		return "▶", ctx.Theme.WarningText
	case engine.NodeStatusCompleted:
		return "✓", ctx.Theme.SuccessText
	case engine.NodeStatusError:
		return "✗", ctx.Theme.ErrorText
	case engine.NodeStatusDraining:
		return "~", ctx.Theme.SecondaryText
	default: // idle
		return "○", ctx.Theme.FaintText
	}
}
