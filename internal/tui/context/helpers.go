package context

import (
	"image/color"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// StyledInput creates a new textinput.Model pre-styled with the current theme.
func (ctx *ProgramContext) StyledInput() textinput.Model {
	t := textinput.New()
	st := textinput.DefaultDarkStyles()
	st.Focused.Prompt = lipgloss.NewStyle().Foreground(ctx.Theme.WarningText)
	st.Focused.Text = lipgloss.NewStyle().Foreground(ctx.Theme.PrimaryText)
	st.Blurred.Prompt = lipgloss.NewStyle().Foreground(ctx.Theme.SecondaryText)
	st.Blurred.Text = lipgloss.NewStyle().Foreground(ctx.Theme.PrimaryText)
	st.Cursor.Color = ctx.Theme.WarningText
	t.SetStyles(st)
	return t
}

// Box wraps content in a rounded border with 1-char padding on all sides.
// w is the outer width (including border), h is the outer height.
func Box(content string, w, h int, borderColor color.Color) string {
	innerH := h - 4
	if innerH < 0 {
		innerH = 0
	}
	truncated := lipgloss.NewStyle().MaxHeight(innerH).Render(content)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 1).
		Width(w - 2).
		Height(h).
		Render(truncated)
}

// Overlay composites content over background at position (x, y).
func Overlay(background, content string, x, y int) string {
	c := lipgloss.NewCompositor(
		lipgloss.NewLayer(background).Z(0),
		lipgloss.NewLayer(content).X(x).Y(y).Z(1),
	)
	return c.Render()
}
