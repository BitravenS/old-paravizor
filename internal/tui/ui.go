package tui

import (
	tea "charm.land/bubbletea/v2"
)

// TODO: Implement the TUI model and views
type Model struct {
}

func NewModel(location string) Model {
	return Model{}
}

func (m *Model) initScreen() tea.Msg {
	return nil
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, m.initScreen)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() tea.View {
	return tea.NewView("placeholder")
}
