package home

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bitravens/paravizor/v1/internal/engine"
	proj "github.com/bitravens/paravizor/v1/internal/project"
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
	projects  []ProjectEntry
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

type ProjectEntry struct {
	Name      string
	Path      string
	Status    EntryStatus
	StatusMsg string
}

// ── Left-panel state ──────────────────────────────────────────────────────────

type leftPanel int

const (
	panelActions leftPanel = iota
	panelOpen
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

	active bool

	// Left panel
	leftState     leftPanel
	actionCursor  int
	projects      []ProjectEntry
	projectCursor int
	projectScroll int

	// Right panel catalog
	focus          focusArea
	pipelines      []CatalogEntry
	tools          []CatalogEntry
	catalogTab     int // 0 = Pipelines, 1 = Tools
	pipelineCursor int
	pipelineScroll int
	toolCursor     int
	toolScroll     int
}

func NewModel(ctx *context.ProgramContext) Model {
	return Model{
		ctx:    ctx,
		w:      ctx.Window.Width,
		h:      ctx.Window.Height,
		active: true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadCatalog(m.recentProjects()))
}

func loadCatalog(recentProjects []string) tea.Cmd {
	return func() tea.Msg {
		var pipelines, tools []CatalogEntry
		projects := discoverProjects(recentProjects)
		prvzrDir, err := utils.PrvzrConfigDir()
		if err != nil {
			return catalogLoadedMsg{projects: projects}
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
		return catalogLoadedMsg{pipelines: pipelines, tools: tools, projects: projects}
	}
}

func discoverProjects(recentProjects []string) []ProjectEntry {
	const maxProjects = 80
	seen := make(map[string]bool)
	projects := make([]ProjectEntry, 0, len(recentProjects))

	addProject := func(path string) {
		if len(projects) >= maxProjects {
			return
		}
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return
		}
		abs = filepath.Clean(abs)
		key := strings.ToLower(abs)
		if seen[key] {
			return
		}

		if _, err := os.Stat(filepath.Join(abs, proj.ProjectConfigFile)); err != nil {
			return
		}

		entry := ProjectEntry{
			Name:   filepath.Base(abs),
			Path:   abs,
			Status: StatusOK,
		}
		if cfg, err := proj.LoadProject(abs); err == nil && cfg.Name != "" {
			entry.Name = cfg.Name
		} else if err != nil {
			entry.Status = StatusError
			entry.StatusMsg = err.Error()
		}

		seen[key] = true
		projects = append(projects, entry)
	}

	for _, path := range recentProjects {
		addProject(path)
	}

	for _, root := range projectSearchRoots() {
		if len(projects) >= maxProjects {
			break
		}
		scanProjectRoot(root, 4, addProject)
	}

	return projects
}

func projectSearchRoots() []string {
	seen := make(map[string]bool)
	var roots []string
	addRoot := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return
		}
		abs = filepath.Clean(abs)
		key := strings.ToLower(abs)
		if seen[key] {
			return
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			return
		}
		seen[key] = true
		roots = append(roots, abs)
	}

	if cwd, err := os.Getwd(); err == nil {
		addRoot(cwd)
		addRoot(filepath.Dir(cwd))
	}
	if home, err := os.UserHomeDir(); err == nil {
		addRoot(filepath.Join(home, "recon"))
		addRoot(filepath.Join(home, "projects"))
	}

	return roots
}

func scanProjectRoot(root string, maxDepth int, addProject func(string)) {
	var walk func(string, int)
	walk = func(dir string, depth int) {
		if _, err := os.Stat(filepath.Join(dir, proj.ProjectConfigFile)); err == nil {
			addProject(dir)
			return
		}
		if depth >= maxDepth {
			return
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})

		for _, entry := range entries {
			if !entry.IsDir() || shouldSkipProjectSearchDir(entry.Name()) {
				continue
			}
			walk(filepath.Join(dir, entry.Name()), depth+1)
		}
	}
	walk(root, 0)
}

func shouldSkipProjectSearchDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".idea", ".vscode", "node_modules", "vendor", "dist", "build", "bin", "obj":
		return true
	default:
		return false
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
	return false
}

func (m *Model) SetActive(active bool) {
	m.active = active
}

