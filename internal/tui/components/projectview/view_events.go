package projectview

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func (m Model) renderEvents(w, h int) string {
	var lines []string
	if len(m.logLines) > h {
		lines = m.logLines[len(m.logLines)-h:]
	} else {
		lines = m.logLines
	}

	logText := strings.Join(lines, "\n")
	if logText == "" {
		logText = lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
			Render("No recent events.")
	}

	titleBg := m.ctx.Theme.SelectedBackground
	if m.activePanel == 1 {
		titleBg = m.ctx.Theme.WarningText
	}

	return lipgloss.NewStyle().
		Width(w).
		Height(h+2). // +2 for border
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.ctx.Theme.PrimaryBorder).
		Padding(0, 1).
		Render(lipgloss.NewStyle().
			Foreground(m.ctx.Theme.PrimaryText).
			Background(titleBg).
			Padding(0, 1).
			Render("Recent Events") + "\n" + logText)
}
