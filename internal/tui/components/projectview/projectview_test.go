package projectview

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bitravens/paravizor/v1/internal/ai"
	"github.com/bitravens/paravizor/v1/internal/config"
	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/theme"
	"github.com/bitravens/paravizor/v1/internal/tui/constants"
	tuictx "github.com/bitravens/paravizor/v1/internal/tui/context"
)

func TestCtrlATogglesAIWindowWithoutLosingResponse(t *testing.T) {
	m := testProjectModel()
	m.aiText = "previous analysis"

	m.handleProjectKey(keyMsg("ctrl+a"))
	if m.activeWindow != projectWindowAI {
		t.Fatalf("activeWindow = %v, want AI", m.activeWindow)
	}

	m.handleProjectKey(keyMsg("ctrl+a"))
	if m.activeWindow != projectWindowRun {
		t.Fatalf("activeWindow = %v, want run", m.activeWindow)
	}
	if m.aiText != "previous analysis" {
		t.Fatalf("aiText = %q, want preserved response", m.aiText)
	}
}

func TestRunKeyStartsDifferentActionsByWindow(t *testing.T) {
	m := testProjectModel()

	cmds := m.handleProjectKey(keyMsg("r"))
	if len(cmds) != 1 {
		t.Fatalf("run window r returned %d cmds, want 1", len(cmds))
	}
	if !m.running {
		t.Fatal("run window r did not mark pipeline running")
	}

	m = testProjectModel()
	m.activeWindow = projectWindowAI
	cmds = m.handleProjectKey(keyMsg("r"))
	if len(cmds) != 1 {
		t.Fatalf("AI window r returned %d cmds, want 1", len(cmds))
	}
	if !m.aiRunning {
		t.Fatal("AI window r did not mark AI analysis running")
	}
	if m.running {
		t.Fatal("AI window r started the pipeline")
	}
}

func TestAIAnalysisBlockedWhilePipelineOrNodesRunning(t *testing.T) {
	m := testProjectModel()
	m.activeWindow = projectWindowAI
	m.running = true

	cmds := m.handleProjectKey(keyMsg("r"))
	if len(cmds) != 0 {
		t.Fatalf("AI r while pipeline running returned %d cmds, want 0", len(cmds))
	}
	if !strings.Contains(m.aiStatus, "locked") {
		t.Fatalf("aiStatus = %q, want locked message", m.aiStatus)
	}

	m.running = false
	m.nodes[0].Status = engine.NodeStatusActive
	cmds = m.handleProjectKey(keyMsg("r"))
	if len(cmds) != 0 {
		t.Fatalf("AI r while node active returned %d cmds, want 0", len(cmds))
	}

	m.nodes[0].Status = engine.NodeStatusCompleted
	cmds = m.handleProjectKey(keyMsg("r"))
	if len(cmds) != 1 {
		t.Fatalf("AI r after completion returned %d cmds, want 1", len(cmds))
	}
}

func TestAIChatFocusesWithCWithoutTypingC(t *testing.T) {
	m := testProjectModel()
	m.activeWindow = projectWindowAI

	updated, _ := m.Update(keyMsg("c"))
	if !updated.aiInput.Focused() {
		t.Fatal("c did not focus the AI chat input")
	}
	if updated.aiInput.Value() != "" {
		t.Fatalf("aiInput value = %q, want empty after focus shortcut", updated.aiInput.Value())
	}
	if !updated.Focused() {
		t.Fatal("project view did not report focused while AI chat input is focused")
	}

	updated, _ = updated.Update(keyMsg("c"))
	if updated.aiInput.Value() != "c" {
		t.Fatalf("focused aiInput value = %q, want typed c", updated.aiInput.Value())
	}
}

func TestAIChatEnterSendsQuestion(t *testing.T) {
	m := testProjectModel()
	m.activeWindow = projectWindowAI
	m.aiInput.Focus()
	m.aiInput.SetValue("Which live domains matter?")

	cmds := m.handleProjectKey(keyMsg("enter"))
	if len(cmds) != 1 {
		t.Fatalf("AI chat enter returned %d cmds, want 1", len(cmds))
	}
	if !m.aiQuestionRunning {
		t.Fatal("AI chat question did not mark question running")
	}
	if got := m.aiInput.Value(); got != "" {
		t.Fatalf("aiInput value = %q, want cleared after send", got)
	}
	if len(m.aiChat) != 1 || m.aiChat[0].Role != "user" || !strings.Contains(m.aiChat[0].Content, "live domains") {
		t.Fatalf("aiChat = %#v, want user question stored", m.aiChat)
	}
}

func TestAIChatDoneAppendsAssistantAnswer(t *testing.T) {
	m := testProjectModel()
	m.activeWindow = projectWindowAI
	m.aiQuestionRunning = true
	m.aiChat = append(m.aiChat, ai.ChatMessage{Role: "user", Content: "What should I inspect?"})

	updated, _ := m.Update(msgAIChatDone{answer: "Inspect the live app endpoint."})
	if updated.aiQuestionRunning {
		t.Fatal("AI chat completion left question running")
	}
	if len(updated.aiChat) != 2 {
		t.Fatalf("aiChat len = %d, want 2", len(updated.aiChat))
	}
	last := updated.aiChat[len(updated.aiChat)-1]
	if last.Role != "assistant" || !strings.Contains(last.Content, "live app") {
		t.Fatalf("last chat message = %#v, want assistant answer", last)
	}
}

func testProjectModel() Model {
	cfg := config.GetDefaultConfig()
	ctx := &tuictx.ProgramContext{
		Config: &cfg,
		Theme:  theme.LoadTheme(nil),
		Window: constants.Dimensions{Width: 120, Height: 40},
		Pipeline: &engine.PipelineConfig{
			Name: "test",
		},
	}
	return Model{
		ctx:             ctx,
		state:           stateProject,
		projectDir:      "/tmp/paravizor-test-project",
		nodes:           []nodeRow{{ID: "node-a", Name: "Node A", Status: engine.NodeStatusIdle}},
		width:           120,
		height:          40,
		activeProcesses: make(map[int64]processRow),
		rateAllocations: make(map[string]float64),
		activeWindow:    projectWindowRun,
		aiInput:         buildAIChatInput(ctx),
	}
}

func keyMsg(value string) tea.KeyMsg {
	switch value {
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	}
	if strings.HasPrefix(value, "ctrl+") && len(value) == len("ctrl+a") {
		return tea.KeyPressMsg(tea.Key{Code: rune(value[len(value)-1]), Mod: tea.ModCtrl})
	}
	return tea.KeyPressMsg(tea.Key{Text: value, Code: rune(value[0])})
}
