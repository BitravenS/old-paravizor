package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/bitravens/paravizor/v1/internal/tui/components/footer"
	"github.com/bitravens/paravizor/v1/internal/tui/components/header"
	"github.com/bitravens/paravizor/v1/internal/tui/components/home"
	"github.com/bitravens/paravizor/v1/internal/tui/components/popup"
	"github.com/bitravens/paravizor/v1/internal/tui/components/projectview"
	"github.com/bitravens/paravizor/v1/internal/tui/components/settings"
	"github.com/bitravens/paravizor/v1/internal/tui/components/toaster"
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
	store        *store.Store
	state        ViewState
	homeView     home.Model
	settingsView settings.Model
	projectView  projectview.Model
	headerView   header.Model
	footerView   footer.Model
	popupView    popup.Model
	toasterView  toaster.Model
	showFooter   bool
}

func NewModel(location, version string, s *store.Store) Model {
	ctx, err := context.NewProgramContext(location)
	if err != nil {
		log.Fatal("Failed to initialize program context", "err", err)
	}
	ctx.Version = version

	state := ViewStateHome
	showFooter := false
	if location != "" {
		state = ViewStateProject
		showFooter = true // Show footer by default in projects
	}

	// If an existing project location was provided, open it directly.
	projInitDir := ""
	if location != "" {
		projInitDir = location
	}

	return Model{
		Ctx:          ctx,
		store:        s,
		state:        state,
		showFooter:   showFooter,
		homeView:     home.NewModel(ctx),
		settingsView: settings.NewModel(ctx),
		projectView:  projectview.NewModel(ctx, projInitDir),
		headerView:   header.NewModel(ctx),
		footerView:   footer.NewModel(ctx, s),
		popupView:    popup.NewModel(ctx),
		toasterView:  toaster.NewModel(ctx),
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
		m.projectView.Init(),
		m.footerView.Init(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Handle toaster global updates
	if tcmd := m.toasterView.Update(msg); tcmd != nil {
		cmds = append(cmds, tcmd)
	}

	// Intercept popup keys if visible
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
		case "ctrl+r":
			if m.state == ViewStateProject {
				if dir := m.projectView.ProjectDir(); dir != "" {
					m.Ctx.ProjectDir = dir
					if p, err := project.LoadProject(dir); err == nil {
						m.Ctx.Project = &p
					}
				}
			}
		case "?":
			if !m.popupView.IsVisible() {
				m.popupView.Show("Help Menu", "Keybindings:\n\n?\tShow this help\nf\tToggle Footer\nq\tQuit")
				return m, nil
			}
		case "q":
			// If we are at home, quit. Otherwise might want to go back.
			if m.state == ViewStateHome {
				return m, tea.Quit
			}
			// But for settings, we only go back on 'esc' or cancel to not break input
		case "f":
			m.showFooter = !m.showFooter
			m.recalculateLayout(msg)
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.Ctx.Window.Width = msg.Width
		m.Ctx.Window.Height = msg.Height
		m.recalculateLayout(msg)
	case home.ActionMsg:
		switch msg.Action.Type {
		case home.ActionOpenSettings:
			m.state = ViewStateSettings
			return m, nil
		case home.ActionCreateProject:
			m.projectView = projectview.NewModel(m.Ctx, "")
			m.Ctx.ProjectDir = ""
			m.Ctx.Project = nil
			m.state = ViewStateProject
			return m, m.projectView.Init()
		case home.ActionOpenProject:
			m.projectView = projectview.NewModel(m.Ctx, msg.Action.ProjectPath)
			m.Ctx.ProjectDir = msg.Action.ProjectPath
			if p, err := project.LoadProject(msg.Action.ProjectPath); err == nil {
				m.Ctx.Project = &p
			}
			m.state = ViewStateProject
			return m, m.projectView.Init()
		}
	case projectview.MsgBack:
		m.state = ViewStateHome
		return m, nil
	case settings.MsgCancel, settings.MsgSaveConfig:
		m.state = ViewStateHome

		if _, ok := msg.(settings.MsgSaveConfig); ok {
			cmds = append(cmds, m.toasterView.Show("Settings saved successfully", toaster.TypeSuccess, 3*1000*1000*1000))
		}

		return m, tea.Batch(cmds...)
	}

	switch m.state {
	case ViewStateHome:
		m.homeView, cmd = m.homeView.Update(msg)
		cmds = append(cmds, cmd)
	case ViewStateSettings:
		m.settingsView, cmd = m.settingsView.Update(msg)
		cmds = append(cmds, cmd)
	case ViewStateProject:
		m.projectView, cmd = m.projectView.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Keep footer state/metrics alive even when hidden.
	m.footerView, cmd = m.footerView.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) recalculateLayout(msg tea.Msg) {
	// calculate heights
	headerHeight := 1
	footerHeight := 0

	if m.showFooter {
		footerHeight = m.Ctx.Window.Height / 4
		m.footerView.SetSize(m.Ctx.Window.Width, footerHeight)
	}

	contentHeight := m.Ctx.Window.Height - headerHeight - footerHeight

	// pass size down
	sizeMsg := tea.WindowSizeMsg{Width: m.Ctx.Window.Width, Height: contentHeight}
	m.homeView, _ = m.homeView.Update(sizeMsg)
	m.settingsView, _ = m.settingsView.Update(sizeMsg)
	m.projectView, _ = m.projectView.Update(sizeMsg)
}

func (m Model) View() tea.View {
	var content string

	switch m.state {
	case ViewStateHome:
		content = m.homeView.View()
	case ViewStateSettings:
		content = m.settingsView.View()
	case ViewStateProject:
		content = m.projectView.View()
	}

	// Update active tab based on state
	if m.state == ViewStateProject {
		m.headerView.ActiveTab = "Project"
	} else {
		m.headerView.ActiveTab = "Home"
	}

	header := m.headerView.View()

	// Content area
	contentHeight := m.Ctx.Window.Height - 1
	var footer string
	if m.showFooter {
		footerHeight := m.Ctx.Window.Height / 4
		contentHeight -= footerHeight
		footer = m.footerView.View()
	}

	contentStyle := lipgloss.NewStyle().
		Width(m.Ctx.Window.Width).
		Height(contentHeight).
		Foreground(m.Ctx.Theme.PrimaryText)

	// Center home/settings; project view manages its own layout.
	if m.state != ViewStateProject {
		contentStyle = contentStyle.Align(lipgloss.Center, lipgloss.Center)
	}

	renderedContent := contentStyle.Render(content)

	finalView := lipgloss.JoinVertical(lipgloss.Left, header, renderedContent, footer)

	// Apply popup overlay
	if m.popupView.IsVisible() {
		finalView = m.popupView.RenderOver(finalView)
	}

	// Apply toaster overlay
	finalView = m.toasterView.RenderOver(finalView)

	v := tea.NewView(finalView)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
