package sidebar

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
)

// Width is the fixed column width of the sidebar (no border, plain text).
const Width = 26

// ProjectSelectedMsg is emitted when the user opens a recent project.
type ProjectSelectedMsg struct{ Path string }

type Model struct {
	ctx     *context.ProgramContext
	height  int
	cursor  int
	focused bool
}

func NewModel(ctx *context.ProgramContext) Model {
	return Model{ctx: ctx}
}

func (m Model) Init() tea.Cmd { return nil }

func (m *Model) SetHeight(h int) { m.height = h }

func (m *Model) SetFocused(focused bool) { m.focused = focused }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	projects := m.ctx.Config.RecentProjects
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(projects)-1 {
				m.cursor++
			}
		case "enter":
			if m.cursor < len(projects) {
				path := projects[m.cursor]
				return m, func() tea.Msg { return ProjectSelectedMsg{Path: path} }
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	th := m.ctx.Theme
	inner := Width - 2 // 1 char left pad + 1 char right pad

	padL := " " // 1-char left padding

	titleStr := lipgloss.NewStyle().Foreground(th.WarningText).Bold(true).Render("Paravizor") +
		lipgloss.NewStyle().Foreground(th.FaintText).Render(" ➼ "+m.ctx.Version)
	titleLine := lipgloss.NewStyle().Width(inner).MaxWidth(inner).Render(titleStr)

	divider := lipgloss.NewStyle().Foreground(th.FaintBorder).Render(strings.Repeat("─", inner))

	labelColor := th.SecondaryText
	if m.focused {
		labelColor = th.WarningText
	}
	sectionLabel := lipgloss.NewStyle().Foreground(labelColor).Bold(true).Width(inner).Render("Recent Projects")

	var rows []string
	rows = append(rows,
		padL+titleLine,
		padL+divider,
		"",
		padL+sectionLabel,
		"",
	)

	projects := m.ctx.Config.RecentProjects
	if len(projects) == 0 {
		rows = append(rows, padL+lipgloss.NewStyle().Foreground(th.FaintText).Italic(true).Render("No recent projects"))
	}

	for i, p := range projects {
		base := filepath.Base(p)
		dir := filepath.Dir(p)

		maxDir := inner - 2
		if len(dir) > maxDir && maxDir > 3 {
			dir = "…" + dir[len(dir)-maxDir+1:]
		}

		if i == m.cursor && m.focused {
			rows = append(rows,
				padL+lipgloss.NewStyle().Foreground(th.WarningText).Bold(true).Width(inner).Render("▶ "+base),
				padL+lipgloss.NewStyle().Foreground(th.SecondaryText).Width(inner).Render("  "+dir),
			)
		} else {
			rows = append(rows,
				padL+lipgloss.NewStyle().Foreground(th.PrimaryText).Width(inner).Render("  "+base),
				padL+lipgloss.NewStyle().Foreground(th.FaintText).Width(inner).Render("  "+dir),
			)
		}
		rows = append(rows, "")
	}

	allContent := strings.Join(rows, "\n")
	contentLines := strings.Split(allContent, "\n")

	h := m.height
	if h <= 0 {
		h = len(contentLines)
	}

	var out []string
	for i := 0; i < h; i++ {
		var line string
		if i < len(contentLines) {
			line = contentLines[i]
		}
		out = append(out, lipgloss.NewStyle().Width(Width).MaxWidth(Width).Render(line))
	}

	return strings.Join(out, "\n")
}
