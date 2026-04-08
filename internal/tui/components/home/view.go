package home

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/bitravens/paravizor/v1/internal/theme"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
)

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.w <= 0 || m.h <= 0 {
		return ""
	}
	leftW := m.w * 36 / 100
	if leftW < 24 {
		leftW = 24
	}
	rightW := m.w - leftW
	if rightW < 1 {
		rightW = 1
	}

	left := m.renderLeft(leftW)
	right := m.renderRight(rightW)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// ── Left panel ────────────────────────────────────────────────────────────────

func (m Model) renderLeft(w int) string {
	th := m.ctx.Theme
	inner := w - 4 // border(2) + padding(2×1)
	if inner < 0 {
		inner = 0
	}

	var rows []string
	rows = append(rows,
		lipgloss.NewStyle().Foreground(th.PrimaryText).Bold(true).Width(inner).Render("Actions"),
		lipgloss.NewStyle().Foreground(th.FaintBorder).Render(strings.Repeat("─", inner)),
		"",
	)

	if m.leftState == panelCreate {
		rows = append(rows,
			lipgloss.NewStyle().Foreground(th.WarningText).Bold(true).Width(inner).Render("New Project"),
			lipgloss.NewStyle().Foreground(th.FaintText).Width(inner).Render("Enter the project directory path."),
			"",
			m.createInput.View(),
			"",
			lipgloss.NewStyle().Foreground(th.FaintText).Render("enter  confirm   esc  cancel"),
		)
	} else {
		// Single action: New Project
		actions := []struct{ title, desc string }{
			{"New Project", "Initialize a new recon project directory"},
		}
		for i, a := range actions {
			selected := m.focus == focusLeft && i == m.actionCursor
			if selected {
				rows = append(rows,
					lipgloss.NewStyle().Foreground(th.WarningText).Bold(true).Width(inner).Render("▶ "+a.title),
					lipgloss.NewStyle().Foreground(th.SecondaryText).Width(inner).Render("  "+a.desc),
				)
			} else {
				rows = append(rows,
					lipgloss.NewStyle().Foreground(th.PrimaryText).Width(inner).Render("  "+a.title),
					lipgloss.NewStyle().Foreground(th.FaintText).Width(inner).Render("  "+a.desc),
				)
			}
			rows = append(rows, "")
		}
		rows = append(rows, "")
		faint := lipgloss.NewStyle().Foreground(th.FaintText)
		rows = append(rows,
			faint.Render("  n    new project"),
			faint.Render("  s    settings"),
			faint.Render("  ?    help"),
			faint.Render(" tab   switch panel"),
		)
	}

	border := th.FaintBorder
	if m.focus == focusLeft {
		border = th.PrimaryBorder
	}
	return box(strings.Join(rows, "\n"), w, m.h, border, th.PrimaryText)
}

// ── Right panel ───────────────────────────────────────────────────────────────

func (m Model) renderRight(w int) string {
	minBottomH := 8
	topH := AnimHeight + 4
	if m.h-topH < minBottomH {
		topH = m.h - minBottomH
	}
	if topH < 6 {
		topH = 6
	}
	bottomH := m.h - topH
	if bottomH < 0 {
		bottomH = 0
	}

	top := m.renderTop(w, topH)
	bottom := m.renderCatalog(w, bottomH)
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

func (m Model) renderTop(w, h int) string {
	th := m.ctx.Theme
	innerW := w - 4 // border(2) + padding(2×1)
	if innerW < 1 {
		innerW = 1
	}

	animW := AnimWidth
	if innerW-animW < 10 {
		animW = innerW - 10
	}
	if animW < 0 {
		animW = 0
	}
	infoW := innerW - animW
	if infoW < 0 {
		infoW = 0
	}

	combined := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(infoW).Render(m.renderInfoContent(infoW)),
		m.cat.Render(th),
	)
	return box(combined, w, h, th.FaintBorder, th.PrimaryText)
}

func (m Model) renderInfoContent(w int) string {
	if w <= 0 {
		return ""
	}
	th := m.ctx.Theme
	accent := lipgloss.NewStyle().Foreground(th.WarningText).Bold(true)
	text := lipgloss.NewStyle().Foreground(th.PrimaryText)
	dim := lipgloss.NewStyle().Foreground(th.FaintText)
	sec := lipgloss.NewStyle().Foreground(th.SecondaryText)

	var rows []string
	rows = append(rows,
		accent.Width(w).Render("Paravizor"),
		dim.Width(w).Render("Automated Recon Orchestration"),
		"",
		text.Width(w).Render("Version  "+m.ctx.Version),
		dim.Width(w).Render("Pipeline-driven recon framework"),
		dim.Width(w).Render("for security researchers."),
		"",
		sec.Width(w).Render("Features"),
		lipgloss.NewStyle().Foreground(th.FaintBorder).Render(strings.Repeat("─", w)),
		dim.Width(w).Render("• Multi-tool pipeline execution"),
		dim.Width(w).Render("• Scope-aware routing"),
		dim.Width(w).Render("• SQLite result storage"),
		dim.Width(w).Render("• Live DNS liveness tracking"),
		dim.Width(w).Render("• Plugin-style tool YAML config"),
	)
	return strings.Join(rows, "\n")
}

