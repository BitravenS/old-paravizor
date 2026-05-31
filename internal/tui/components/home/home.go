package home

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/theme"
	"github.com/bitravens/paravizor/v1/internal/tool"
	tuictx "github.com/bitravens/paravizor/v1/internal/tui/context"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

type ActionType int

const (
	ActionCreateProject ActionType = iota
	ActionOpenProject
)

type Item struct {
	Type        ActionType
	ProjectPath string
}

type ActionMsg struct{ Action Item }
type CatalogSelectMsg struct{ Entry CatalogEntry }
type catalogLoadedMsg struct {
	pipelines []CatalogEntry
	tools     []CatalogEntry
}

type EntryStatus int

const (
	StatusOK EntryStatus = iota
	StatusWarn
	StatusError
)

type CatalogEntry struct {
	Kind, Name, Path, RawYAML string
	Status                    EntryStatus
	StatusMsg                 string
	NotInstall                bool
}

type Model struct {
	ctx       *tuictx.ProgramContext
	w, h      int
	pipelines []CatalogEntry
	tools     []CatalogEntry
	tab       int
	cursor    int
	scroll    int
}

func NewModel(ctx *tuictx.ProgramContext) Model {
	return Model{ctx: ctx, w: ctx.Window.Width, h: ctx.Window.Height}
}

func (m Model) Init() tea.Cmd { return loadCatalog() }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case catalogLoadedMsg:
		m.pipelines, m.tools = msg.pipelines, msg.tools
	case tea.KeyMsg:
		return m.updateBrowse(msg)
	}
	return m, nil
}

func (m Model) updateBrowse(msg tea.KeyMsg) (Model, tea.Cmd) {
	entries := m.currentEntries()
	switch msg.String() {
	case "t":
		m.tab, m.cursor, m.scroll = 1-m.tab, 0, 0
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.clampScroll()
		}
	case "down", "j":
		if m.cursor < len(entries)-1 {
			m.cursor++
			m.clampScroll()
		}
	case "enter":
		if m.cursor < len(entries) {
			e := entries[m.cursor]
			return m, func() tea.Msg { return CatalogSelectMsg{Entry: e} }
		}
	case "n":
		return m, func() tea.Msg { return ActionMsg{Action: Item{Type: ActionCreateProject}} }
	}
	return m, nil
}

func (m Model) currentEntries() []CatalogEntry {
	if m.tab == 0 {
		return m.pipelines
	}
	return m.tools
}

func (m *Model) clampScroll() {
	vis := m.h - 4
	if vis < 1 {
		vis = 1
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	} else if m.cursor >= m.scroll+vis {
		m.scroll = m.cursor - vis + 1
	}
}

