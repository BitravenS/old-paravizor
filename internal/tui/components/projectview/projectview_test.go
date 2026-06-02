package projectview

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bitravens/paravizor/v1/internal/bootstrap"
	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/events"
	"github.com/bitravens/paravizor/v1/internal/project"
	tuictx "github.com/bitravens/paravizor/v1/internal/tui/context"
)

func TestProjectViewLoadsProjectSpecificPipeline(t *testing.T) {
	configHome := filepath.Join(t.TempDir(), "xdg")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if err := bootstrap.Init(); err != nil {
		t.Fatalf("bootstrap init: %v", err)
	}

	custom := engine.PipelineConfig{
		Name: "custom",
		Init: []engine.InitConfig{
			{Scope: "exact", Node: "seed", ItemType: "domain"},
		},
		Stages: []engine.StageConfig{
			{ID: 0, Name: "Input"},
		},
		Nodes: []engine.NodeConfig{
			{
				ID:       "seed",
				Name:     "Seed",
				Stage:    0,
				Consumes: "domain",
				Produces: "domain",
			},
		},
	}
	if err := engine.WritePipelineConfig(filepath.Join(configHome, "paravizor", "pipelines", "custom.yaml"), custom); err != nil {
		t.Fatalf("write custom pipeline: %v", err)
	}

	projectDir := createProjectViewTestProjectWithPipeline(t, "custom")
	ctx, err := tuictx.NewProgramContext("")
	if err != nil {
		t.Fatalf("NewProgramContext returned error: %v", err)
	}

	model := NewModel(ctx, projectDir, nil)

	if model.ctx.Pipeline == nil || model.ctx.Pipeline.Name != "custom" {
		t.Fatalf("loaded pipeline = %#v, want custom", model.ctx.Pipeline)
	}
	if len(model.nodes) != 1 || model.nodes[0].ID != "seed" {
		t.Fatalf("nodes = %#v, want custom seed node", model.nodes)
	}
}

func TestProcessEventsTrackNodeID(t *testing.T) {
	model := Model{
		activeProcesses: make(map[int64]processRow),
	}

	model.applyEvent(events.ProcessStarted{
		ProcessID: 42,
		ToolName:  "subfinder",
		PID:       1234,
		NodeID:    "subdomain-passive-subfinder",
		Time:      time.Now(),
	})

	row, ok := model.activeProcesses[42]
	if !ok {
		t.Fatal("process row was not recorded")
	}
	if row.NodeID != "subdomain-passive-subfinder" {
		t.Fatalf("NodeID = %q, want subdomain-passive-subfinder", row.NodeID)
	}

	model.applyEvent(events.ProcessCompleted{
		ProcessID: 42,
		ToolName:  "subfinder",
		PID:       1234,
		NodeID:    "subdomain-passive-subfinder",
		Time:      time.Now(),
	})
	if len(model.activeProcesses) != 0 {
		t.Fatalf("activeProcesses = %#v, want empty after completion", model.activeProcesses)
	}
}

func TestLogMessageEventAppendsToProjectLog(t *testing.T) {
	model := Model{}

	model.applyEvent(events.LogMessage{
		Level:   "warn",
		Message: "tool subfinder is not installed or not on PATH; skipping node subdomain-passive-subfinder",
		Time:    time.Now(),
	})

	if len(model.logLines) != 1 {
		t.Fatalf("logLines len = %d, want 1", len(model.logLines))
	}
	if got := model.logLines[0]; got == "" || !containsAll(got, "WARN", "subfinder", "skipping node") {
		t.Fatalf("log line = %q, want warning with missing tool details", got)
	}
}

func createProjectViewTestProjectWithPipeline(t *testing.T, pipeline string) string {
	t.Helper()

	cfg, err := project.CreateProject("demo", "Demo project", "", "", "", nil, project.ScopeConfig{
		Include: []string{"example.com"},
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	if pipeline != "" {
		cfg.Pipeline = pipeline
	}

	projectDir, err := project.InitProject(t.TempDir(), *cfg)
	if err != nil {
		t.Fatalf("InitProject returned error: %v", err)
	}
	return projectDir
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
