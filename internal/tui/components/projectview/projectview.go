// Package projectview provides the TUI page for creating and running recon projects.
package projectview

import (
	gocontext "context"
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/events"
	proj "github.com/bitravens/paravizor/v1/internal/project"
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

func (m Model) Focused() bool {
	return m.state == stateCreate
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

// appendLog adds a line to the log, capping at maxLogLines.
func (m *Model) appendLog(line string) {
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > maxLogLines {
		m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
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