func (m Model) View() string {
	if m.w <= 0 || m.h <= 0 {
		return ""
	}
	th := m.ctx.Theme
	faint := lipgloss.NewStyle().Foreground(th.FaintText)
	hi := lipgloss.NewStyle().Foreground(th.WarningText).Bold(true)

	var b strings.Builder
	b.WriteString(hi.Render("Paravizor") + "  " + faint.Render(m.ctx.Version) + "\n")
	b.WriteString(faint.Render("n:new  s:settings  ?:help  f:footer  q:quit") + "\n\n")

	for i, t := range []string{"Pipelines", "Tools"} {
		if i == m.tab {
			b.WriteString(hi.Render("[" + t + "]"))
		} else {
			b.WriteString(faint.Render(" " + t + " "))
		}
		b.WriteString("  ")
	}
	b.WriteString(faint.Render("t:switch") + "\n\n")

	entries := m.currentEntries()
	if len(entries) == 0 {
		b.WriteString(faint.Render("  Loading…"))
	} else {
		vis := m.h - 8
		if vis < 1 {
			vis = 1
		}
		end := m.scroll + vis
		if end > len(entries) {
			end = len(entries)
		}
		for i := m.scroll; i < end; i++ {
			e := entries[i]
			icon, iconColor := entryIcon(e.Status, th)
			sel := i == m.cursor
			prefix := "  "
			if sel {
				prefix = "▶ "
			}
			nameSt := lipgloss.NewStyle().Foreground(th.PrimaryText)
			if sel {
				nameSt = hi
			}
			line := lipgloss.NewStyle().Foreground(iconColor).Render(icon) + " " + nameSt.Render(prefix+e.Name)
			if i == m.scroll && m.scroll > 0 {
				line += faint.Render(" ↑")
			} else if i == end-1 && end < len(entries) {
				line += faint.Render(" ↓")
			}
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func entryIcon(s EntryStatus, th *theme.Theme) (string, color.Color) {
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

// RenderYAMLPopupContent builds popup body for a catalog entry.
func RenderYAMLPopupContent(entry CatalogEntry, ctx *tuictx.ProgramContext) string {
	th := ctx.Theme
	icon, iconColor := entryIcon(entry.Status, th)
	badge := lipgloss.NewStyle().Foreground(iconColor).Bold(true)
	var statusLine string
	if entry.NotInstall {
		statusLine = lipgloss.NewStyle().Foreground(th.WarningText).Render("⚠ Binary not installed")
	} else if entry.StatusMsg != "" {
		statusLine = badge.Render(icon + " " + entry.StatusMsg)
	} else {
		statusLine = badge.Render(icon + " Valid")
	}
	kindLabel := lipgloss.NewStyle().Foreground(th.FaintText).Render(entry.Kind + " · " + entry.Name)
	return lipgloss.JoinVertical(lipgloss.Left, kindLabel, statusLine, "", renderYAML(entry.RawYAML, ctx))
}

func renderYAML(raw string, ctx *tuictx.ProgramContext) string {
	th := ctx.Theme
	key := lipgloss.NewStyle().Foreground(th.WarningText)
	val := lipgloss.NewStyle().Foreground(th.PrimaryText)
	cmt := lipgloss.NewStyle().Foreground(th.FaintText).Italic(true)
	sep := lipgloss.NewStyle().Foreground(th.SecondaryText)
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			out = append(out, cmt.Render(line))
		} else if t == "---" {
			out = append(out, sep.Render(line))
		} else if idx := strings.Index(line, ": "); idx >= 0 {
			out = append(out, key.Render(line[:idx+1])+val.Render(line[idx+1:]))
		} else if strings.HasSuffix(t, ":") {
			out = append(out, key.Render(line))
		} else {
			out = append(out, val.Render(line))
		}
	}
	return strings.Join(out, "\n")
}

func findToolYAML(toolsDir, name string) []byte {
	raw, _ := os.ReadFile(filepath.Join(toolsDir, name+".yaml"))
	if len(raw) > 0 {
		return raw
	}
	if files, err := os.ReadDir(toolsDir); err == nil {
		for _, f := range files {
			if data, _ := os.ReadFile(filepath.Join(toolsDir, f.Name())); strings.Contains(string(data), "name: "+name) {
				return data
			}
		}
	}
	return nil
}

func loadCatalog() tea.Cmd {
	return func() tea.Msg {
		prvzrDir, err := utils.PrvzrConfigDir()
		if err != nil {
			return catalogLoadedMsg{}
		}
		var pipelines, tools []CatalogEntry

		pipelinesDir := filepath.Join(prvzrDir, "pipelines")
		if files, err := os.ReadDir(pipelinesDir); err == nil {
			for _, f := range files {
				ext := filepath.Ext(f.Name())
				if f.IsDir() || (ext != ".yaml" && ext != ".yml") {
					continue
				}
				path := filepath.Join(pipelinesDir, f.Name())
				raw, _ := os.ReadFile(path)
				e := CatalogEntry{Kind: "pipeline", Name: strings.TrimSuffix(f.Name(), ext), Path: path, RawYAML: string(raw)}
				if _, err := engine.ParsePipelineConfig(path); err != nil {
					e.Status, e.StatusMsg = StatusError, err.Error()
				}
				pipelines = append(pipelines, e)
			}
		}

		toolsDir := filepath.Join(prvzrDir, "tools")
		reg := tool.NewRegistry()
		_ = reg.LoadDir(toolsDir)
		reg.CheckAvailability(nil)
		for _, def := range reg.All() {
			raw := findToolYAML(toolsDir, def.Name)
			e := CatalogEntry{Kind: "tool", Name: def.Name, Path: filepath.Join(toolsDir, def.Name+".yaml"), RawYAML: string(raw)}
			if !def.Available {
				e.Status, e.StatusMsg, e.NotInstall = StatusWarn, fmt.Sprintf("binary %q not found", def.Binary), true
			}
			if verr := tool.ValidateTool(def); verr != nil {
				e.Status, e.StatusMsg = StatusError, verr.Error()
			}
			tools = append(tools, e)
		}
		return catalogLoadedMsg{pipelines: pipelines, tools: tools}
	}
}
