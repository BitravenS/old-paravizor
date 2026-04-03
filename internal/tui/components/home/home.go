package home

import (
	"fmt"
	"io"
	"path/filepath"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bitravens/paravizor/v1/internal/tui/constants"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
)

type ActionType int

const (
	ActionCreateProject ActionType = iota
	ActionOpenSettings
	ActionOpenProject
)

type Item struct {
	Title       string
	Description string
	Type        ActionType
	ProjectPath string
}

func (i Item) FilterValue() string { return i.Title }

type itemDelegate struct {
	ctx *context.ProgramContext
}

func (d itemDelegate) Height() int                               { return 2 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}

	titleStyle := lipgloss.NewStyle().Foreground(d.ctx.Theme.PrimaryText)
	descStyle := lipgloss.NewStyle().Foreground(d.ctx.Theme.FaintText)

	if index == m.Index() {
		titleStyle = lipgloss.NewStyle().Foreground(context.LogoColor).Bold(true)
		descStyle = lipgloss.NewStyle().Foreground(d.ctx.Theme.SecondaryText)
		fmt.Fprintf(w, "%s\n%s", titleStyle.Render("▶ "+i.Title), descStyle.Render("  "+i.Description))
	} else {
		fmt.Fprintf(w, "%s\n%s", titleStyle.Render("  "+i.Title), descStyle.Render("  "+i.Description))
	}
}

type Model struct {
	ctx  *context.ProgramContext
	list list.Model
}

func NewModel(ctx *context.ProgramContext) Model {
	items := []list.Item{
		Item{Title: "Create New Project", Description: "Initialize a new recon project", Type: ActionCreateProject},
		Item{Title: "Settings", Description: "Edit global Paravizor configuration", Type: ActionOpenSettings},
	}

	for _, p := range ctx.Config.RecentProjects {
		items = append(items, Item{
			Title:       "Open " + filepath.Base(p),
			Description: p,
			Type:        ActionOpenProject,
			ProjectPath: p,
		})
	}

	delegate := itemDelegate{ctx: ctx}
	l := list.New(items, delegate, ctx.Window.Width, ctx.Window.Height-10)
	l.Title = "Home"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().Foreground(ctx.Theme.PrimaryText).Background(ctx.Theme.SelectedBackground).Padding(0, 1)

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "settings")),
		}
	}

	return Model{
		ctx:  ctx,
		list: l,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

type ActionMsg struct {
	Action Item
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-10) // leave room for logo

	case tea.KeyMsg:
		switch msg.String() {
		case "s":
			return m, func() tea.Msg {
				return ActionMsg{Action: Item{Type: ActionOpenSettings}}
			}
		case "enter":
			if i, ok := m.list.SelectedItem().(Item); ok {
				return m, func() tea.Msg {
					return ActionMsg{Action: i}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	logoStyle := lipgloss.NewStyle().
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(context.LogoColor).
		MarginBottom(2)

	logo := logoStyle.Render(constants.Logo)

	listStyle := lipgloss.NewStyle().MarginLeft(2)

	return lipgloss.JoinVertical(lipgloss.Left, logo, listStyle.Render(m.list.View()))
}
