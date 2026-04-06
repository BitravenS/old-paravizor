package popup

import (
	"strings"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
)

const (
	maxPopupW = 80
	maxPopupH = 60
)

type Model struct {
	ctx       *context.ProgramContext
	title     string
	body      string
	hint      string
	show      bool
	scrollTop int // first visible body line
}

func NewModel(ctx *context.ProgramContext) Model { return Model{ctx: ctx} }

// Show opens the popup. hint is rendered in the footer row (e.g. "esc close").
func (m *Model) Show(title, body, hint string) {
	m.title = title
	m.body = body
	m.hint = hint
	m.scrollTop = 0
	m.show = true
}

func (m *Model) Hide()          { m.show = false }
func (m Model) IsVisible() bool { return m.show }

// Update handles keyboard while the popup is open. Returns (consumed, cmd).
func (m *Model) Update(msg tea.Msg) (bool, tea.Cmd) {
	if !m.show {
		return false, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "q":
			m.Hide()
			return true, nil
		case "up", "k":
			if m.scrollTop > 0 {
				m.scrollTop--
			}
			return true, nil
		case "down", "j":
			lines := strings.Split(m.body, "\n")
			if m.scrollTop < len(lines)-1 {
				m.scrollTop++
			}
			return true, nil
		}
	}
	return true, nil
}

// dim applies ANSI faint to a pre-rendered string, re-applying after resets.
func dim(s string) string {
	s = lipgloss.NewStyle().Faint(true).Render(s)
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m\x1b[2m")
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m\x1b[2m")
	return s
}

// RenderOver composites the popup over a pre-rendered background string.
func (m Model) RenderOver(background string) string {
	if !m.show {
		return background
	}

	th := m.ctx.Theme
	bg := dim(background)
	bgW := lipgloss.Width(bg)
	bgH := lipgloss.Height(bg)

	// ── Compute popup dimensions ─────────────────────────────────────────────
	popW := bgW * 80 / 100
	if popW > maxPopupW {
		popW = maxPopupW
	}
	if popW < 30 {
		popW = 30
	}
	popH := bgH * 80 / 100
	if popH > maxPopupH {
		popH = maxPopupH
	}
	if popH < 8 {
		popH = 8
	}

	innerW := popW - 2 // subtract left+right border chars
	bodyPad := 1       // 1-char inner horizontal padding each side
	bodyW := innerW - 2*bodyPad

	// Reserve rows: 1 top border + 1 body padding top + N body lines + 1 body padding bottom
	// + (1 hint sep + 1 hint + 1 hint pad) + 1 bottom border
	hintRows := 0
	if m.hint != "" {
		hintRows = 3 // sep + hint line + pad
	}
	availBodyLines := popH - 2 - 2 - hintRows // top border + top pad + bottom border + bottom pad
	if availBodyLines < 1 {
		availBodyLines = 1
	}

	// ── Prepare body lines ───────────────────────────────────────────────────
	rawLines := strings.Split(m.body, "\n")
	// Word-wrap lines that exceed bodyW.
	var bodyLines []string
	for _, l := range rawLines {
		if lipgloss.Width(l) <= bodyW {
			bodyLines = append(bodyLines, l)
		} else {
			// Simple hard-wrap by rune width.
			runes := []rune(l)
			for len(runes) > 0 {
				end := bodyW
				if end > len(runes) {
					end = len(runes)
				}
				bodyLines = append(bodyLines, string(runes[:end]))
				runes = runes[end:]
			}
		}
	}

	totalLines := len(bodyLines)
	if m.scrollTop > totalLines-availBodyLines {
		m.scrollTop = totalLines - availBodyLines
	}
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}

	end := m.scrollTop + availBodyLines
	if end > totalLines {
		end = totalLines
	}
	visible := bodyLines[m.scrollTop:end]
	// Pad to availBodyLines
	for len(visible) < availBodyLines {
		visible = append(visible, "")
	}

	// ── Render helpers ───────────────────────────────────────────────────────
	bStyle := lipgloss.NewStyle().Foreground(th.PrimaryBorder)
	tStyle := lipgloss.NewStyle().Foreground(th.WarningText).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(th.PrimaryText).Width(bodyW)
	hintStyle := lipgloss.NewStyle().Foreground(th.FaintText).Width(bodyW)
	padStr := strings.Repeat(" ", bodyPad)

	side := func(inner string) string {
		return bStyle.Render("│") + padStr + inner + padStr + bStyle.Render("│")
	}

	// ── Top border with title ────────────────────────────────────────────────
	titleText := tStyle.Render(m.title)
	titleVisW := lipgloss.Width(titleText)
	// ╭──┤ Title ├──────╮
	leftDash := 2
	rightDash := innerW - leftDash - 2 - titleVisW - 2 // ─┤ + ├─
	if rightDash < 0 {
		rightDash = 0
	}
	topBorder := bStyle.Render("╭"+strings.Repeat("─", leftDash)+"┤") +
		titleText +
		bStyle.Render("├"+strings.Repeat("─", rightDash)+"╮")

	// ── Body rows ────────────────────────────────────────────────────────────
	emptyRow := side(bodyStyle.Render(""))
	var bodyRows []string
	bodyRows = append(bodyRows, emptyRow) // top padding
	for _, l := range visible {
		bodyRows = append(bodyRows, side(bodyStyle.Render(l)))
	}
	bodyRows = append(bodyRows, emptyRow) // bottom padding

	// ── Hint section ─────────────────────────────────────────────────────────
	var hintSection []string
	if m.hint != "" {
		sep := bStyle.Render("├" + strings.Repeat("─", innerW) + "┤")
		hintSection = append(hintSection,
			sep,
			side(hintStyle.Render(m.hint)),
			side(hintStyle.Render("")),
		)
	}

	// Scroll indicator in top-right corner if content overflows.
	scrollIndicator := ""
	if totalLines > availBodyLines {
		pct := 100 * (m.scrollTop + availBodyLines) / totalLines
		scrollIndicator = lipgloss.NewStyle().Foreground(th.FaintText).Render(
			strings.Repeat("─", rightDash-5) +
				lipgloss.NewStyle().Foreground(th.SecondaryText).Render(
					fmt.Sprintf("%3d%%", pct),
				),
		)
		_ = scrollIndicator // TODO: embed in topBorder
	}

	// ── Bottom border ─────────────────────────────────────────────────────────
	bottomBorder := bStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")

	// ── Assemble ─────────────────────────────────────────────────────────────
	parts := []string{topBorder}
	parts = append(parts, bodyRows...)
	parts = append(parts, hintSection...)
	parts = append(parts, bottomBorder)
	modal := strings.Join(parts, "\n")

	mW := lipgloss.Width(modal)
	mH := lipgloss.Height(modal)
	x := (bgW - mW) / 2
	y := (bgH - mH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	c := lipgloss.NewCompositor(
		lipgloss.NewLayer(bg).Z(0),
		lipgloss.NewLayer(modal).X(x).Y(y).Z(1),
	)
	return c.Render()
}
