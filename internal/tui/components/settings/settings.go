package settings

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bitravens/paravizor/v1/internal/config"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
)

type MsgSaveConfig struct{}
type MsgCancel struct{}

const totalFields = 5 // 4 text inputs + 1 dropdown

type Model struct {
	ctx     *context.ProgramContext
	inputs  []textinput.Model
	logOpts []string // dropdown options
	logIdx  int      // current log level index
	focus   int
	err     error
}

func NewModel(ctx *context.ProgramContext) Model {
	logOpts := []string{"debug", "info", "warn", "error"}
	logIdx := 0
	for i, o := range logOpts {
		if o == ctx.Config.LogLevel {
			logIdx = i
			break
		}
	}

	prompts := []struct{ label, placeholder, value string }{
		{"Theme: ", "default", ctx.Config.Theme},
		{"Default Pipeline: ", "default", ctx.Config.DefaultPipeline},
		{"Max Processes: ", "10", fmt.Sprintf("%d", ctx.Config.MaxProcesses)},
		{"Healthcheck Interval (s): ", "10", fmt.Sprintf("%d", ctx.Config.HealthCheckInterval)},
	}

	inputs := make([]textinput.Model, len(prompts))
	for i, p := range prompts {
		t := ctx.StyledInput()
		t.Prompt = p.label
		t.Placeholder = p.placeholder
		t.SetValue(p.value)
		if i == 0 {
			t.Focus()
		}
		inputs[i] = t
	}

	return Model{ctx: ctx, inputs: inputs, logOpts: logOpts, logIdx: logIdx}
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+s":
			return m.save()
		case "enter":
			if m.focus < totalFields-1 {
				m.setFocus((m.focus + 1) % totalFields)
				return m, nil
			}
			return m.save()
		case "esc":
			return m, func() tea.Msg { return MsgCancel{} }
		case "up", "shift+tab":
			f := m.focus - 1
			if f < 0 {
				f = totalFields - 1
			}
			m.setFocus(f)
			return m, nil
		case "down", "tab":
			m.setFocus((m.focus + 1) % totalFields)
			return m, nil
		case "left":
			if m.focus == totalFields-1 {
				m.logIdx = (m.logIdx - 1 + len(m.logOpts)) % len(m.logOpts)
				return m, nil
			}
		case "right":
			if m.focus == totalFields-1 {
				m.logIdx = (m.logIdx + 1) % len(m.logOpts)
				return m, nil
			}
		}
	}
	if m.focus < len(m.inputs) {
		t, cmd := m.inputs[m.focus].Update(msg)
		m.inputs[m.focus] = t
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) setFocus(f int) {
	m.focus = f
	for i := range m.inputs {
		if i == f {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m Model) save() (Model, tea.Cmd) {
	m.ctx.Config.Theme = strings.TrimSpace(m.inputs[0].Value())
	m.ctx.Config.DefaultPipeline = strings.TrimSpace(m.inputs[1].Value())
	if v, err := strconv.Atoi(strings.TrimSpace(m.inputs[2].Value())); err == nil {
		m.ctx.Config.MaxProcesses = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(m.inputs[3].Value())); err == nil {
		m.ctx.Config.HealthCheckInterval = v
	}
	m.ctx.Config.LogLevel = m.logOpts[m.logIdx]

	path, err := config.GetGlobalConfigPath()
	if err == nil {
		err = config.WriteConfig(path, *m.ctx.Config)
	}
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, func() tea.Msg { return MsgSaveConfig{} }
}

func (m Model) View() string {
	th := m.ctx.Theme
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(th.PrimaryText).
		Background(th.SelectedBackground).
		Padding(0, 1).
		MarginBottom(1)
	b.WriteString(titleStyle.Render("Global Settings"))
	b.WriteString("\n\n")

	for i := range m.inputs {
		b.WriteString(m.inputs[i].View())
		b.WriteString("\n\n")
	}

	// Render log level dropdown inline
	dropdownFocused := m.focus == totalFields-1
	var labelStyle, arrowStyle lipgloss.Style
	if dropdownFocused {
		labelStyle = lipgloss.NewStyle().Foreground(th.WarningText)
		arrowStyle = lipgloss.NewStyle().Foreground(th.WarningText)
	} else {
		labelStyle = lipgloss.NewStyle().Foreground(th.PrimaryText)
		arrowStyle = lipgloss.NewStyle().Foreground(th.FaintText)
	}
	var opts []string
	for i, o := range m.logOpts {
		if i == m.logIdx {
			opts = append(opts, lipgloss.NewStyle().Foreground(th.PrimaryText).Bold(dropdownFocused).Render(o))
		} else {
			opts = append(opts, lipgloss.NewStyle().Foreground(th.FaintText).Render(o))
		}
	}
	sep := lipgloss.NewStyle().Foreground(th.FaintText).Render(" / ")
	b.WriteString(labelStyle.Render("Log Level: ") + arrowStyle.Render("◀ ") + strings.Join(opts, sep) + arrowStyle.Render(" ▶"))
	b.WriteString("\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(th.FaintText).Render("↑/↓ tab: navigate  ◀/▶: cycle dropdown  ctrl+s: save  esc: cancel"))

	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(th.ErrorText).Render(fmt.Sprintf("Error saving: %v", m.err)))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.PrimaryBorder).
		Padding(1, 4).
		Render(b.String())
}