func (m Model) renderCatalog(w, h int) string {
	th := m.ctx.Theme
	inner := w - 4
	if inner < 1 {
		inner = 1
	}

	visibleItems := h - 6
	if visibleItems < 1 {
		visibleItems = 1
	}

	var content string
	if len(m.pipelines) == 0 && len(m.tools) == 0 {
		content = lipgloss.NewStyle().Foreground(th.FaintText).Render(" Loading…")
	} else {
		var title string
		if m.catalogTab == 0 {
			title = "Pipelines"
			if m.focus == focusRight {
				title += " (t: Tools)"
			}
			content = strings.Join(m.renderSectionRows(title, m.pipelines, inner, m.pipelineCursor, m.pipelineScroll, visibleItems), "\n")
		} else {
			title = "Tools"
			if m.focus == focusRight {
				title += " (t: Pipelines)"
			}
			content = strings.Join(m.renderSectionRows(title, m.tools, inner, m.toolCursor, m.toolScroll, visibleItems), "\n")
		}
	}

	border := th.FaintBorder
	if m.focus == focusRight {
		border = th.PrimaryBorder
	}
	return box(content, w, h, border, th.PrimaryText)
}

// renderSectionRows returns a slice of lines each exactly w display-columns wide.
func (m Model) renderSectionRows(title string, entries []CatalogEntry, w, cursor, scroll, maxItems int) []string {
	if w <= 0 {
		return nil
	}
	th := m.ctx.Theme

	// norm forces every line to exactly w columns.
	norm := func(content string) string {
		return lipgloss.NewStyle().Width(w).Render(content)
	}

	rows := []string{
		norm(lipgloss.NewStyle().Foreground(th.SecondaryText).Bold(true).Render(title)),
		norm(lipgloss.NewStyle().Foreground(th.FaintBorder).Render(strings.Repeat("─", w))),
	}

	if len(entries) == 0 {
		rows = append(rows, norm(lipgloss.NewStyle().Foreground(th.FaintText).Italic(true).Render("  none")))
		return rows
	}

	end := scroll + maxItems
	if end > len(entries) {
		end = len(entries)
	}

	for i := scroll; i < end; i++ {
		e := entries[i]
		selected := m.focus == focusRight && i == cursor
		icon, iconColor := statusIcon(e.Status, th)

		prefix := "  "
		if selected {
			prefix = "▶ "
		}
		iconStyled := lipgloss.NewStyle().Foreground(iconColor).Render(icon)
		var nameStyled string
		if selected {
			nameStyled = lipgloss.NewStyle().Foreground(th.WarningText).Bold(true).Render(prefix + e.Name)
		} else {
			nameStyled = lipgloss.NewStyle().Foreground(th.PrimaryText).Render(prefix + e.Name)
		}

		// Scroll indicators
		var suffix string
		if i == scroll && scroll > 0 {
			suffix = lipgloss.NewStyle().Foreground(th.FaintText).Render(" ↑")
		} else if i == end-1 && end < len(entries) {
			suffix = lipgloss.NewStyle().Foreground(th.FaintText).Render(" ↓")
		}

		// Combine left aligned name and right aligned suffix
		nameWidth := lipgloss.Width(iconStyled + " " + nameStyled)
		suffixWidth := lipgloss.Width(suffix)
		padW := w - nameWidth - suffixWidth
		if padW < 0 {
			padW = 0
		}

		line := iconStyled + " " + nameStyled + strings.Repeat(" ", padW) + suffix
		rows = append(rows, norm(line))
	}

	// Pad out to maxItems if necessary
	for len(rows) < maxItems+2 {
		rows = append(rows, norm(""))
	}

	return rows
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// box wraps content in a rounded border with 1-char internal padding on all sides.
func box(content string, w, h int, borderColor, _ color.Color) string {
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

func statusIcon(s EntryStatus, th *theme.Theme) (string, color.Color) {
	switch s {
	case StatusOK:
		return "✓", th.SuccessText
	case StatusWarn:
		return "⚠", th.WarningText
	case StatusError:
		return "✗", th.ErrorText
	}
	return "·", th.FaintText
}

// RenderYAMLPopupContent builds popup body content for a catalog entry.
func RenderYAMLPopupContent(entry CatalogEntry, ctx *context.ProgramContext) string {
	th := ctx.Theme
	icon, iconColor := statusIcon(entry.Status, th)
	badgeStyle := lipgloss.NewStyle().Foreground(iconColor).Bold(true)

	var statusLine string
	switch {
	case entry.NotInstall:
		statusLine = lipgloss.NewStyle().Foreground(th.WarningText).Render("⚠ Binary not installed")
	case entry.StatusMsg != "":
		statusLine = badgeStyle.Render(icon + " " + entry.StatusMsg)
	default:
		statusLine = badgeStyle.Render(icon + " Valid")
	}

	kindLabel := lipgloss.NewStyle().Foreground(th.FaintText).Render(entry.Kind + " · " + entry.Name)
	yamlBlock := renderYAMLBlock(entry.RawYAML, ctx)
	return lipgloss.JoinVertical(lipgloss.Left, kindLabel, statusLine, "", yamlBlock)
}

func renderYAMLBlock(raw string, ctx *context.ProgramContext) string {
	th := ctx.Theme
	keyStyle := lipgloss.NewStyle().Foreground(th.WarningText)
	valStyle := lipgloss.NewStyle().Foreground(th.PrimaryText)
	commentStyle := lipgloss.NewStyle().Foreground(th.FaintText).Italic(true)
	sepStyle := lipgloss.NewStyle().Foreground(th.SecondaryText)

	var rendered []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "#"):
			rendered = append(rendered, commentStyle.Render(line))
		case trimmed == "---":
			rendered = append(rendered, sepStyle.Render(line))
		case strings.Contains(line, ": "):
			idx := strings.Index(line, ": ")
			rendered = append(rendered, keyStyle.Render(line[:idx+1])+valStyle.Render(line[idx+1:]))
		case strings.HasSuffix(trimmed, ":"):
			rendered = append(rendered, keyStyle.Render(line))
		default:
			rendered = append(rendered, valStyle.Render(line))
		}
	}
	return strings.Join(rendered, "\n")
}
