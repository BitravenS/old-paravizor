package projectview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/bitravens/paravizor/v1/internal/engine"
)

func (m Model) renderPipeline(w, h int) string {
	borderColor := m.ctx.Theme.PrimaryBorder
	if m.running {
		borderColor = m.ctx.Theme.SuccessText
	}

	title := "Pipeline"
	if m.projCfg != nil {
		title = m.projCfg.Name + " — Pipeline"
	}

	var b strings.Builder
	visibleRows := h
	scrollable := len(m.nodes) > h
	if scrollable {
		visibleRows = max(1, h-1)
	}
	start := m.pipelineScroll
	maxScroll := max(0, len(m.nodes)-visibleRows)
	if start < 0 {
		start = 0
	}
	if start > maxScroll {
		start = maxScroll
	}
	end := min(len(m.nodes), start+visibleRows)

	for i := start; i < end; i++ {
		n := m.nodes[i]
		icon, color := nodeStatusIcon(n.Status, m.ctx)
		iconStyle := lipgloss.NewStyle().Foreground(color)

		nameStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText)
		if m.activePanel == 0 && i == m.pipelineCursor {
			nameStyle = lipgloss.NewStyle().Foreground(m.ctx.Theme.WarningText).Bold(true)
		}

		dimStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)

		line := fmt.Sprintf("%s %-16s", iconStyle.Render(icon), nameStyle.Render(n.Name))
		if n.Status == engine.NodeStatusCompleted {
			line += dimStyle.Render(fmt.Sprintf(" (%d)", n.ItemsOut))
		}
		b.WriteString(line + "\n")
	}

	if len(m.nodes) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
			Render("No pipeline nodes loaded.") + "\n")
	} else if scrollable {
		position := fmt.Sprintf("%d-%d/%d", start+1, end, len(m.nodes))
		b.WriteString(lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
			Render("↑/↓ pgup/pgdn home/end  "+position) + "\n")
	}

	inner := b.String()
	lines := strings.Split(strings.TrimRight(inner, "\n"), "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	inner = strings.Join(lines, "\n")

	titleBg := m.ctx.Theme.SelectedBackground
	if m.activePanel == 0 {
		titleBg = m.ctx.Theme.WarningText
	}

	return lipgloss.NewStyle().
		Width(w).
		Height(h+2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(lipgloss.NewStyle().
			Foreground(m.ctx.Theme.PrimaryText).
			Background(titleBg).
			Padding(0, 1).
			Render(title) + "\n" + inner)
}
