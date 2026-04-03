package popup

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
)

type Model struct {
	ctx     *context.ProgramContext
	content string
	title   string
	show    bool
}

func NewModel(ctx *context.ProgramContext) Model {
	return Model{
		ctx:  ctx,
		show: false,
	}
}

// Show opens the popup.
func (m *Model) Show(title, content string) {
	m.title = title
	m.content = content
	m.show = true
}

// Hide closes the popup.
func (m *Model) Hide() {
	m.show = false
}

// IsVisible returns whether the popup is currently showing.
func (m Model) IsVisible() bool {
	return m.show
}

// Update handles closing the popup via escape. Returns true if the message was consumed.
func (m *Model) Update(msg tea.Msg) (bool, tea.Cmd) {
	if !m.show {
		return false, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" || msg.String() == "enter" {
			m.Hide()
			return true, nil
		}
	}
	// While the popup is active, it consumes all other messages to prevent background interaction
	return true, nil
}

// dim applies the ANSI faint attribute to an already-rendered string.
func dim(s string) string {
	s = lipgloss.NewStyle().Faint(true).Render(s)
	// lipgloss emit \x1b[0m or \x1b[m, which resets everything including faint.
	// We re-apply faint after each reset.
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m\x1b[2m")
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m\x1b[2m")
	return s
}

// RenderOver composites the popup over the given background.
func (m Model) RenderOver(background string) string {
	if !m.show {
		return background
	}

	// Dim the background
	bg := dim(background)

	// Create the modal box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.ctx.Theme.PrimaryBorder).
		Padding(1, 2)

	var content string
	if m.title != "" {
		title := lipgloss.NewStyle().
			Foreground(m.ctx.Theme.PrimaryText).
			Bold(true).
			MarginBottom(1).
			Render(m.title)
		content = lipgloss.JoinVertical(lipgloss.Left, title, m.content)
	} else {
		content = m.content
	}

	modal := boxStyle.Render(content)

	bgW, bgH := lipgloss.Width(bg), lipgloss.Height(bg)
	mW, mH := lipgloss.Width(modal), lipgloss.Height(modal)

	// Calculate center
	x := (bgW - mW) / 2
	y := (bgH - mH) / 2

	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	c := lipgloss.NewCompositor(
		lipgloss.NewLayer(bg).Z(0),
		lipgloss.NewLayer(modal).X(x).Y(y).Z(1),
	)

	return c.Render()
}
