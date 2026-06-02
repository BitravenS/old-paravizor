package projectview

import (
	"context"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bitravens/paravizor/v1/internal/ai"
)

func (m *Model) startAIAnalysis() tea.Cmd {
	projectDir := m.projectDir
	cfg := m.ctx.Config.AIConfig
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		text, err := ai.AnalyzeProject(ctx, cfg, projectDir)
		if err == nil {
			_ = os.WriteFile(ai.ReportPath(projectDir), []byte(text+"\n"), 0o644)
		}
		return msgAIAnalysisDone{text: text, err: err}
	}
}

func (m *Model) startAIChatQuestion(question string, history []ai.ChatMessage) tea.Cmd {
	projectDir := m.projectDir
	cfg := m.ctx.Config.AIConfig
	history = append([]ai.ChatMessage(nil), history...)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		answer, err := ai.AskProjectQuestion(ctx, cfg, projectDir, history, question)
		return msgAIChatDone{answer: answer, err: err}
	}
}
