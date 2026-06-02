package projectview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m Model) renderEvents(w, h int) string {
	visibleRows := h
	scrollable := len(m.logLines) > h
	if scrollable {
		visibleRows = max(1, h-1)
	}
	start := m.eventScroll
	maxScroll := max(0, len(m.logLines)-visibleRows)
	if start < 0 {
		start = 0
	}
	if start > maxScroll {
		start = maxScroll
	}
	end := min(len(m.logLines), start+visibleRows)

	var lines []string
	if start < end {
		lines = m.logLines[start:end]
	}

	logText := strings.Join(lines, "\n")
	if logText == "" {
		logText = lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
			Render("No recent events.")
	} else if scrollable {
		position := fmt.Sprintf("%d-%d/%d", start+1, end, len(m.logLines))
		logText += "\n" + lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
			Render("↑/↓ pgup/pgdn home/end  "+position)
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
