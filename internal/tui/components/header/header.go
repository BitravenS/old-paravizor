package header

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
)

type MsgChangeTab struct {
	Tab string // "Home" or "Project"
}

type Model struct {
	ctx       *context.ProgramContext
	ActiveTab string
}

func NewModel(ctx *context.ProgramContext) Model {
	tab := "Home"
	if ctx.Project != nil {
		tab = "Project"
	}

	return Model{
		ctx:       ctx,
		ActiveTab: tab,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() string {
	var projectName string
	if m.ctx.Project != nil {
		projectName = m.ctx.Project.Name
	} else {
		projectName = "no-project"
	}

	// Title: ///paravizor/version/project_name///
	titleText := fmt.Sprintf("///paravizor/%s/%s///", m.ctx.Version, projectName)
	titleStyle := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.PrimaryText).
		Bold(true).
		Padding(0, 2)
	title := titleStyle.Render(titleText)

	// Tabs: Home | Project
	var tabs []string

	activeTabStyle := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.SelectedBackground).
		Background(m.ctx.Theme.PrimaryText).
		Padding(0, 1)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.SecondaryText).
		Padding(0, 1)

	if m.ActiveTab == "Home" {
		tabs = append(tabs, activeTabStyle.Render("Home"))
	} else {
		tabs = append(tabs, inactiveTabStyle.Render("Home"))
	}

	if m.ActiveTab == "Project" {
		tabs = append(tabs, activeTabStyle.Render("Project"))
	} else {
		tabs = append(tabs, inactiveTabStyle.Render("Project"))
	}

	tabsView := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// Create a bar
	barStyle := lipgloss.NewStyle().
		Width(m.ctx.Window.Width)

	// Layout: [Title]            [Tabs]
	paddingWidth := m.ctx.Window.Width - lipgloss.Width(title) - lipgloss.Width(tabsView)
	if paddingWidth < 0 {
		paddingWidth = 0
	}

	filler := strings.Repeat(" ", paddingWidth)

	content := lipgloss.JoinHorizontal(lipgloss.Top, title, filler, tabsView)
	return barStyle.Render(content)
}
