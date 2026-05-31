package popup

import (
	"fmt"
	"strings"

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
	scrollTop int
}

func NewModel(ctx *context.ProgramContext) Model { return Model{ctx: ctx} }

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
		case "up", "k":
			if m.scrollTop > 0 {
				m.scrollTop--
			}
		case "down", "j":
			lines := strings.Split(m.body, "\n")
			if m.scrollTop < len(lines)-1 {
				m.scrollTop++
			}
		}
	}
	return true, nil
}

// dim applies ANSI faint to a pre-rendered string.
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

	innerW := popW - 2
	bodyPad := 1
	bodyW := innerW - 2*bodyPad

	hintRows := 0
	if m.hint != "" {
		hintRows = 3
	}
	availBodyLines := popH - 2 - 2 - hintRows
	if availBodyLines < 1 {
		availBodyLines = 1
	}

	// Word-wrap body lines
	var bodyLines []string
	for _, l := range strings.Split(m.body, "\n") {
		if lipgloss.Width(l) <= bodyW {
			bodyLines = append(bodyLines, l)
		} else {
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
	for len(visible) < availBodyLines {
		visible = append(visible, "")
	}

	bStyle := lipgloss.NewStyle().Foreground(th.PrimaryBorder)
	tStyle := lipgloss.NewStyle().Foreground(th.WarningText).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(th.PrimaryText).Width(bodyW)
	hintStyle := lipgloss.NewStyle().Foreground(th.FaintText).Width(bodyW)
	padStr := strings.Repeat(" ", bodyPad)

	side := func(inner string) string {
		return bStyle.Render("│") + padStr + inner + padStr + bStyle.Render("│")
	}

	// Top border: ╭──┤ Title ├──────╮
	titleText := tStyle.Render(m.title)
	leftDash := 2
	rightDash := innerW - leftDash - 2 - lipgloss.Width(titleText) - 2
	if rightDash < 0 {
		rightDash = 0
	}
	// Show scroll percentage in top-right if overflowing
	var pctStr string
	if totalLines > availBodyLines {
		pct := 100 * (m.scrollTop + availBodyLines) / totalLines
		pctStr = fmt.Sprintf("%3d%%", pct)
		rightDash -= len(pctStr) + 1
		if rightDash < 0 {
			rightDash = 0
		}
	}
	topBorder := bStyle.Render("╭"+strings.Repeat("─", leftDash)+"┤") +
		titleText +
		bStyle.Render("├"+strings.Repeat("─", rightDash))
	if pctStr != "" {
		topBorder += lipgloss.NewStyle().Foreground(th.SecondaryText).Render(pctStr) + bStyle.Render("╮")
	} else {
		topBorder += bStyle.Render("╮")
	}

	var rows []string
	rows = append(rows, topBorder)
	rows = append(rows, side(bodyStyle.Render(""))) // top pad
	for _, l := range visible {
		rows = append(rows, side(bodyStyle.Render(l)))
	}
	rows = append(rows, side(bodyStyle.Render(""))) // bottom pad

	if m.hint != "" {
		rows = append(rows,
			bStyle.Render("├"+strings.Repeat("─", innerW)+"┤"),
			side(hintStyle.Render(m.hint)),
			side(hintStyle.Render("")),
		)
	}

	rows = append(rows, bStyle.Render("╰"+strings.Repeat("─", innerW)+"╯"))
	modal := strings.Join(rows, "\n")

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
	return context.Overlay(bg, modal, x, y)
}
