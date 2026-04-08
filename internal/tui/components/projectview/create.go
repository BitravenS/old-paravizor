package projectview

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bitravens/paravizor/v1/internal/config"
	proj "github.com/bitravens/paravizor/v1/internal/project"
	tuictx "github.com/bitravens/paravizor/v1/internal/tui/context"
)

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
