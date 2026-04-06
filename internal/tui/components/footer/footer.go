package footer

import (
	stdctx "context"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

// PerformanceTickMsg carries fresh metrics delivered from FetchPerformanceMetrics.
type PerformanceTickMsg struct {
	CPUPercent float64
	MemPercent float64
	MemMB      float64
	DBSizeMB   float64
	HasDB      bool
}

// metricsClockMsg is an internal tick used to trigger a metrics refresh.
type metricsClockMsg struct{}

// TickMetrics arms a 200ms repeating clock that drives metric refreshes.
func TickMetrics() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg {
		return metricsClockMsg{}
	})
}

// FetchPerformanceMetrics collects real CPU, memory, and DB size data off the
// main goroutine and delivers the result as a PerformanceTickMsg.
func FetchPerformanceMetrics(ctx *context.ProgramContext, s *store.Store) tea.Cmd {
	return func() tea.Msg {
		msg := PerformanceTickMsg{}

		if ctx.ProjectDir != "" {
			msg.HasDB = true
			dbPath := filepath.Join(ctx.ProjectDir, "paravizor.db")
			if info, err := os.Stat(dbPath); err == nil {
				msg.DBSizeMB = float64(info.Size()) / 1024 / 1024
			}
		}

		var childPIDs []int64
		if s != nil {
			childPIDs, _ = s.GetRunningProcessPIDs(stdctx.Background())
		}

		m := utils.GetPerformanceMetrics(childPIDs)
		msg.CPUPercent = m.CPUPercent
		msg.MemPercent = m.MemPercent
		msg.MemMB = m.MemMB
		return msg
	}
}

const dbCapMB = 1024.0 // 1 GB shown as 100% in the DB bar

type Model struct {
	ctx      *context.ProgramContext
	store    *store.Store
	cpuProg  progress.Model
	memProg  progress.Model
	dbProg   progress.Model
	height   int
	width    int
	cpuPct   float64
	memPct   float64
	memMB    float64
	dbSizeMB float64
	hasDB    bool
	hasPerf  bool
}