func (m *Model) ActivateLeft() {
	m.active = true
	m.focus = focusLeft
}

func (m *Model) FocusActions() {
	m.active = true
	m.focus = focusLeft
	m.leftState = panelActions
}

func (m *Model) FocusCatalog() {
	m.active = true
	m.focus = focusRight
}

func (m *Model) FocusProjectBrowser() {
	m.active = true
	m.focus = focusLeft
	m.leftState = panelOpen
	m.adjustScroll()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.adjustScroll()

	case catalogLoadedMsg:
		m.pipelines = msg.pipelines
		m.tools = msg.tools
		m.projects = msg.projects
		if m.projectCursor >= len(m.projects) {
			m.projectCursor = max(0, len(m.projects)-1)
		}
		m.adjustScroll()

	case tea.KeyMsg:
		if !m.active {
			return m, nil
		}

		if m.leftState == panelOpen {
			switch msg.String() {
			case "esc":
				m.leftState = panelActions
				return m, consumedCmd()
			case "r":
				return m, loadCatalog(m.recentProjects())
			case "n":
				return m, func() tea.Msg {
					return ActionMsg{Action: Item{Type: ActionCreateProject, ProjectPath: ""}}
				}
			case "up", "k":
				if m.projectCursor > 0 {
					m.projectCursor--
					m.adjustScroll()
				}
				return m, consumedCmd()
			case "down", "j":
				if m.projectCursor < len(m.projects)-1 {
					m.projectCursor++
					m.adjustScroll()
				}
				return m, consumedCmd()
			case "enter":
				if m.projectCursor >= 0 && m.projectCursor < len(m.projects) {
					selected := m.projects[m.projectCursor]
					if selected.Status == StatusError {
						return m, consumedCmd()
					}
					return m, func() tea.Msg {
						return ActionMsg{Action: Item{Type: ActionOpenProject, ProjectPath: selected.Path}}
					}
				}
				return m, consumedCmd()
			}
			return m, consumedCmd()
		}

		switch msg.String() {
		case "t":
			m.catalogTab = 1 - m.catalogTab
			m.adjustScroll()
			return m, consumedCmd()
		case "tab":
			if m.focus == focusLeft {
				m.focus = focusRight
			} else {
				m.focus = focusLeft
			}
			return m, consumedCmd()
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
			return m, consumedCmd()
		case "down", "j":
			actions := len(homeActions())
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
			return m, consumedCmd()
		case "enter":
			if m.focus == focusLeft {
				switch homeActions()[m.actionCursor].actionType {
				case ActionCreateProject:
					return m, func() tea.Msg {
						return ActionMsg{Action: Item{Type: ActionCreateProject, ProjectPath: ""}}
					}
				case ActionOpenProject:
					m.leftState = panelOpen
					m.focus = focusLeft
					m.adjustScroll()
					return m, consumedCmd()
				}
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
			return m, func() tea.Msg {
				return ActionMsg{Action: Item{Type: ActionCreateProject, ProjectPath: ""}}
			}
		case "o":
			m.leftState = panelOpen
			m.focus = focusLeft
			m.adjustScroll()
			return m, consumedCmd()
		}
	}
	return m, nil
}

func consumedCmd() tea.Cmd {
	return func() tea.Msg { return nil }
}

func (m Model) recentProjects() []string {
	if m.ctx == nil || m.ctx.Config == nil {
		return nil
	}
	return m.ctx.Config.RecentProjects
}

type homeAction struct {
	title      string
	desc       string
	actionType ActionType
}

func homeActions() []homeAction {
	return []homeAction{
		{"New Project", "Create a new recon project", ActionCreateProject},
		{"Open Project", "Browse existing projects", ActionOpenProject},
	}
}

func (m *Model) adjustScroll() {
	minBottomH := 8
	topH := m.h - minBottomH
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
	if m.leftState == panelOpen {
		visibleProjects := m.visibleProjectItems()
		if m.projectCursor < m.projectScroll {
			m.projectScroll = m.projectCursor
		} else if m.projectCursor >= m.projectScroll+visibleProjects {
			m.projectScroll = m.projectCursor - visibleProjects + 1
		}
	}
}

func (m Model) visibleProjectItems() int {
	visible := m.h - 10
	if visible < 1 {
		visible = 1
	}
	return visible
}
