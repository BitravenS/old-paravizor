package home

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/theme"
	"github.com/bitravens/paravizor/v1/internal/tool"
	"github.com/bitravens/paravizor/v1/internal/tui/context"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

// ── Public messages ───────────────────────────────────────────────────────────

type ActionType int

const (
	ActionCreateProject ActionType = iota
	ActionOpenProject
)

type Item struct {
	Title       string
	Description string
	Type        ActionType
	ProjectPath string
}

type ActionMsg struct{ Action Item }

// CatalogSelectMsg is sent when the user presses enter on a pipeline or tool.
type CatalogSelectMsg struct{ Entry CatalogEntry }

// ── Internal messages ─────────────────────────────────────────────────────────

type animTickMsg struct{}
type catalogLoadedMsg struct{ entries []CatalogEntry }

// ── Catalog entry ─────────────────────────────────────────────────────────────

type EntryStatus int

const (
	StatusOK    EntryStatus = iota
	StatusWarn  EntryStatus = iota
	StatusError EntryStatus = iota
)

type CatalogEntry struct {
	Kind       string
	Name       string
	Path       string
	RawYAML    string
	Status     EntryStatus
	StatusMsg  string
	NotInstall bool
}

// ── Left-panel state ──────────────────────────────────────────────────────────

type leftPanel int

const (
	panelActions leftPanel = iota
	panelCreate
)

// ── Model ─────────────────────────────────────────────────────────────────────

type focusArea int

const (
	focusLeft  focusArea = iota
	focusRight focusArea = iota
)

type Model struct {
	ctx  *context.ProgramContext
	w, h int

	// Left panel
	leftState    leftPanel
	actionCursor int
	createInput  textinput.Model // project path input for inline create

	// Right panel catalog
	focus         focusArea
	catalog       []CatalogEntry
	catalogCursor int

	// Animation
	cat CatAnimation
}

func NewModel(ctx *context.ProgramContext) Model {
	inp := textinput.New()
	st := textinput.DefaultDarkStyles()
	st.Focused.Prompt = lipgloss.NewStyle().Foreground(ctx.Theme.WarningText)
	st.Focused.Text = lipgloss.NewStyle().Foreground(ctx.Theme.PrimaryText)
	st.Blurred.Prompt = lipgloss.NewStyle().Foreground(ctx.Theme.SecondaryText)
	st.Blurred.Text = lipgloss.NewStyle().Foreground(ctx.Theme.FaintText)
	st.Cursor.Color = ctx.Theme.WarningText
	inp.SetStyles(st)
	inp.Prompt = "Path: "
	inp.Placeholder = "/path/to/project"

	return Model{
		ctx:         ctx,
		w:           ctx.Window.Width,
		h:           ctx.Window.Height,
		createInput: inp,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(animTick(), loadCatalog())
}

func animTick() tea.Cmd {
	return tea.Tick(AnimInterval, func(time.Time) tea.Msg { return animTickMsg{} })
}

func loadCatalog() tea.Cmd {
	return func() tea.Msg {
		var entries []CatalogEntry
		prvzrDir, err := utils.PrvzrConfigDir()
		if err != nil {
			return catalogLoadedMsg{}
		}

		pipelinesDir := filepath.Join(prvzrDir, "pipelines")
		if files, err := os.ReadDir(pipelinesDir); err == nil {
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				ext := filepath.Ext(f.Name())
				if ext != ".yaml" && ext != ".yml" {
					continue
				}
				path := filepath.Join(pipelinesDir, f.Name())
				raw, _ := os.ReadFile(path)
				e := CatalogEntry{
					Kind: "pipeline", Name: strings.TrimSuffix(f.Name(), ext),
					Path: path, RawYAML: string(raw), Status: StatusOK,
				}
				if _, err := engine.ParsePipelineConfig(path); err != nil {
					e.Status = StatusError
					e.StatusMsg = err.Error()
				}
				entries = append(entries, e)
			}
		}

		toolsDir := filepath.Join(prvzrDir, "tools")
		reg := tool.NewRegistry()
		_ = reg.LoadDir(toolsDir)
		reg.CheckAvailability(nil)
		for _, def := range reg.All() {
			raw, _ := os.ReadFile(filepath.Join(toolsDir, def.Name+".yaml"))
			if len(raw) == 0 {
				for _, f := range yamlFiles(toolsDir) {
					data, _ := os.ReadFile(f)
					if strings.Contains(string(data), "name: "+def.Name) {
						raw = data
						break
					}
				}
			}
			e := CatalogEntry{
				Kind: "tool", Name: def.Name,
				Path:    filepath.Join(toolsDir, def.Name+".yaml"),
				RawYAML: string(raw), Status: StatusOK,
			}
			if !def.Available {
				e.Status = StatusWarn
				e.StatusMsg = fmt.Sprintf("binary %q not found", def.Binary)
				e.NotInstall = true
			}
			if verr := tool.ValidateTool(def); verr != nil {
				e.Status = StatusError
				e.StatusMsg = verr.Error()
			}
			entries = append(entries, e)
		}
		return catalogLoadedMsg{entries: entries}
	}
}

