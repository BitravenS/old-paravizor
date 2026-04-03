package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/bitravens/paravizor/v1/internal/tui/components/home"
	"github.com/bitravens/paravizor/v1/internal/tui/components/settings"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
)

type ViewState int

const (
	ViewStateHome ViewState = iota
	ViewStateSettings
	ViewStateProject
)

type Model struct {
	Ctx          *context.ProgramContext
	state        ViewState
	homeView     home.Model
	settingsView settings.Model
}

func NewModel(location string) Model {
	ctx, err := context.NewProgramContext(location)
	if err != nil {
		log.Fatal("Failed to initialize program context", "err", err)
	}

	state := ViewStateHome
	if location != "" {
		state = ViewStateProject
	}

	return Model{
		Ctx:          ctx,
		state:        state,
		homeView:     home.NewModel(ctx),
		settingsView: settings.NewModel(ctx),
	}
}

func (m *Model) initScreen() tea.Msg {
	return nil
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		m.initScreen,
		m.homeView.Init(),
		m.settingsView.Init(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			// If we are at home, quit. Otherwise might want to go back.
			if m.state == ViewStateHome {
				return m, tea.Quit
			}
			// But for settings, we only go back on 'esc' or cancel to not break input
		}
	case tea.WindowSizeMsg:
		m.Ctx.Window.Width = msg.Width
		m.Ctx.Window.Height = msg.Height
	case home.ActionMsg:
		switch msg.Action.Type {
		case home.ActionOpenSettings:
			m.state = ViewStateSettings
			return m, nil
		case home.ActionCreateProject:
			m.state = ViewStateProject
			return m, nil
		case home.ActionOpenProject:
			m.state = ViewStateProject
			return m, nil
		}
	case settings.MsgCancel, settings.MsgSaveConfig:
		m.state = ViewStateHome
		return m, nil
	}

	switch m.state {
	case ViewStateHome:
		m.homeView, cmd = m.homeView.Update(msg)
		cmds = append(cmds, cmd)
	case ViewStateSettings:
		m.settingsView, cmd = m.settingsView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
	var content string

	switch m.state {
	case ViewStateHome:
		content = m.homeView.View()
	case ViewStateSettings:
		content = m.settingsView.View()
	case ViewStateProject:
		content = lipgloss.NewStyle().Align(lipgloss.Center).Render("Project View (WIP)\nPress 'ctrl+c' to exit")
	}

	// Center everything on screen
	style := lipgloss.NewStyle().
		Width(m.Ctx.Window.Width).
		Height(m.Ctx.Window.Height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(m.Ctx.Theme.PrimaryText)

	v := tea.NewView(style.Render(content))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
