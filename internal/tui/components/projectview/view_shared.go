package projectview

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/bitravens/paravizor/v1/internal/engine"
	tuictx "github.com/bitravens/paravizor/v1/internal/tui/context"
)

func (m Model) renderStatus(w int) string {
	var parts []string

	if m.projCfg != nil {
		parts = append(parts, "Project: "+m.projCfg.Name)
		if len(m.projCfg.Scope.Include) > 0 {
			parts = append(parts, "Scope: "+strings.Join(m.projCfg.Scope.Include, ", "))
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

	left := strings.Join(parts, "  -  ")

	helpStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	var helpParts []string
	if !m.running {
		helpParts = append(helpParts, "ctrl+r: Run")
	} else {
		helpParts = append(helpParts, "esc: Stop")
	}
	helpParts = append(helpParts, "esc: Back")
	right := helpStyle.Render(strings.Join(helpParts, "  "))

	leftStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.SecondaryText)
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return leftStyle.Render(left) + strings.Repeat(" ", gap) + right
}

func nodeStatusIcon(s engine.NodeStatus, ctx *tuictx.ProgramContext) (icon string, c color.Color) {
	switch s {
	case engine.NodeStatusActive:
		return ">", ctx.Theme.WarningText
	case engine.NodeStatusCompleted:
		return "v", ctx.Theme.SuccessText
	case engine.NodeStatusError:
		return "x", ctx.Theme.ErrorText
	case engine.NodeStatusDraining:
		return "~", ctx.Theme.SecondaryText
	default:
		return "o", ctx.Theme.FaintText
	}
}