func yamlFiles(dir string) []string {
	var out []string
	if files, err := os.ReadDir(dir); err == nil {
		for _, f := range files {
			if ext := filepath.Ext(f.Name()); ext == ".yaml" || ext == ".yml" {
				out = append(out, filepath.Join(dir, f.Name()))
			}
		}
	}
	return out
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height

	case animTickMsg:
		m.cat.Tick()
		return m, animTick()

	case catalogLoadedMsg:
		m.catalog = msg.entries

	case tea.KeyMsg:
		// Inline create form active
		if m.leftState == panelCreate {
			switch msg.String() {
			case "esc":
				m.leftState = panelActions
				m.createInput.Blur()
			case "enter":
				path := strings.TrimSpace(m.createInput.Value())
				if path != "" {
					m.createInput.Blur()
					return m, func() tea.Msg {
						return ActionMsg{Action: Item{Type: ActionCreateProject, ProjectPath: path}}
					}
				}
			default:
				m.createInput, cmd = m.createInput.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		switch msg.String() {
		case "tab":
			if m.focus == focusLeft {
				m.focus = focusRight
			} else {
				m.focus = focusLeft
			}
		case "up", "k":
			if m.focus == focusLeft && m.actionCursor > 0 {
				m.actionCursor--
			} else if m.focus == focusRight && m.catalogCursor > 0 {
				m.catalogCursor--
			}
		case "down", "j":
			actions := 1 // "New Project"
			if m.focus == focusLeft && m.actionCursor < actions-1 {
				m.actionCursor++
			} else if m.focus == focusRight && m.catalogCursor < len(m.catalog)-1 {
				m.catalogCursor++
			}
		case "enter":
			if m.focus == focusLeft {
				// Show inline create form
				m.leftState = panelCreate
				m.createInput.SetValue("")
				m.createInput.Focus()
			} else if m.focus == focusRight && m.catalogCursor < len(m.catalog) {
				entry := m.catalog[m.catalogCursor]
				return m, func() tea.Msg { return CatalogSelectMsg{Entry: entry} }
			}
		case "n":
			m.leftState = panelCreate
			m.createInput.SetValue("")
			m.createInput.Focus()
		}
	}
	return m, nil
}

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
	// Top: info (left) + animation (right)
	// Bottom: catalog
	topH := AnimHeight + 4 // animation height + borders + padding
	bottomH := m.h - topH
	if bottomH < 6 {
		bottomH = 6
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

	var content string
	if len(m.catalog) == 0 {
		content = lipgloss.NewStyle().Foreground(th.FaintText).Render(" Loading…")
	} else {
		var pipelines, tools []CatalogEntry
		for _, e := range m.catalog {
			if e.Kind == "pipeline" {
				pipelines = append(pipelines, e)
			} else {
				tools = append(tools, e)
			}
		}
		leftW := inner / 2
		rightW := inner - 1 - leftW // -1 for divider; handles even/odd inner

		leftRows := m.renderSectionRows("Pipelines", pipelines, leftW, 0)
		rightRows := m.renderSectionRows("Tools", tools, rightW, len(pipelines))

		// Pad both columns to the same height.
		emptyL := lipgloss.NewStyle().Width(leftW).Render("")
		emptyR := lipgloss.NewStyle().Width(rightW).Render("")
		for len(leftRows) < len(rightRows) {
			leftRows = append(leftRows, emptyL)
		}
		for len(rightRows) < len(leftRows) {
			rightRows = append(rightRows, emptyR)
		}

		div := lipgloss.NewStyle().Foreground(th.FaintBorder).Render("│")
		lines := make([]string, len(leftRows))
		for i := range leftRows {
			lines[i] = leftRows[i] + div + rightRows[i]
		}
		content = strings.Join(lines, "\n")
	}

	border := th.FaintBorder
	if m.focus == focusRight {
		border = th.PrimaryBorder
	}
	return box(content, w, h, border, th.PrimaryText)
}

// renderSectionRows returns a slice of lines each exactly w display-columns wide.
func (m Model) renderSectionRows(title string, entries []CatalogEntry, w, offset int) []string {
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

	for i, e := range entries {
		globalIdx := offset + i
		selected := m.focus == focusRight && globalIdx == m.catalogCursor
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
		rows = append(rows, norm(iconStyled+" "+nameStyled))
	}
	return rows
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// box wraps content in a rounded border with 1-char internal padding on all sides.
func box(content string, w, h int, borderColor, _ color.Color) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 1).
		Width(w - 2).
		Height(h - 2).
		Render(content)
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
