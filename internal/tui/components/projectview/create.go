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

func buildInputs(ctx *tuictx.ProgramContext) []textinput.Model {
	pipeline := ctx.Config.DefaultPipeline
	if pipeline == "" {
		pipeline = "default"
	}
	home := os.Getenv("HOME") + "/recon"
	specs := []struct{ prompt, placeholder, value string }{
		{"Project Name: ", "my-target", ""},
		{"Target Domains: ", "example.com, *.example.com", ""},
		{"Base Directory: ", home, home},
		{"Pipeline: ", "default", pipeline},
	}
	inputs := make([]textinput.Model, len(specs))
	for i, s := range specs {
		t := ctx.StyledInput()
		t.Prompt, t.Placeholder = s.prompt, s.placeholder
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
		m.focus = (m.focus + 1) % len(m.inputs)
		m.setFocus()
	case "shift+tab", "up":
		m.focus--
		if m.focus < 0 {
			m.focus = len(m.inputs) - 1
		}
		m.setFocus()
	case "enter":
		if m.focus < len(m.inputs)-1 {
			m.focus++
			m.setFocus()
		} else {
			return []tea.Cmd{m.submitCreate()}
		}
	case "ctrl+r":
		return []tea.Cmd{m.submitCreate()}
	}
	return nil
}

func (m *Model) setFocus() {
	for i := range m.inputs {
		if i == m.focus {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

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
	var includes []string
	for _, d := range strings.FieldsFunc(strings.TrimSpace(m.inputs[1].Value()), func(r rune) bool { return r == ',' || r == ' ' }) {
		if d = strings.TrimSpace(d); d != "" {
			includes = append(includes, d)
		}
	}
	pipeline := strings.TrimSpace(m.inputs[3].Value())
	if pipeline == "" {
		pipeline = "default"
	}
	cfg, err := proj.CreateProject(name, "", baseDir, pipeline, "", nil, proj.ScopeConfig{Include: includes})
	if err != nil {
		m.formErr = err
		return nil
	}
	dir, err := proj.InitProject(baseDir, *cfg)
	if err != nil {
		m.formErr = err
		return nil
	}
	m.projectDir, m.projCfg, m.formErr, m.state = dir, cfg, nil, stateProject
	m.rebuildNodes()
	m.appendLog(fmt.Sprintf("Project created at %s", dir))
	if path, err := config.GetGlobalConfigPath(); err == nil {
		cfg2 := m.ctx.Config
		filtered := make([]string, 0, len(cfg2.RecentProjects))
		for _, p := range cfg2.RecentProjects {
			if p != dir {
				filtered = append(filtered, p)
			}
		}
		cfg2.RecentProjects = append([]string{dir}, filtered...)
		if len(cfg2.RecentProjects) > 10 {
			cfg2.RecentProjects = cfg2.RecentProjects[:10]
		}
		_ = config.WriteConfig(path, *cfg2)
	}
	return nil
}

func (m Model) viewCreate() string {
	th := m.ctx.Theme
	faint := lipgloss.NewStyle().Foreground(th.FaintText)
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(th.WarningText).Bold(true).Render("New Project") + "\n\n")
	for i, inp := range m.inputs {
		b.WriteString(inp.View())
		if i < len(m.inputs)-1 {
			b.WriteString("\n\n")
		}
	}
	b.WriteString("\n\n" + faint.Render("↑/↓ Tab: Navigate  •  Enter/ctrl+r: Create  •  Esc: Back"))
	if m.formErr != nil {
		b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(th.ErrorText).Render("Error: "+m.formErr.Error()))
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}
