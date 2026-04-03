package toaster

import (
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
)

type ToastType int

const (
	TypeInfo ToastType = iota
	TypeSuccess
	TypeError
	TypeWarning
)

type Toast struct {
	id      int
	Message string
	Type    ToastType
}

type MsgHideToast struct {
	id int
}

type Model struct {
	ctx    *context.ProgramContext
	toasts []Toast
	nextID int
}

func NewModel(ctx *context.ProgramContext) Model {
	return Model{
		ctx: ctx,
	}
}

// Show adds a new toast and returns a command that will hide it after the duration.
func (m *Model) Show(msg string, t ToastType, duration time.Duration) tea.Cmd {
	m.nextID++
	id := m.nextID

	m.toasts = append(m.toasts, Toast{
		id:      id,
		Message: msg,
		Type:    t,
	})

	return tea.Tick(duration, func(t time.Time) tea.Msg {
		return MsgHideToast{id: id}
	})
}

// Update processes toast hide messages.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case MsgHideToast:
		var remaining []Toast
		for _, t := range m.toasts {
			if t.id != msg.id {
				remaining = append(remaining, t)
			}
		}
		m.toasts = remaining
	}
	return nil
}

// RenderOver composites the active toasts over the background at the bottom.
func (m Model) RenderOver(background string) string {
	if len(m.toasts) == 0 {
		return background
	}

	bgW, bgH := lipgloss.Width(background), lipgloss.Height(background)

	// Build the toasts column
	var renderedToasts []string
	for _, t := range m.toasts {
		var borderColor color.Color
		switch t.Type {
		case TypeSuccess:
			borderColor = m.ctx.Theme.SuccessText
		case TypeError:
			borderColor = m.ctx.Theme.ErrorText
		case TypeWarning:
			borderColor = m.ctx.Theme.WarningText
		default:
			borderColor = m.ctx.Theme.PrimaryBorder
		}

		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Foreground(m.ctx.Theme.PrimaryText).
			Padding(0, 1)

		renderedToasts = append(renderedToasts, style.Render(t.Message))
	}

	toastBlock := lipgloss.JoinVertical(lipgloss.Center, renderedToasts...)
	tW, tH := lipgloss.Width(toastBlock), lipgloss.Height(toastBlock)

	// Position at bottom center, slightly above the absolute bottom
	x := (bgW - tW) / 2
	y := bgH - tH - 2

	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	c := lipgloss.NewCompositor(
		lipgloss.NewLayer(background).Z(0),
		lipgloss.NewLayer(toastBlock).X(x).Y(y).Z(1),
	)

	return c.Render()
}
