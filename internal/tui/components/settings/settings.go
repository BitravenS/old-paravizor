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

// ── Inline dropdown ───────────────────────────────────────────────────────────

type dropdown struct {
	label   string
	options []string
	idx     int
	focused bool
}

func newDropdown(label string, options []string, current string) dropdown {
	d := dropdown{label: label, options: options}
	for i, o := range options {
		if o == current {
			d.idx = i
			break
		}
	}
	return d
}

func (d *dropdown) next() {
	d.idx = (d.idx + 1) % len(d.options)
}
func (d *dropdown) prev() {
	d.idx--
	if d.idx < 0 {
		d.idx = len(d.options) - 1
	}
}
func (d dropdown) value() string {
	if len(d.options) == 0 {
		return ""
	}
	return d.options[d.idx]
}

func (d dropdown) render(ctx *context.ProgramContext) string {
	th := ctx.Theme
	var labelStyle, valStyle, arrowStyle, dimStyle lipgloss.Style
	if d.focused {
		labelStyle = lipgloss.NewStyle().Foreground(th.WarningText)
		valStyle = lipgloss.NewStyle().Foreground(th.PrimaryText).Bold(true)
		arrowStyle = lipgloss.NewStyle().Foreground(th.WarningText)
	} else {
		labelStyle = lipgloss.NewStyle().Foreground(th.PrimaryText)
		valStyle = lipgloss.NewStyle().Foreground(th.SecondaryText)
		arrowStyle = lipgloss.NewStyle().Foreground(th.FaintText)
	}
	dimStyle = lipgloss.NewStyle().Foreground(th.FaintText)

	var opts []string
	for i, o := range d.options {
		if i == d.idx {
			opts = append(opts, valStyle.Render(o))
		} else {
			opts = append(opts, dimStyle.Render(o))
		}
	}
	bar := arrowStyle.Render("◀ ") + strings.Join(opts, dimStyle.Render(" / ")) + arrowStyle.Render(" ▶")
	return labelStyle.Render(d.label) + bar
}

// ── Model ─────────────────────────────────────────────────────────────────────

// totalFields = 4 text inputs + 1 dropdown
const totalFields = 5

type Model struct {
	ctx      *context.ProgramContext
	inputs   []textinput.Model
	logLevel dropdown
	focus    int // 0..3 = text inputs, 4 = dropdown
	err      error
}

func NewModel(ctx *context.ProgramContext) Model {
	m := Model{
		ctx:      ctx,
		inputs:   make([]textinput.Model, 4),
		logLevel: newDropdown("Log Level: ", []string{"debug", "info", "warn", "error"}, ctx.Config.LogLevel),
	}

	prompts := []struct{ label, placeholder, value string }{
		{"Theme: ", "default", ctx.Config.Theme},
		{"Default Pipeline: ", "default", ctx.Config.DefaultPipeline},
		{"Max Processes: ", "10", fmt.Sprintf("%d", ctx.Config.MaxProcesses)},
		{"Healthcheck Interval (s): ", "10", fmt.Sprintf("%d", ctx.Config.HealthCheckInterval)},
	}

	for i := range m.inputs {
		t := textinput.New()
		st := textinput.DefaultDarkStyles()
		st.Focused.Prompt = lipgloss.NewStyle().Foreground(ctx.Theme.WarningText)
		st.Focused.Text = lipgloss.NewStyle().Foreground(ctx.Theme.PrimaryText)
		st.Blurred.Prompt = lipgloss.NewStyle().Foreground(ctx.Theme.PrimaryText)
		st.Blurred.Text = lipgloss.NewStyle().Foreground(ctx.Theme.SecondaryText)
		st.Cursor.Color = ctx.Theme.WarningText
		t.SetStyles(st)
		t.Prompt = prompts[i].label
		t.Placeholder = prompts[i].placeholder
		t.SetValue(prompts[i].value)
		if i == 0 {
			t.Focus()
		}
		m.inputs[i] = t
	}
	return m
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			return m.save()

		case "enter":
			if m.focus < totalFields-1 {
				m.nextField()
				return m, nil
			}
			return m.save()

		case "esc":
			return m, func() tea.Msg { return MsgCancel{} }

		case "up", "shift+tab":
			m.prevField()
			return m, nil

		case "down", "tab":
			m.nextField()
			return m, nil

		case "left":
			if m.focus == totalFields-1 {
				m.logLevel.prev()
				return m, nil
			}
		case "right":
			if m.focus == totalFields-1 {
				m.logLevel.next()
				return m, nil
			}
		}
	}

	// Propagate to focused text input
	if m.focus < len(m.inputs) {
		t, cmd := m.inputs[m.focus].Update(msg)
		m.inputs[m.focus] = t
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
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
	m.ctx.Config.LogLevel = m.logLevel.value()

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

func (m *Model) nextField() {
	m.setFocus((m.focus + 1) % totalFields)
}
func (m *Model) prevField() {
	f := m.focus - 1
	if f < 0 {
		f = totalFields - 1
	}
	m.setFocus(f)
}
func (m *Model) setFocus(f int) {
	m.focus = f
	m.logLevel.focused = (f == totalFields-1)
	for i := range m.inputs {
		if i == f {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
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

	b.WriteString(m.logLevel.render(m.ctx))
	b.WriteString("\n\n")

	hintStyle := lipgloss.NewStyle().Foreground(th.FaintText)
	b.WriteString(hintStyle.Render("↑/↓ tab: navigate  ◀/▶: cycle dropdown  ctrl+s: save  esc: cancel"))

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
