package projectview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/bitravens/paravizor/v1/internal/engine"
)

// viewProject replaces the old monolith view in view.go
func (m Model) viewProject() string {
	totalW := m.width
	if totalW < 10 {
		totalW = 80
	}

	innerH := m.height - 3
	if innerH < 10 {
		innerH = 10
	}

	statsH := 6
	midH := innerH - statsH
	if midH < 5 {
		midH = 5
	}

	pipelineW := totalW * 30 / 100
	eventsW := totalW * 45 / 100
	procsW := totalW - pipelineW - eventsW - 4 // spacing

	var main string
	if m.activeWindow == projectWindowAI {
		main = m.renderAIWindow(totalW, innerH)
	} else {
		stats := m.renderStats(totalW)
		pipeline := m.renderPipeline(pipelineW, midH)
		events := m.renderEvents(eventsW, midH)
		procs := m.renderProcesses(procsW, midH)

		mid := lipgloss.JoinHorizontal(lipgloss.Top, pipeline, "  ", events, "  ", procs)
		main = lipgloss.JoinVertical(lipgloss.Left, stats, mid)

		if m.showNodeLogs {
			logsModal := m.renderLogsModal(totalW, innerH)
			main = lipgloss.Place(totalW, innerH, lipgloss.Center, lipgloss.Center, logsModal, lipgloss.WithWhitespaceChars(" "))
		}
	}

	status := m.renderStatus(totalW)
	return lipgloss.JoinVertical(lipgloss.Left, main, status)
}

func (m Model) renderAIWindow(w, h int) string {
	bodyW := w - 6
	if bodyW < 20 {
		bodyW = 20
	}
	bodyH := m.aiBodyHeight()

	lines := wrapPlainText(m.aiDisplayText(), bodyW)
	maxScroll := max(0, len(lines)-bodyH)
	scroll := m.aiScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := min(len(lines), scroll+bodyH)
	visible := lines[scroll:end]
	for len(visible) < bodyH {
		visible = append(visible, "")
	}

	borderColor := m.ctx.Theme.PrimaryBorder
	if m.aiRunning || m.aiQuestionRunning || m.aiInput.Focused() {
		borderColor = m.ctx.Theme.WarningText
	}
	titleText := "AI Assistant"
	if m.aiRunning {
		titleText += " - generating"
	} else if m.aiQuestionRunning {
		titleText += " - answering"
	}
	title := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.PrimaryText).
		Background(m.ctx.Theme.WarningText).
		Padding(0, 1).
		Render(titleText)

	status := strings.TrimSpace(m.aiStatus)
	if status == "" {
		status = "Press r to analyze after the run finishes, or c to chat."
	}
	position := fmt.Sprintf("%d/%d", min(scroll+bodyH, len(lines)), len(lines))
	controls := "c chat  r analyze  ctrl+a running  up/down scroll"
	if m.aiInput.Focused() {
		controls = "enter send  esc blur  ctrl+a running"
	}
	hint := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
		Render(status + "  |  " + controls + "  " + position)
	body := lipgloss.NewStyle().
		Width(bodyW).
		Height(bodyH).
		Foreground(m.ctx.Theme.PrimaryText).
		Render(strings.Join(visible, "\n"))

	input := m.aiInput
	input.SetWidth(max(1, bodyW-len(input.Prompt)))
	inputLine := lipgloss.NewStyle().
		Width(bodyW).
		Foreground(m.ctx.Theme.PrimaryText).
		Render(input.View())

	return lipgloss.NewStyle().
		Width(w).
		Height(h).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Render(title + "\n" + body + "\n" + inputLine + "\n" + hint)
}

