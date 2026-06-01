package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/bitravens/paravizor/v1/internal/tui/components/footer"
	"github.com/bitravens/paravizor/v1/internal/tui/components/home"
	"github.com/bitravens/paravizor/v1/internal/tui/components/popup"
	"github.com/bitravens/paravizor/v1/internal/tui/components/projectview"
	"github.com/bitravens/paravizor/v1/internal/tui/components/settings"
	"github.com/bitravens/paravizor/v1/internal/tui/components/sidebar"
	"github.com/bitravens/paravizor/v1/internal/tui/components/toaster"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
)

type ViewState int

const (
	ViewStateHome ViewState = iota
	ViewStateProject
)

type Model struct {
	Ctx         *context.ProgramContext
	store       *store.Store
	state       ViewState
	sidebarView sidebar.Model
	homeView    home.Model
	projectView projectview.Model
	footerView  footer.Model
	popupView   popup.Model
	toasterView toaster.Model
	showFooter  bool
}

func NewModel(location, version string, s *store.Store) Model {
	ctx, err := context.NewProgramContext(location)
	if err != nil {
		log.Fatal("Failed to initialize program context", "err", err)
	}
	ctx.Version = version

	state := ViewStateHome
	showFooter := false
	projInitDir := ""
	if location != "" {
		state = ViewStateProject
		showFooter = true
		projInitDir = location
	}

	return Model{
		Ctx:         ctx,
		store:       s,
		state:       state,
		showFooter:  showFooter,
		sidebarView: sidebar.NewModel(ctx),
		homeView:    home.NewModel(ctx),
		projectView: projectview.NewModel(ctx, projInitDir, s),
		footerView:  footer.NewModel(ctx, s),
		popupView:   popup.NewModel(ctx),
		toasterView: toaster.NewModel(ctx),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		m.homeView.Init(),
		m.projectView.Init(),
		m.footerView.Init(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Toaster gets every message.
	if tcmd := m.toasterView.Update(msg); tcmd != nil {
		cmds = append(cmds, tcmd)
	}

	// Popup consumes all input while open.
	if m.popupView.IsVisible() {
		handled, pcmd := m.popupView.Update(msg)
		if pcmd != nil {
			cmds = append(cmds, pcmd)
		}
		if handled {
			return m, tea.Batch(cmds...)
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "q":
			if m.isInputFocused() {
				break
			}
			if m.state == ViewStateHome {
				return m, tea.Quit
			}

		case "f":
			if m.isInputFocused() {
				break
			}
			m.showFooter = !m.showFooter
			m.recalculateLayout()
			return m, nil

		case "?":
			if m.isInputFocused() {
				break
			}
			helpText := m.renderHelp(m.state)
			m.popupView.Show("Help", helpText, "esc  close")
			return m, nil

		case "s":
			if m.isInputFocused() {
				break
			}
			// Settings popup — fresh model each time so fields reflect current config.
			sm := settings.NewModel(m.Ctx)
			m.popupView.Show("Settings", sm.View(), "ctrl+s save  esc cancel")
			return m, sm.Init()

		case "ctrl+r":
			if m.state == ViewStateProject {
				if m.isInputFocused() {
					break
				}
				if dir := m.projectView.ProjectDir(); dir != "" {
					m.Ctx.ProjectDir = dir
					if p, err := project.LoadProject(dir); err == nil {
						m.Ctx.Project = &p
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.Ctx.Window.Width = msg.Width
		m.Ctx.Window.Height = msg.Height
		m.recalculateLayout()

	// Sidebar: user selected a recent project.
	case sidebar.ProjectSelectedMsg:
		m.projectView = projectview.NewModel(m.Ctx, msg.Path, m.store)
		m.Ctx.ProjectDir = msg.Path
		if p, err := project.LoadProject(msg.Path); err == nil {
			m.Ctx.Project = &p
		}
		m.state = ViewStateProject
		m.recalculateLayout()
		return m, m.projectView.Init()

	// Home: user wants to create/open project.
	case home.ActionMsg:
		switch msg.Action.Type {
		case home.ActionCreateProject:
			m.projectView = projectview.NewModel(m.Ctx, "", m.store)
			m.Ctx.ProjectDir = ""
			m.Ctx.Project = nil
			m.state = ViewStateProject
			m.recalculateLayout()
			return m, m.projectView.Init()
		case home.ActionOpenProject:
			m.projectView = projectview.NewModel(m.Ctx, msg.Action.ProjectPath, m.store)
			m.Ctx.ProjectDir = msg.Action.ProjectPath
			if p, err := project.LoadProject(msg.Action.ProjectPath); err == nil {
				m.Ctx.Project = &p
			}
			m.state = ViewStateProject
			m.recalculateLayout()
			return m, m.projectView.Init()
		}

	// Home: user selected a pipeline or tool entry.
	case home.CatalogSelectMsg:
		content := home.RenderYAMLPopupContent(msg.Entry, m.Ctx)
		m.popupView.Show(msg.Entry.Kind+": "+msg.Entry.Name, content, "esc  close")
		return m, nil

	// Project view: back to home.
	case projectview.MsgBack:
		m.state = ViewStateHome
		m.recalculateLayout()
		return m, nil

	// Settings saved — show toast (settings is a popup, no state change needed).
	case settings.MsgSaveConfig:
		m.popupView.Hide()
		cmds = append(cmds, m.toasterView.Show("Settings saved", toaster.TypeSuccess, 3_000_000_000))
		return m, tea.Batch(cmds...)

	case settings.MsgCancel:
		m.popupView.Hide()
		return m, nil
	}

	// Route input to active view.
	switch m.state {
	case ViewStateHome:
		m.homeView, cmd = m.homeView.Update(msg)
		cmds = append(cmds, cmd)
		m.sidebarView, cmd = m.sidebarView.Update(msg)
		cmds = append(cmds, cmd)
	case ViewStateProject:
		m.projectView, cmd = m.projectView.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Footer always ticks.
	m.footerView, cmd = m.footerView.Update(msg)
	cmds = append(cmds, cmd)

	// Always recalculate after every update so sizing stays consistent.
	m.recalculateLayout()

	return m, tea.Batch(cmds...)
}

func (m Model) isInputFocused() bool {
	switch m.state {
	case ViewStateHome:
		return m.homeView.Focused()
	case ViewStateProject:
		return m.projectView.Focused()
	}
	return false
}

func (m *Model) recalculateLayout() {
	footerHeight := 0
	if m.showFooter {
		footerHeight = m.Ctx.Window.Height / 4
		m.footerView.SetSize(m.Ctx.Window.Width, footerHeight)
	} else {
		m.footerView.SetSize(m.Ctx.Window.Width, 0)
	}

	contentHeight := m.Ctx.Window.Height - footerHeight
	contentWidth := m.Ctx.Window.Width - sidebar.Width

	m.sidebarView.SetHeight(contentHeight)

	sizeMsg := tea.WindowSizeMsg{Width: contentWidth, Height: contentHeight}
	m.homeView, _ = m.homeView.Update(sizeMsg)
	m.projectView, _ = m.projectView.Update(sizeMsg)
}

func (m Model) View() tea.View {
	th := m.Ctx.Theme
	footerHeight := 0
	var footerStr string
	if m.showFooter {
		footerHeight = m.Ctx.Window.Height / 4
		footerStr = m.footerView.View()
	}

	contentHeight := m.Ctx.Window.Height - footerHeight
	contentWidth := m.Ctx.Window.Width - sidebar.Width

	// Sidebar (always visible)
	sidebarStr := m.sidebarView.View()

	// Main content
	var mainStr string
	switch m.state {
	case ViewStateHome:
		mainStr = m.homeView.View()
	case ViewStateProject:
		mainStr = m.projectView.View()
	}

	mainStyle := lipgloss.NewStyle().
		Width(contentWidth).
		Height(contentHeight).
		Foreground(th.PrimaryText)
	mainStr = mainStyle.Render(mainStr)

	// Compose: sidebar | main, then footer below
	row := lipgloss.JoinHorizontal(lipgloss.Top, sidebarStr, mainStr)
	finalView := lipgloss.JoinVertical(lipgloss.Left, row, footerStr)

	// Overlays
	if m.popupView.IsVisible() {
		finalView = m.popupView.RenderOver(finalView)
	}
	finalView = m.toasterView.RenderOver(finalView)

	v := tea.NewView(finalView)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) renderHelp(state ViewState) string {
	th := m.Ctx.Theme
	keyStyle := lipgloss.NewStyle().Foreground(th.SecondaryText).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(th.PrimaryText)

	lines := []struct{ key, desc string }{
		{"?", "this help"},
		{"f", "toggle footer"},
		{"s", "settings"},
		{"q", "quit (from home)"},
		{"ctrl+c", "force quit"},
	}
	if state == ViewStateProject {
		lines = append(lines,
			struct{ key, desc string }{"esc", "return to home"},
			struct{ key, desc string }{"ctrl+r", "reload project"},
		)
	} else {
		lines = append(lines,
			struct{ key, desc string }{"n", "new project"},
			struct{ key, desc string }{"tab", "switch panel"},
			struct{ key, desc string }{"↑/↓", "navigate list"},
			struct{ key, desc string }{"enter", "select / open"},
		)
	}

	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(keyStyle.Render(padRight(l.key, 14)))
		sb.WriteString(descStyle.Render(l.desc))
		sb.WriteRune('\n')
	}
	return sb.String()
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
