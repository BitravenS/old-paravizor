// Package projectview provides the TUI page for creating and running recon projects.
package projectview

import (
	gocontext "context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bitravens/paravizor/v1/internal/config"
	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/events"
	proj "github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/store"
	tuictx "github.com/bitravens/paravizor/v1/internal/tui/context"
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
	stateCreate    pageState = iota // new-project form
	stateProject                    // project view with pipeline nodes + log
	stateEditScope                  // editing in/out of scope
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

type processRow struct {
	ID     int64
	Tool   string
	PID    int
	NodeID string
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

	// ── Dashboard Metrics ──
	domainsCount  int
	liveCount     int
	urlsCount     int
	findingsCount int

	activeProcesses map[int64]processRow
	rateAllocations map[string]float64
	totalBudget     float64

	pipelineCursor int
	processCursor  int
	activePanel    int // 0: pipeline, 1: events, 2: processes
	selectedNode   *nodeRow
	showNodeLogs   bool
	nodeLogsText   string

	// channels shared with goroutine (allocated once per run)
	eventCh chan events.Event
	doneCh  chan error
	cancel  gocontext.CancelFunc

	// layout
	width  int
	height int

	store *store.Store
}

func (m Model) Focused() bool {
	return m.state == stateCreate || m.state == stateEditScope
}

// ProjectDir returns the active project directory for this view.
func (m Model) ProjectDir() string {
	return m.projectDir
}

// ── Construction ──────────────────────────────────────────────────────────────

// NewModel creates a project page. If existingDir is non-empty the form is
// skipped and the project is loaded directly.
func NewModel(ctx *tuictx.ProgramContext, existingDir string, st *store.Store) Model {
	m := Model{
		ctx:             ctx,
		store:           st,
		state:           stateCreate,
		width:           ctx.Window.Width,
		height:          ctx.Window.Height,
		activeProcesses: make(map[int64]processRow),
		rateAllocations: make(map[string]float64),
	}

	m.inputs = buildInputs(ctx)

	if existingDir != "" {
		m.loadExistingProject(existingDir)
	}

	return m
}

// loadExistingProject transitions directly to stateProject using an existing dir.
func (m *Model) loadExistingProject(dir string) {
	pcfg, err := proj.LoadProject(dir)
	if err != nil {
		m.formErr = fmt.Errorf("load project: %w", err)
		return
	}
	if err := m.syncProjectContext(dir, &pcfg); err != nil {
		m.formErr = err
		return
	}
	m.projectDir = dir
	m.projCfg = &pcfg
	m.state = stateProject
	m.rebuildNodes()
	m.loadMetricsFromStore()
}

func (m *Model) syncProjectContext(dir string, pcfg *proj.ProjectConfig) error {
	cfg := config.LoadConfig(dir)
	m.ctx.Config = &cfg
	m.ctx.Project = pcfg
	m.ctx.ProjectDir = dir

	pipelineName := cfg.DefaultPipeline
	if pcfg != nil && pcfg.Pipeline != "" {
		pipelineName = pcfg.Pipeline
	}
	pipeline, err := engine.LoadExternalPipeline(pipelineName)
	if err != nil && pipeline == nil {
		return fmt.Errorf("load pipeline %q: %w", pipelineName, err)
	}
	m.ctx.Pipeline = pipeline
	return nil
}

// loadMetricsFromStore hydrates the TUI counts from the database on resume.
func (m *Model) loadMetricsFromStore() {
	st, closeStore, err := m.metricsStore()
	if err != nil {
		m.appendLog("Metrics unavailable: " + err.Error())
		return
	}
	defer closeStore()

	ctx := gocontext.Background()
	var count int

	_ = st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM domains").Scan(&count)
	m.domainsCount = count

	_ = st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM domains WHERE source = 'dnsx-live'").Scan(&count)
	m.liveCount = count

	_ = st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM urls").Scan(&count)
	m.urlsCount = count

	_ = st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM findings").Scan(&count)
	m.findingsCount = count

	for i, n := range m.nodes {
		var outCount int
		_ = st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM pipeline_state WHERE node_id = ? AND status = 'completed'", n.ID).Scan(&outCount)
		m.nodes[i].ItemsOut = outCount

		if outCount > 0 {
			m.nodes[i].Status = engine.NodeStatusCompleted
		}
	}
}

func (m *Model) metricsStore() (*store.Store, func(), error) {
	if m.store != nil && m.store.DB() != nil {
		return m.store, func() {}, nil
	}
	if m.projectDir == "" {
		return nil, nil, fmt.Errorf("project directory is not set")
	}

	dbCfg := store.DBConfig{}
	if m.ctx != nil && m.ctx.Config != nil && m.ctx.Config.DBConfig != nil {
		dbCfg = *m.ctx.Config.DBConfig
	}
	st, err := store.Open(gocontext.Background(), proj.DBPath(m.projectDir), dbCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("open project database: %w", err)
	}
	return st, func() { _ = st.Close() }, nil
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
	case stateEditScope:
		return m.handleEditScopeKey(msg)
	case stateProject:
		return m.handleProjectKey(msg)
	}
	return nil
}

