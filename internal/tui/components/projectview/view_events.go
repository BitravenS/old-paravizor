package projectview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m Model) renderEvents(w, h int) string {
	var lines []string
	if len(m.logLines) > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.ctx.Theme.SecondaryText).Bold(true).Render("Recent Events"))
		lines = append(lines, lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(strings.Repeat("-", max(1, w-4))))
		lines = append(lines, m.logLines...)
	}

	if len(lines) > h {
		lines = truncateEventLines(lines, h)
	}

	content := strings.Join(lines, "\n")
	if content == "" {
		content = lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
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
			Render("Events") + "\n" + content)
}

func truncateEventLines(lines []string, maxLines int) []string {
	if maxLines <= 0 {
		return nil
	}
	if len(lines) <= maxLines {
		return lines
	}
	truncated := append([]string{}, lines[:maxLines]...)
	truncated[maxLines-1] = fmt.Sprintf("... %d more lines", len(lines)-maxLines+1)
	return truncated
}
