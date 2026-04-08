package projectview

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/bitravens/paravizor/v1/internal/engine"
	tuictx "github.com/bitravens/paravizor/v1/internal/tui/context"
)

// ── Project view ──────────────────────────────────────────────────────────────

func (m Model) viewProject() string {
	// Calculate panel widths: 40% nodes, 60% log.
	totalW := m.width
	if totalW < 10 {
		totalW = 80
	}
	leftW := totalW * 40 / 100
	rightW := totalW - leftW - 3 // 3 = lipgloss join spacing
	if rightW < 10 {
		rightW = 10
	}

	// Inner heights: subtract border (2) + title (1) + padding (2) + status bar (2).
	innerH := m.height - 7
	if innerH < 4 {
		innerH = 4
	}

	left := m.renderNodes(leftW, innerH)
	right := m.renderLog(rightW, innerH)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	status := m.renderStatus(totalW)

	return lipgloss.JoinVertical(lipgloss.Left, panels, status)
}

func (m Model) renderNodes(w, h int) string {
	borderColor := m.ctx.Theme.PrimaryBorder
	if m.running {
		borderColor = m.ctx.Theme.SuccessText
	}

	title := "Pipeline Nodes"
	if m.projCfg != nil {
		title = m.projCfg.Name + " — nodes"
	}

	var b strings.Builder
	for _, n := range m.nodes {
		icon, color := nodeStatusIcon(n.Status, m.ctx)
		iconStyle := lipgloss.NewStyle().Foreground(color)
		nameStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText)
		dimStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)

		line := fmt.Sprintf("%s %-16s %-12s",
			iconStyle.Render(icon),
			nameStyle.Render(n.Name),
			dimStyle.Render(n.Tool),
		)
		if n.Status == engine.NodeStatusCompleted {
			line += dimStyle.Render(fmt.Sprintf(" (%d→%d)", n.ItemsIn, n.ItemsOut))
		}
		b.WriteString(line + "\n")
	}

	if len(m.nodes) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
			Render("No pipeline nodes loaded."))
	}

	inner := b.String()
	// Trim to fit height.
	lines := strings.Split(strings.TrimRight(inner, "\n"), "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	inner = strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Width(w).
		Height(h+2). // +2 for border
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(lipgloss.NewStyle().
			Foreground(m.ctx.Theme.PrimaryText).
			Background(m.ctx.Theme.SelectedBackground).
			Padding(0, 1).
			Render(title) + "\n" + inner)
}

func (m Model) renderLog(w, h int) string {
	var lines []string
	if len(m.logLines) > h {
		lines = m.logLines[len(m.logLines)-h:]
	} else {
		lines = m.logLines
	}

	logText := strings.Join(lines, "\n")
	if logText == "" {
		logText = lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
			Render("No output yet.")
	}

	return lipgloss.NewStyle().
		Width(w).
		Height(h+2). // +2 for border
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.ctx.Theme.PrimaryBorder).
		Padding(0, 1).
		Render(lipgloss.NewStyle().
			Foreground(m.ctx.Theme.PrimaryText).
			Background(m.ctx.Theme.SelectedBackground).
			Padding(0, 1).
			Render("Log") + "\n" + logText)
}

func (m Model) renderStatus(w int) string {
	var parts []string

	if m.projCfg != nil {
		parts = append(parts, fmt.Sprintf("Project: %s", m.projCfg.Name))
		if len(m.projCfg.Scope.Include) > 0 {
			parts = append(parts, fmt.Sprintf("Scope: %s", strings.Join(m.projCfg.Scope.Include, ", ")))
		}
	}

	statusPart := "Ready"
	if m.running {
		statusPart = lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText).Render("Running...")
	}
	if m.runErr != nil {
		statusPart = lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText).
			Render("Error: " + m.runErr.Error())
	}
	parts = append(parts, statusPart)

	left := strings.Join(parts, "  •  ")

	helpStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	var helpParts []string
	if !m.running {
		helpParts = append(helpParts, "ctrl+r: Run")
	} else {
		helpParts = append(helpParts, "esc: Stop")
	}
	helpParts = append(helpParts, "esc: Back")
	right := helpStyle.Render(strings.Join(helpParts, "  "))

	// Pad to fill width.
	leftStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.SecondaryText)
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return leftStyle.Render(left) + strings.Repeat(" ", gap) + right
}

// nodeStatusIcon returns the display icon and themed colour for a node status.
func nodeStatusIcon(s engine.NodeStatus, ctx *tuictx.ProgramContext) (icon string, c color.Color) {
	switch s {
	case engine.NodeStatusActive:
		return "▶", ctx.Theme.WarningText
	case engine.NodeStatusCompleted:
		return "✓", ctx.Theme.SuccessText
	case engine.NodeStatusError:
		return "✗", ctx.Theme.ErrorText
	case engine.NodeStatusDraining:
		return "~", ctx.Theme.SecondaryText
	default: // idle
		return "○", ctx.Theme.FaintText
	}
}
