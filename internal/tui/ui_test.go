package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bitravens/paravizor/v1/internal/bootstrap"
	"github.com/bitravens/paravizor/v1/internal/project"
)

func TestOpenProjectPathLoadsExistingProject(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	if err := bootstrap.Init(); err != nil {
		t.Fatalf("bootstrap init: %v", err)
	}

	projectDir := createTUITestProject(t)
	model := NewModel("", "test", nil)

	updatedModel, _ := model.openProjectPath(projectDir, nil)
	updated := updatedModel.(Model)

	if updated.state != ViewStateProject {
		t.Fatalf("state = %v, want ViewStateProject", updated.state)
	}
	if updated.Ctx.Project == nil || updated.Ctx.Project.Name != "demo" {
		t.Fatalf("Ctx.Project = %#v, want loaded demo project", updated.Ctx.Project)
	}
	if updated.Ctx.ProjectDir != projectDir {
		t.Fatalf("ProjectDir = %q, want %q", updated.Ctx.ProjectDir, projectDir)
	}
	if len(updated.Ctx.Config.RecentProjects) == 0 || updated.Ctx.Config.RecentProjects[0] != projectDir {
		t.Fatalf("RecentProjects = %#v, want project at front", updated.Ctx.Config.RecentProjects)
	}
}

func TestOpenProjectPathRejectsInvalidProject(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	if err := bootstrap.Init(); err != nil {
		t.Fatalf("bootstrap init: %v", err)
	}

	model := NewModel("", "test", nil)
	missing := filepath.Join(t.TempDir(), "missing")

	updatedModel, _ := model.openProjectPath(missing, nil)
	updated := updatedModel.(Model)

	if updated.state != ViewStateHome {
		t.Fatalf("state = %v, want ViewStateHome", updated.state)
	}
	if updated.Ctx.Project != nil {
		t.Fatalf("Ctx.Project = %#v, want nil after rejected open", updated.Ctx.Project)
	}
}

func TestHomeTabCyclesThroughRecentProjectsAndCatalog(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	if err := bootstrap.Init(); err != nil {
		t.Fatalf("bootstrap init: %v", err)
	}

	model := NewModel("", "test", nil)

	updatedModel, _ := model.Update(uiKey("tab"))
	updated := updatedModel.(Model)
	if updated.homeFocus != homeFocusSidebar {
		t.Fatalf("homeFocus after first tab = %v, want homeFocusSidebar", updated.homeFocus)
	}

	updatedModel, _ = updated.Update(uiKey("tab"))
	updated = updatedModel.(Model)
	if updated.homeFocus != homeFocusCatalog {
		t.Fatalf("homeFocus after second tab = %v, want homeFocusCatalog", updated.homeFocus)
	}

	updatedModel, _ = updated.Update(uiKey("tab"))
	updated = updatedModel.(Model)
	if updated.homeFocus != homeFocusActions {
		t.Fatalf("homeFocus after third tab = %v, want homeFocusActions", updated.homeFocus)
	}
}

func TestHomeRecentFocusEnterOpensProject(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	if err := bootstrap.Init(); err != nil {
		t.Fatalf("bootstrap init: %v", err)
	}

	projectDir := createTUITestProject(t)
	model := NewModel("", "test", nil)
	model.Ctx.Config.RecentProjects = []string{projectDir}
	model.homeFocus = homeFocusSidebar
	model.applyHomeFocus()

	updatedModel, cmd := model.Update(uiKey("enter"))
	if cmd == nil {
		t.Fatal("enter returned nil cmd, want sidebar project selection command")
	}

	msg := cmd()
	updatedModel, _ = updatedModel.(Model).Update(msg)
	updated := updatedModel.(Model)

	if updated.state != ViewStateProject {
		t.Fatalf("state = %v, want ViewStateProject", updated.state)
	}
	if updated.Ctx.ProjectDir != projectDir {
		t.Fatalf("ProjectDir = %q, want %q", updated.Ctx.ProjectDir, projectDir)
	}
}

func createTUITestProject(t *testing.T) string {
	t.Helper()

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
	return projectDir
}

func uiKey(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "tab":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})
	default:
		return tea.KeyPressMsg(tea.Key{Text: s, Code: []rune(s)[0]})
	}
}