func NewModel(ctx *context.ProgramContext, s *store.Store) Model {
	return Model{
		ctx:   ctx,
		store: s,
		cpuProg: progress.New(progress.WithColors(
			ctx.Theme.WarningText,
			ctx.Theme.SecondaryText,
		),
			progress.WithoutPercentage()),
		memProg: progress.New(progress.WithColors(
			ctx.Theme.SuccessText,
			ctx.Theme.SecondaryText,
		),
			progress.WithoutPercentage()),
		dbProg: progress.New(progress.WithColors(
			ctx.Theme.PrimaryText,
			ctx.Theme.SecondaryText,
		),
			progress.WithoutPercentage()),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(TickMetrics(), FetchPerformanceMetrics(m.ctx, m.store))
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case metricsClockMsg:
		cmds = append(cmds, TickMetrics(), FetchPerformanceMetrics(m.ctx, m.store))

	case PerformanceTickMsg:
		if !m.hasPerf {
			m.cpuPct = msg.CPUPercent
			m.memPct = msg.MemPercent
			m.memMB = msg.MemMB
			m.hasPerf = true
		} else {
			const alpha = 0.10
			m.cpuPct = m.cpuPct + alpha*(msg.CPUPercent-m.cpuPct)
			m.memPct = m.memPct + alpha*(msg.MemPercent-m.memPct)
			m.memMB = m.memMB + alpha*(msg.MemMB-m.memMB)
		}
		m.dbSizeMB = msg.DBSizeMB
		m.hasDB = msg.HasDB
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	colWidth1 := width / 3
	colWidth2 := width / 3
	perfColWidth := width - colWidth1 - colWidth2

	// Keep progress bars on the same visual row as label/value by sizing
	// against the performance column's inner content width.
	const (
		labelWidth = 3
		valueWidth = 8
	)
	barWidth := perfColWidth - 2 /* border */ - 2 /* content pad */ - labelWidth - 1 - valueWidth - 1 - 1 /* right inner pad */
	if barWidth < 4 {
		barWidth = 4
	}
	m.cpuProg.SetWidth(barWidth)
	m.memProg.SetWidth(barWidth)
	m.dbProg.SetWidth(barWidth)
}

func borderWithTitle(content, title string, width, height int, borderColor color.Color, titleColor color.Color) string {
	bStyle := lipgloss.NewStyle().Foreground(borderColor)
	tStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(true)

	tLen := lipgloss.Width(title)
	dashCount := width - 4 - tLen - 2 // ╭──┤(4) + title + ├(1) + ╮(1)
	if dashCount < 0 {
		dashCount = 0
	}
	topBar := bStyle.Render("╭──┤") + tStyle.Render(title) + bStyle.Render("├"+strings.Repeat("─", dashCount)+"╮")

	bodyStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), false, true, true, true).
		BorderForeground(borderColor).
		Width(width).
		Height(height - 1)

	return lipgloss.JoinVertical(lipgloss.Left, topBar, bodyStyle.Render(content))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func (m Model) View() string {
	if m.height <= 0 {
		return ""
	}

	colWidth1 := m.width / 3
	colWidth2 := m.width / 3
	colWidth3 := m.width - colWidth1 - colWidth2
	pad := lipgloss.NewStyle().Padding(0, 1)

	projectName := "No project"
	projectPath := ""
	projectTargets := 0
	if m.ctx.Project != nil {
		projectName = m.ctx.Project.Name
		projectTargets = len(m.ctx.Project.Scope.Include)
	}
	if m.ctx.ProjectDir != "" {
		projectPath = m.ctx.ProjectDir
	}
	if projectPath == "" {
		projectPath = "-"
	}

	projectContent := fmt.Sprintf("\nName: %s\nTargets: %d\nPath: %s", projectName, projectTargets, projectPath)
	projectView := borderWithTitle(pad.Render(projectContent), "Project", colWidth1, m.height,
		m.ctx.Theme.PrimaryBorder, m.ctx.Theme.SecondaryText)

	pipelineName := "No pipeline loaded"
	stageCount := 0
	nodeCount := 0
	if m.ctx.Pipeline != nil {
		pipelineName = m.ctx.Pipeline.Name
		stageCount = len(m.ctx.Pipeline.Stages)
		nodeCount = len(m.ctx.Pipeline.Nodes)
	}
	pipelineContent := fmt.Sprintf("\nName: %s\nStages: %d\nNodes: %d", pipelineName, stageCount, nodeCount)
	pipelineView := borderWithTitle(pad.Render(pipelineContent), "Pipeline", colWidth2, m.height,
		m.ctx.Theme.PrimaryBorder, m.ctx.Theme.SecondaryText)

	// Performance: single-line rows (label + value + bar)
	labelStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.SecondaryText)
	valueStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText)
	labelCol := lipgloss.NewStyle().Width(3)
	valueCol := lipgloss.NewStyle().Width(8).Align(lipgloss.Right)

	cpuRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		labelCol.Render(labelStyle.Render("CPU")),
		" ",
		valueCol.Render(valueStyle.Render(fmt.Sprintf("%5.1f%%", m.cpuPct))),
		" ",
		m.cpuProg.ViewAs(clamp01(m.cpuPct/100)),
		" ",
	)
	memRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		labelCol.Render(labelStyle.Render("MEM")),
		" ",
		valueCol.Render(valueStyle.Render(fmt.Sprintf("%.1fMB", m.memMB))),
		" ",
		m.memProg.ViewAs(clamp01(m.memPct/100)),
		" ",
	)

	var dbRow string
	if m.hasDB {
		dbRow = lipgloss.JoinHorizontal(
			lipgloss.Left,
			labelCol.Render(labelStyle.Render("DB")),
			" ",
			valueCol.Render(valueStyle.Render(fmt.Sprintf("%.1fMB", m.dbSizeMB))),
			" ",
			m.dbProg.ViewAs(clamp01(m.dbSizeMB/dbCapMB)),
		)
	} else {
		greyProg := progress.New(
			progress.WithColors(m.ctx.Theme.FaintText, m.ctx.Theme.FaintText),
			progress.WithoutPercentage(),
		)
		greyProg.SetWidth(m.dbProg.Width())
		dbRow = lipgloss.JoinHorizontal(
			lipgloss.Left,
			labelCol.Render(labelStyle.Render("DB")),
			" ",
			valueCol.Render(valueStyle.Render("N/A")),
			" ",
			greyProg.ViewAs(0),
		)
	}

	perfContent := "\n" + strings.Join([]string{cpuRow, memRow, dbRow}, "\n\n")
	perfView := borderWithTitle(pad.Render(perfContent), "Performance", colWidth3, m.height,
		m.ctx.Theme.PrimaryBorder, m.ctx.Theme.SecondaryText)

	return lipgloss.JoinHorizontal(lipgloss.Top, projectView, pipelineView, perfView)
}