func (m *Model) handleProjectKey(msg tea.KeyMsg) []tea.Cmd {
	if m.showNodeLogs {
		switch msg.String() {
		case "esc", "enter", "q":
			m.showNodeLogs = false
		}
		return nil
	}

	switch msg.String() {
	case "esc":
		if m.running {
			m.stopRun()
			return nil
		}
		return []tea.Cmd{func() tea.Msg { return MsgBack{} }}

	case "e":
		if !m.running && m.projCfg != nil {
			m.state = stateEditScope
			m.inputs = buildScopeInputs(m.ctx, m.projCfg)
			m.focus = 0
		}

	case "ctrl+r":
		if !m.running {
			return []tea.Cmd{m.startRun()}
		}

	case "tab":
		m.activePanel = (m.activePanel + 1) % 3

	case "shift+tab":
		m.activePanel--
		if m.activePanel < 0 {
			m.activePanel = 2
		}

	case "down", "j":
		if m.activePanel == 0 {
			if m.pipelineCursor < len(m.nodes)-1 {
				m.pipelineCursor++
			}
		} else if m.activePanel == 2 {
			if m.processCursor < len(m.activeProcesses)-1 {
				m.processCursor++
			}
		}

	case "up", "k":
		if m.activePanel == 0 {
			if m.pipelineCursor > 0 {
				m.pipelineCursor--
			}
		} else if m.activePanel == 2 {
			if m.processCursor > 0 {
				m.processCursor--
			}
		}

	case "enter":
		if m.activePanel == 0 && m.pipelineCursor < len(m.nodes) {
			node := m.nodes[m.pipelineCursor]
			m.showNodeLogs = true
			m.nodeLogsText = m.loadNodeLogs(node.ID)
		} else if m.activePanel == 2 {
			// Get process log
			if m.processCursor >= 0 && len(m.activeProcesses) > 0 {
				var procs []processRow
				for _, p := range m.activeProcesses {
					procs = append(procs, p)
				}
				sort.Slice(procs, func(i, j int) bool {
					return procs[i].ID < procs[j].ID
				})
				if m.processCursor < len(procs) {
					m.showNodeLogs = true
					m.nodeLogsText = m.loadNodeLogs(procs[m.processCursor].NodeID)
				}
			}
		}
	}
	return nil
}

// appendLog adds a line to the log, capping at maxLogLines.
func (m *Model) appendLog(line string) {
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > maxLogLines {
		m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
	}
}

func (m *Model) loadNodeLogs(nodeID string) string {
	if m.projectDir == "" || nodeID == "" {
		return "No logs available."
	}
	dir := filepath.Join(m.projectDir, "logs", nodeID)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return "No logs found for node " + nodeID
	}

	// Just read the latest stderr/stdout
	var latestFile os.DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			latestFile = e
		}
	}
	if latestFile == nil {
		return "No logs found for node " + nodeID
	}

	path := filepath.Join(dir, latestFile.Name())
	content, err := os.ReadFile(path)
	if err != nil {
		return "Error reading log: " + err.Error()
	}
	return string(content)
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	switch m.state {
	case stateCreate:
		return m.viewCreate()
	case stateEditScope:
		return m.viewEditScope()
	case stateProject:
		return m.viewProject()
	}
	return ""
}
