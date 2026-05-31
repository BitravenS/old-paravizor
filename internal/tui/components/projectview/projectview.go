// Package projectview provides the TUI page for creating and running recon projects.
package projectview

import (
	gocontext "context"
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/events"
	proj "github.com/bitravens/paravizor/v1/internal/project"
	tuictx "github.com/bitravens/paravizor/v1/internal/tui/context"
)

type MsgBack struct{}
type msgRunCompleted struct{ err error }
type msgEngineEvent struct{ event events.Event }
type pageState int

const (
	stateCreate  pageState = iota
	stateProject pageState = iota
)

type nodeRow struct {
	ID, Name, Tool    string
	Status            engine.NodeStatus
	ItemsIn, ItemsOut int
}

const maxLogLines = 200

type Model struct {
	ctx           *tuictx.ProgramContext
	state         pageState
	inputs        []textinput.Model
	focus         int
	formErr       error
	projectDir    string
	projCfg       *proj.ProjectConfig
	nodes         []nodeRow
	logLines      []string
	running       bool
	runErr        error
	eventCh       chan events.Event
	doneCh        chan error
	cancel        gocontext.CancelFunc
	width, height int
}

func (m Model) ProjectDir() string { return m.projectDir }

func NewModel(ctx *tuictx.ProgramContext, existingDir string) Model {
	m := Model{ctx: ctx, state: stateCreate, width: ctx.Window.Width, height: ctx.Window.Height, inputs: buildInputs(ctx)}
	if existingDir != "" {
		m.loadExistingProject(existingDir)
	}
	return m
}

func (m *Model) loadExistingProject(dir string) {
	pcfg, err := proj.LoadProject(dir)
	if err != nil {
		m.formErr = fmt.Errorf("load project: %w", err)
		return
	}
	m.projectDir, m.projCfg, m.state = dir, &pcfg, stateProject
	m.rebuildNodes()
}

func (m *Model) rebuildNodes() {
	if m.ctx.Pipeline == nil {
		return
	}
	m.nodes = make([]nodeRow, len(m.ctx.Pipeline.Nodes))
	for i, n := range m.ctx.Pipeline.Nodes {
		m.nodes[i] = nodeRow{ID: n.ID, Name: n.Name, Tool: n.Tool, Status: engine.NodeStatusIdle}
	}
}

func (m *Model) nodeIndex(id string) int {
	for i, n := range m.nodes {
		if n.ID == id {
			return i
		}
	}
	return -1
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.state == stateCreate {
			cmds = append(cmds, m.handleCreateKey(msg)...)
		} else {
			cmds = append(cmds, m.handleProjectKey(msg)...)
		}
	case msgEngineEvent:
		m.applyEvent(msg.event)
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
	if m.state == stateCreate {
		for i := range m.inputs {
			t, cmd := m.inputs[i].Update(msg)
			m.inputs[i] = t
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
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

func (m *Model) appendLog(line string) {
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > maxLogLines {
		m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
	}
}

func (m Model) View() string {
	if m.state == stateCreate {
		return m.viewCreate()
	}
	return m.viewProject()
}

func (m Model) viewProject() string {
	th := m.ctx.Theme
	faint := lipgloss.NewStyle().Foreground(th.FaintText)
	hi := lipgloss.NewStyle().Foreground(th.PrimaryText).Bold(true)
	var b strings.Builder

	name := "Project"
	if m.projCfg != nil {
		name = m.projCfg.Name
		if len(m.projCfg.Scope.Include) > 0 {
			name += "  " + faint.Render(strings.Join(m.projCfg.Scope.Include, ", "))
		}
	}
	status := faint.Render("Ready")
	if m.running {
		status = lipgloss.NewStyle().Foreground(th.SuccessText).Render("Running…")
	} else if m.runErr != nil {
		status = lipgloss.NewStyle().Foreground(th.ErrorText).Render("Error: " + m.runErr.Error())
	}
	help := "ctrl+r:Run  esc:Back"
	if m.running {
		help = "esc:Stop"
	}
	b.WriteString(hi.Render(name) + "  " + status + "  " + faint.Render(help) + "\n\n")

	b.WriteString(faint.Render("── nodes ──") + "\n")
	if len(m.nodes) == 0 {
		b.WriteString(faint.Render("  No pipeline loaded.") + "\n")
	} else {
		for _, n := range m.nodes {
			icon, col := nodeStatusIcon(n.Status, m.ctx)
			line := lipgloss.NewStyle().Foreground(col).Render(icon) + " " +
				lipgloss.NewStyle().Foreground(th.PrimaryText).Render(fmt.Sprintf("%-16s", n.Name)) +
				faint.Render(n.Tool)
			if n.Status == engine.NodeStatusCompleted {
				line += faint.Render(fmt.Sprintf(" (%d→%d)", n.ItemsIn, n.ItemsOut))
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n")

	b.WriteString(faint.Render("── log ──") + "\n")
	vis := m.height - 10 - len(m.nodes)
	if vis < 3 {
		vis = 3
	}
	lines := m.logLines
	if len(lines) > vis {
		lines = lines[len(lines)-vis:]
	}
	if len(lines) == 0 {
		b.WriteString(faint.Render("  No output yet.") + "\n")
	} else {
		for _, l := range lines {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

func nodeStatusIcon(s engine.NodeStatus, ctx *tuictx.ProgramContext) (string, color.Color) {
	switch s {
	case engine.NodeStatusActive:
		return "▶", ctx.Theme.WarningText
	case engine.NodeStatusCompleted:
		return "✓", ctx.Theme.SuccessText
	case engine.NodeStatusError:
		return "✗", ctx.Theme.ErrorText
	case engine.NodeStatusDraining:
		return "~", ctx.Theme.SecondaryText
	default:
		return "○", ctx.Theme.FaintText
	}
}
