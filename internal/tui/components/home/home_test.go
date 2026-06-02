package home

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bitravens/paravizor/v1/internal/config"
	"github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/theme"
	"github.com/bitravens/paravizor/v1/internal/tui/constants"
	tuictx "github.com/bitravens/paravizor/v1/internal/tui/context"
)

func TestOpenProjectShortcutShowsProjectBrowser(t *testing.T) {
	model := NewModel(testContext())

	updated, cmd := model.Update(key("o"))

	if cmd == nil {
		t.Fatal("open shortcut returned nil cmd, want consumed command")
	}
	if updated.leftState != panelOpen {
		t.Fatalf("leftState = %v, want panelOpen", updated.leftState)
	}
	if updated.focus != focusLeft {
		t.Fatalf("focus = %v, want focusLeft", updated.focus)
	}
}

func TestOpenProjectBrowserSubmitsSelectedProject(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "existing-project")
	model := NewModel(testContext())
	model.leftState = panelOpen
	model.projects = []ProjectEntry{
		{Name: "demo", Path: projectPath, Status: StatusOK},
	}

	_, cmd := model.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter returned nil cmd, want ActionOpenProject command")
	}

	gotMsg := cmd()
	msg, ok := gotMsg.(ActionMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want ActionMsg", gotMsg)
	}
	if msg.Action.Type != ActionOpenProject {
		t.Fatalf("Action.Type = %v, want ActionOpenProject", msg.Action.Type)
	}
	if msg.Action.ProjectPath != projectPath {
		t.Fatalf("ProjectPath = %q, want %q", msg.Action.ProjectPath, projectPath)
	}
}

func TestOpenProjectBrowserMovesSelection(t *testing.T) {
	firstPath := filepath.Join(t.TempDir(), "first")
	secondPath := filepath.Join(t.TempDir(), "second")
	model := NewModel(testContext())
	model.leftState = panelOpen
	model.projects = []ProjectEntry{
		{Name: "first", Path: firstPath, Status: StatusOK},
		{Name: "second", Path: secondPath, Status: StatusOK},
	}

	updated, _ := model.Update(key("down"))
	if updated.projectCursor != 1 {
		t.Fatalf("projectCursor = %d, want 1", updated.projectCursor)
	}

	_, cmd := updated.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter returned nil cmd, want ActionOpenProject command")
	}
	msg := cmd().(ActionMsg)
	if msg.Action.ProjectPath != secondPath {
		t.Fatalf("ProjectPath = %q, want %q", msg.Action.ProjectPath, secondPath)
	}
}

func TestOpenProjectBrowserEmptyDoesNotSubmit(t *testing.T) {
	model := NewModel(testContext())
	model.leftState = panelOpen

	_, cmd := model.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter returned nil cmd, want consumed command")
	}
	if got := cmd(); got != nil {
		t.Fatalf("cmd returned %T, want nil", got)
	}
}

func TestDiscoverProjectsIncludesRecentProject(t *testing.T) {
	cfg, err := project.CreateProject("demo", "", "", "default", "", nil, project.ScopeConfig{
		Include: []string{"example.com"},
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	projectDir, err := project.InitProject(t.TempDir(), *cfg)
	if err != nil {
		t.Fatalf("InitProject returned error: %v", err)
	}

	entries := discoverProjects([]string{projectDir})
	for _, entry := range entries {
		if filepath.Clean(entry.Path) == filepath.Clean(projectDir) {
			return
		}
	}
	t.Fatalf("discoverProjects() did not include recent project %q: %#v", projectDir, entries)
}

func testContext() *tuictx.ProgramContext {
	cfg := config.GetDefaultConfig()
	return &tuictx.ProgramContext{
		Config: &cfg,
		Theme:  theme.LoadTheme(nil),
		Window: constants.Dimensions{Width: 120, Height: 40},
	}
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
	case "up":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})
	case "down":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})
	default:
		return tea.KeyPressMsg(tea.Key{Text: s, Code: []rune(s)[0]})
	}
}
