package home

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bitravens/paravizor/v1/internal/engine"
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
type catalogLoadedMsg struct {
	pipelines []CatalogEntry
	tools     []CatalogEntry
}

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
	focus          focusArea
	pipelines      []CatalogEntry
	tools          []CatalogEntry
	catalogTab     int // 0 = Pipelines, 1 = Tools
	pipelineCursor int
	pipelineScroll int
	toolCursor     int
	toolScroll     int

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
		var pipelines, tools []CatalogEntry
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
				pipelines = append(pipelines, e)
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
			tools = append(tools, e)
		}
		return catalogLoadedMsg{pipelines: pipelines, tools: tools}
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

func (m Model) Focused() bool {
	return m.leftState == panelCreate
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.adjustScroll()

	case animTickMsg:
		m.cat.Tick()
		return m, animTick()

	case catalogLoadedMsg:
		m.pipelines = msg.pipelines
		m.tools = msg.tools
		m.adjustScroll()

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
		case "t":
			m.catalogTab = 1 - m.catalogTab
			m.adjustScroll()
		case "tab":
			if m.focus == focusLeft {
				m.focus = focusRight
			} else {
				m.focus = focusLeft
			}
		case "up", "k":
			if m.focus == focusLeft && m.actionCursor > 0 {
				m.actionCursor--
			} else if m.focus == focusRight {
				if m.catalogTab == 0 && m.pipelineCursor > 0 {
					m.pipelineCursor--
				} else if m.catalogTab == 1 && m.toolCursor > 0 {
					m.toolCursor--
				}
				m.adjustScroll()
			}
		case "down", "j":
			actions := 1 // "New Project"
			if m.focus == focusLeft && m.actionCursor < actions-1 {
				m.actionCursor++
			} else if m.focus == focusRight {
				if m.catalogTab == 0 && m.pipelineCursor < len(m.pipelines)-1 {
					m.pipelineCursor++
				} else if m.catalogTab == 1 && m.toolCursor < len(m.tools)-1 {
					m.toolCursor++
				}
				m.adjustScroll()
			}
		case "enter":
			if m.focus == focusLeft {
				// Show inline create form
				m.leftState = panelCreate
				m.createInput.SetValue("")
				m.createInput.Focus()
			} else if m.focus == focusRight {
				if m.catalogTab == 0 && m.pipelineCursor < len(m.pipelines) {
					entry := m.pipelines[m.pipelineCursor]
					return m, func() tea.Msg { return CatalogSelectMsg{Entry: entry} }
				} else if m.catalogTab == 1 && m.toolCursor < len(m.tools) {
					entry := m.tools[m.toolCursor]
					return m, func() tea.Msg { return CatalogSelectMsg{Entry: entry} }
				}
			}
		case "n":
			if m.leftState != panelCreate {
				m.leftState = panelCreate
				m.createInput.SetValue("")
				m.createInput.Focus()
			}
		}
	}
	return m, nil
}

func (m *Model) adjustScroll() {
	minBottomH := 8
	topH := AnimHeight + 4
	if m.h-topH < minBottomH {
		topH = m.h - minBottomH
	}
	if topH < 6 {
		topH = 6
	}
	bottomH := m.h - topH

	visibleItems := bottomH - 6 // border(2) + padding(2) + title(1) + divider(1)
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.catalogTab == 0 {
		if m.pipelineCursor < m.pipelineScroll {
			m.pipelineScroll = m.pipelineCursor
		} else if m.pipelineCursor >= m.pipelineScroll+visibleItems {
			m.pipelineScroll = m.pipelineCursor - visibleItems + 1
		}
	} else {
		if m.toolCursor < m.toolScroll {
			m.toolScroll = m.toolCursor
		} else if m.toolCursor >= m.toolScroll+visibleItems {
			m.toolScroll = m.toolCursor - visibleItems + 1
		}
	}
}
