package projectview

import (
	"fmt"

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

	stats := m.renderStats(totalW)
	pipeline := m.renderPipeline(pipelineW, midH)
	events := m.renderEvents(eventsW, midH)
	procs := m.renderProcesses(procsW, midH)

	mid := lipgloss.JoinHorizontal(lipgloss.Top, pipeline, "  ", events, "  ", procs)
	main := lipgloss.JoinVertical(lipgloss.Left, stats, mid)

	if m.showNodeLogs {
		logsModal := m.renderLogsModal(totalW, innerH)
		// Overlap main with logsModal
		main = lipgloss.Place(totalW, innerH, lipgloss.Center, lipgloss.Center, logsModal, lipgloss.WithWhitespaceChars(" "))
	}

	status := m.renderStatus(totalW)
	return lipgloss.JoinVertical(lipgloss.Left, main, status)
}

func (m Model) renderLogsModal(w, h int) string {
	boxW := w * 80 / 100
	boxH := h * 80 / 100

	content := lipgloss.NewStyle().
		Width(boxW - 4).
		Height(boxH - 4).
		Foreground(m.ctx.Theme.PrimaryText).
		Render(m.nodeLogsText)

	title := lipgloss.NewStyle().Foreground(m.ctx.Theme.WarningText).Bold(true).Render(" Node Logs (esc to close) ")

	return lipgloss.NewStyle().
		Width(boxW).
		Height(boxH).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(m.ctx.Theme.PrimaryBorder).
		Background(m.ctx.Theme.SelectedBackground).
		Padding(1, 2).
		Render(title + "\n\n" + content)
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

	pipelineName := "n/a"
	if m.ctx != nil && m.ctx.Pipeline != nil && m.ctx.Pipeline.Name != "" {
		pipelineName = m.ctx.Pipeline.Name
	}
	budget := "n/a"
	if m.totalBudget > 0 {
		budget = fmt.Sprintf("%.1f rps", m.totalBudget)
	}
	col4 := fmt.Sprintf("%s: %s\n%s: %s",
		primary.Render("Pipeline"), highlight.Render(pipelineName),
		primary.Render("Budget"), highlight.Render(budget),
	)

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(w/4).Render(col1),
		lipgloss.NewStyle().Width(w/4).Render(col2),
		lipgloss.NewStyle().Width(w/4).Render(col3),
		lipgloss.NewStyle().Width(w/4).Render(col4),
	)

	title := lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Background(m.ctx.Theme.SelectedBackground).Padding(0, 1).Render("Dashboard Stats")
	return box.Render(title + "\n" + content)
}
