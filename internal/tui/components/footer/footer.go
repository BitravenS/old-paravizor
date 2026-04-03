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
	"charm.land/bubbles/v2/textarea"
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
	DBSizeMB   float64
	HasDB      bool
}

// metricsClockMsg is an internal tick used to trigger a metrics refresh.
type metricsClockMsg struct{}

// TickMetrics arms a 2-second repeating clock that drives metric refreshes.
func TickMetrics() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return metricsClockMsg{}
	})
}

// FetchPerformanceMetrics collects real CPU, memory, and DB size data off the
// main goroutine and delivers the result as a PerformanceTickMsg.
func FetchPerformanceMetrics(ctx *context.ProgramContext, s *store.Store) tea.Cmd {
	return func() tea.Msg {
		msg := PerformanceTickMsg{}

		if ctx.Project != nil && ctx.ProjectDir != "" {
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
		return msg
	}
}

type MiddleView int

const (
	MiddleViewPipeline MiddleView = iota
	MiddleViewTools
)

const dbCapMB = 1024.0 // 1 GB shown as 100% in the DB bar

type Model struct {
	ctx        *context.ProgramContext
	store      *store.Store
	notes      textarea.Model
	cpuProg    progress.Model
	memProg    progress.Model
	dbProg     progress.Model
	middleView MiddleView
	height     int
	width      int
	cpuPct     float64
	memPct     float64
	dbSizeMB   float64
	hasDB      bool
}

func NewModel(ctx *context.ProgramContext, s *store.Store) Model {
	ta := textarea.New()
	ta.Placeholder = "Write sticky notes here..."
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	styles := textarea.DefaultDarkStyles()
	styles.Focused.CursorLine = lipgloss.NewStyle()
	styles.Blurred.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(styles)

	return Model{
		ctx:        ctx,
		store:      s,
		notes:      ta,
		cpuProg:    progress.New(progress.WithColors(lipgloss.Color("#FF7CCB"), lipgloss.Color("#FDFF8C"))),
		memProg:    progress.New(progress.WithColors(lipgloss.Color("#8CFFB8"), lipgloss.Color("#7CFFF6"))),
		dbProg:     progress.New(progress.WithColors(lipgloss.Color("#7C95FF"), lipgloss.Color("#B88CFF"))),
		middleView: MiddleViewPipeline,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(TickMetrics(), FetchPerformanceMetrics(m.ctx, m.store))
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, m.height)

	case metricsClockMsg:
		cmds = append(cmds, TickMetrics(), FetchPerformanceMetrics(m.ctx, m.store))

	case PerformanceTickMsg:
		m.cpuPct = msg.CPUPercent
		m.memPct = msg.MemPercent
		m.dbSizeMB = msg.DBSizeMB
		m.hasDB = msg.HasDB

	case tea.KeyMsg:
		if msg.String() == "tab" {
			if m.middleView == MiddleViewPipeline {
				m.middleView = MiddleViewTools
			} else {
				m.middleView = MiddleViewPipeline
			}
		}
	}

	var cmd tea.Cmd
	m.notes, cmd = m.notes.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	colWidth := width / 3
	m.notes.SetWidth(colWidth - 4)
	m.notes.SetHeight(height - 4)

	barWidth := colWidth - 10
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

	// Left: Notes
	notesView := borderWithTitle(m.notes.View(), "Notes", colWidth1, m.height,
		m.ctx.Theme.PrimaryBorder, context.LogoColor)

	// Middle: Pipeline / Tools
	var midContent string
	if m.middleView == MiddleViewPipeline {
		if m.ctx.Pipeline != nil {
			midContent = fmt.Sprintf("\nName: %s\nStages: %d", m.ctx.Pipeline.Name, len(m.ctx.Pipeline.Stages))
		} else {
			midContent = "\nNo pipeline loaded."
		}
	} else {
		midContent = "\nPress Tab to switch views."
	}
	midTitle := "Pipeline Config"
	if m.middleView == MiddleViewTools {
		midTitle = "Tools Config"
	}
	midView := borderWithTitle(pad.Render(midContent), midTitle, colWidth2, m.height,
		m.ctx.Theme.PrimaryBorder, context.LogoColor)

	// Right: Performance — real values
	cpuLine := fmt.Sprintf("CPU %5.1f%%\n%s", m.cpuPct, m.cpuProg.ViewAs(clamp01(m.cpuPct/100)))
	memLine := fmt.Sprintf("MEM %5.1f%%\n%s", m.memPct, m.memProg.ViewAs(clamp01(m.memPct/100)))

	var dbLine string
	if m.hasDB {
		dbLine = fmt.Sprintf("DB  %.1f MB / 1 GB\n%s", m.dbSizeMB, m.dbProg.ViewAs(clamp01(m.dbSizeMB/dbCapMB)))
	} else {
		greyProg := progress.New(progress.WithColors(lipgloss.Color("#555555"), lipgloss.Color("#555555")))
		greyProg.SetWidth(m.dbProg.Width())
		dbLine = fmt.Sprintf("DB  N/A\n%s", greyProg.ViewAs(0))
	}

	perfContent := "\n" + strings.Join([]string{cpuLine, memLine, dbLine}, "\n\n")
	perfView := borderWithTitle(pad.Render(perfContent), "Performance", colWidth3, m.height,
		m.ctx.Theme.PrimaryBorder, context.LogoColor)

	return lipgloss.JoinHorizontal(lipgloss.Top, notesView, midView, perfView)
}
