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

type Model struct {
	ctx    *context.ProgramContext
	inputs []textinput.Model
	focus  int
	err    error
}

func NewModel(ctx *context.ProgramContext) Model {
	m := Model{
		ctx:    ctx,
		inputs: make([]textinput.Model, 5),
	}

	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()

		styles := textinput.DefaultDarkStyles()
		styles.Focused.Prompt = lipgloss.NewStyle().Foreground(context.LogoColor)
		styles.Focused.Text = lipgloss.NewStyle().Foreground(ctx.Theme.PrimaryText)
		styles.Blurred.Prompt = lipgloss.NewStyle().Foreground(ctx.Theme.PrimaryText)
		styles.Blurred.Text = lipgloss.NewStyle().Foreground(ctx.Theme.SecondaryText)
		styles.Cursor.Color = context.LogoColor

		t.SetStyles(styles)

		switch i {
		case 0:
			t.Prompt = "Theme: "
			t.Placeholder = "default"
			t.SetValue(ctx.Config.Theme)
			t.Focus()
		case 1:
			t.Prompt = "Default Pipeline: "
			t.Placeholder = "default"
			t.SetValue(ctx.Config.DefaultPipeline)
		case 2:
			t.Prompt = "Max Concurrent Processes: "
			t.Placeholder = "10"
			t.SetValue(fmt.Sprintf("%d", ctx.Config.MaxProcesses))
		case 3:
			t.Prompt = "Process Healthcheck Interval (s): "
			t.Placeholder = "10"
			t.SetValue(fmt.Sprintf("%d", ctx.Config.HealthCheckInterval))
		case 4:
			t.Prompt = "Log Level (debug/info/warn/error): "
			t.Placeholder = "info"
			t.SetValue(ctx.Config.LogLevel)
		}

		m.inputs[i] = t
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s", "enter":
			// Save config
			if msg.String() == "enter" && m.focus < len(m.inputs)-1 {
				m.nextInput()
				return m, nil
			}

			// Try to save
			m.ctx.Config.Theme = strings.TrimSpace(m.inputs[0].Value())
			m.ctx.Config.DefaultPipeline = strings.TrimSpace(m.inputs[1].Value())

			if v, err := strconv.Atoi(strings.TrimSpace(m.inputs[2].Value())); err == nil {
				m.ctx.Config.MaxProcesses = v
			}
			if v, err := strconv.Atoi(strings.TrimSpace(m.inputs[3].Value())); err == nil {
				m.ctx.Config.HealthCheckInterval = v
			}

			m.ctx.Config.LogLevel = strings.TrimSpace(m.inputs[4].Value())

			path, err := config.GetGlobalConfigPath()
			if err == nil {
				err = config.WriteConfig(path, *m.ctx.Config)
			}

			if err != nil {
				m.err = err
				return m, nil
			}

			// Successfully saved
			return m, func() tea.Msg { return MsgSaveConfig{} }

		case "esc":
			return m, func() tea.Msg { return MsgCancel{} }

		case "up", "shift+tab":
			m.prevInput()
		case "down", "tab":
			m.nextInput()
		}

	case tea.WindowSizeMsg:
		// Not strictly needed but keeping it for structure
	}

	for i := range m.inputs {
		t, cmd := m.inputs[i].Update(msg)
		m.inputs[i] = t
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

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
	for i := 0; i < len(m.inputs); i++ {
		if i == m.focus {
			m.inputs[i].Focus()
			continue
		}
		m.inputs[i].Blur()
	}
}

func (m Model) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.PrimaryText).
		Background(m.ctx.Theme.SelectedBackground).
		Padding(0, 1).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Global Settings"))
	b.WriteString("\n\n")

	for i := range m.inputs {
		b.WriteString(m.inputs[i].View())
		if i < len(m.inputs)-1 {
			b.WriteRune('\n')
			b.WriteRune('\n')
		}
	}

	b.WriteString("\n\n")
	helpStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	b.WriteString(helpStyle.Render("↑/↓: Navigate • Enter: Next/Save • Esc: Cancel"))

	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText)
		b.WriteString("\n\n")
		b.WriteString(errStyle.Render(fmt.Sprintf("Error saving: %v", m.err)))
	}

	// Calculate total height to center manually or let the parent container center it
	formView := b.String()

	// Add padding around the form
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.ctx.Theme.PrimaryBorder).
		Padding(1, 4).
		Render(formView)
}