func (m Model) renderLogsModal(w, h int) string {
	boxW := w * 80 / 100
	boxH := h * 80 / 100
	if boxW < 40 {
		boxW = 40
	}
	if boxH < 10 {
		boxH = 10
	}

	bodyW := m.modalBodyWidth()
	bodyH := m.modalBodyHeight()
	lines := wrapPlainText(m.nodeLogsText, bodyW)
	maxScroll := max(0, len(lines)-bodyH)
	scroll := m.nodeLogsScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + bodyH
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[scroll:end]
	for len(visible) < bodyH {
		visible = append(visible, "")
	}

	content := lipgloss.NewStyle().
		Width(bodyW).
		Height(bodyH).
		Foreground(m.ctx.Theme.PrimaryText).
		Render(strings.Join(visible, "\n"))

	modalTitle := m.nodeLogsTitle
	if modalTitle == "" {
		modalTitle = "Node Logs"
	}
	position := fmt.Sprintf("%d/%d", min(scroll+bodyH, len(lines)), len(lines))
	title := lipgloss.NewStyle().Foreground(m.ctx.Theme.WarningText).Bold(true).Render(" " + modalTitle + " ")
	hint := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render("↑/↓ pgup/pgdn home/end scroll  esc close  " + position)

	return lipgloss.NewStyle().
		Width(boxW).
		Height(boxH).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(m.ctx.Theme.PrimaryBorder).
		Background(m.ctx.Theme.SelectedBackground).
		Padding(1, 2).
		Render(title + "\n" + content + "\n" + hint)
}

func (m Model) modalBoxSize() (int, int) {
	w := m.width
	h := m.height - 3
	if w < 10 {
		w = 80
	}
	if h < 10 {
		h = 10
	}
	boxW := w * 80 / 100
	boxH := h * 80 / 100
	if boxW < 40 {
		boxW = 40
	}
	if boxH < 10 {
		boxH = 10
	}
	return boxW, boxH
}

func (m Model) modalBodyWidth() int {
	boxW, _ := m.modalBoxSize()
	bodyW := boxW - 6
	if bodyW < 10 {
		bodyW = 10
	}
	return bodyW
}

func (m Model) modalBodyHeight() int {
	_, boxH := m.modalBoxSize()
	bodyH := boxH - 6
	if bodyH < 1 {
		bodyH = 1
	}
	return bodyH
}

func (m Model) maxNodeLogsScroll() int {
	return max(0, len(wrapPlainText(m.nodeLogsText, m.modalBodyWidth()))-m.modalBodyHeight())
}

func wrapPlainText(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	var out []string
	for _, raw := range strings.Split(text, "\n") {
		if raw == "" {
			out = append(out, "")
			continue
		}
		for len(raw) > width {
			cut := width
			if space := strings.LastIndex(raw[:width], " "); space > width/2 {
				cut = space
			}
			out = append(out, strings.TrimRight(raw[:cut], " "))
			raw = strings.TrimLeft(raw[cut:], " ")
		}
		out = append(out, raw)
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func (m Model) renderStats(w int) string {
	box := lipgloss.NewStyle().
		Width(w).
		Height(4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.ctx.Theme.PrimaryBorder).
		Padding(0, 2)

	primary := lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText)
	highlight := lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText).Bold(true)

	runNodes, doneNodes, errNodes := 0, 0, 0
	for _, n := range m.nodes {
		if n.Status == engine.NodeStatusActive || n.Status == engine.NodeStatusDraining {
			runNodes++
		} else if n.Status == engine.NodeStatusCompleted {
			doneNodes++
		} else if n.Status == engine.NodeStatusError {
			errNodes++
		}
	}

	col1 := fmt.Sprintf("%s: %s\n%s: %s",
		primary.Render("Domains"), highlight.Render(fmt.Sprintf("%d", m.domainsCount)),
		primary.Render("URLs"), highlight.Render(fmt.Sprintf("%d", m.urlsCount)),
	)

	col2 := fmt.Sprintf("%s: %s\n%s: %s",
		primary.Render("Live"), highlight.Render(fmt.Sprintf("%d", m.liveCount)),
		primary.Render("Findings"), highlight.Render(fmt.Sprintf("%d", m.findingsCount)),
	)

	col3 := fmt.Sprintf("%s: %s\n%s: %s",
		primary.Render("Nodes Running"), highlight.Render(fmt.Sprintf("%d", runNodes)),
		primary.Render("Nodes Done/Err"), highlight.Render(fmt.Sprintf("%d/%d", doneNodes, errNodes)),
	)

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(w/3).Render(col1),
		lipgloss.NewStyle().Width(w/3).Render(col2),
		lipgloss.NewStyle().Width(w/3).Render(col3),
	)

	title := lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Background(m.ctx.Theme.SelectedBackground).Padding(0, 1).Render("Dashboard Stats")
	return box.Render(title + "\n" + content)
}
