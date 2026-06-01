package projectview

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m Model) renderProcesses(w, h int) string {
	titleBg := m.ctx.Theme.SelectedBackground
	if m.activePanel == 2 {
		titleBg = m.ctx.Theme.WarningText
	}

	title := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.PrimaryText).
		Background(titleBg).
		Padding(0, 1).
		Render("Active Processes")

	var b strings.Builder

	// Convert map to slice and sort for stable display
	var procs []processRow
	for _, p := range m.activeProcesses {
		procs = append(procs, p)
	}
	sort.Slice(procs, func(i, j int) bool {
		return procs[i].ID < procs[j].ID
	})

	for i, p := range procs {
		alloc := 0.0
		if val, ok := m.rateAllocations[p.NodeID]; ok {
			alloc = val
		}

		rateStr := ""
		if alloc > 0 {
			rateStr = fmt.Sprintf("%.0fr/s", alloc)
		}

		pidStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.SecondaryText)
		nameStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText)
		if m.activePanel == 2 && i == m.processCursor {
			nameStyle = lipgloss.NewStyle().Foreground(m.ctx.Theme.WarningText).Bold(true)
		}

		rateStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.WarningText)

		b.WriteString(fmt.Sprintf("%-6s %-12s %8s\n",
			pidStyle.Render(fmt.Sprintf("%d", p.PID)),
			nameStyle.Render(p.Tool),
			rateStyle.Render(rateStr),
		))
	}

	if len(procs) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
			Render("No active processes."))
	}

	inner := b.String()
	lines := strings.Split(strings.TrimRight(inner, "\n"), "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	inner = strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Width(w).
		Height(h+2). // +2 for border
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.ctx.Theme.PrimaryBorder).
		Padding(0, 1).
		Render(title + "\n" + inner)
}
