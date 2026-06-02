// Package projectview provides the TUI page for creating and running recon projects.
package projectview

import (
	gocontext "context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bitravens/paravizor/v1/internal/ai"
	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/events"
	proj "github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/store"
	tuictx "github.com/bitravens/paravizor/v1/internal/tui/context"
)

// ── Messages ──────────────────────────────────────────────────────────────────

// MsgBack is sent when the user wants to return to the home screen.
type MsgBack struct{}

// msgRunCompleted is sent (internally) when the engine goroutine finishes.
type msgRunCompleted struct{ err error }

// msgEngineEvent carries a single engine event forwarded to the TUI.
type msgEngineEvent struct{ event events.Event }

// msgAIAnalysisDone carries the local AI assistant result.
type msgAIAnalysisDone struct {
	text string
	err  error
}

type msgAIChatDone struct {
	answer string
	err    error
}

// ── Internal page state ───────────────────────────────────────────────────────

type pageState int

type projectWindow int

const (
	stateCreate    pageState = iota // new-project form
	stateProject                    // project view with pipeline nodes + log
	stateEditScope                  // editing in/out of scope
)

const (
	projectWindowRun projectWindow = iota
	projectWindowAI
)

// nodeRow is the UI representation of a single pipeline node.
type nodeRow struct {
	ID       string
	Name     string
	Tool     string
	Status   engine.NodeStatus
	ItemsIn  int
	ItemsOut int
}

type processRow struct {
	ID     int64
	Tool   string
	PID    int
	NodeID string
}

const maxLogLines = 200

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the bubbletea model for the project page.
type Model struct {
	ctx   *tuictx.ProgramContext
	state pageState

	// ── Create form ──
	inputs  []textinput.Model
	focus   int
	formErr error

	// ── Project state ──
	projectDir string
	projCfg    *proj.ProjectConfig
	nodes      []nodeRow
	logLines   []string
	running    bool
	finished   bool
	runErr     error

	// ── Dashboard Metrics ──
	domainsCount  int
	liveCount     int
	urlsCount     int
	findingsCount int

	activeProcesses map[int64]processRow
	rateAllocations map[string]float64
	totalBudget     float64

	pipelineCursor int
	pipelineScroll int
	eventScroll    int
	processCursor  int
	activePanel    int // 0: pipeline, 1: events, 2: processes
	activeWindow   projectWindow
	selectedNode   *nodeRow
	showNodeLogs   bool
	nodeLogsTitle  string
	nodeLogsText   string
	nodeLogsScroll int

	aiText            string
	aiStatus          string
	aiRunning         bool
	aiScroll          int
	aiChat            []ai.ChatMessage
	aiInput           textinput.Model
	aiQuestionRunning bool

	// channels shared with goroutine (allocated once per run)
	eventCh chan events.Event
	doneCh  chan error
	cancel  gocontext.CancelFunc

	// layout
	width  int
	height int

	store *store.Store
}

func (m Model) Focused() bool {
	return m.state == stateCreate || m.state == stateEditScope || m.aiChatFocused()
}

// ProjectDir returns the active project directory for this view.
func (m Model) ProjectDir() string {
	return m.projectDir
}

// ── Construction ──────────────────────────────────────────────────────────────

// NewModel creates a project page. If existingDir is non-empty the form is
// skipped and the project is loaded directly.
func NewModel(ctx *tuictx.ProgramContext, existingDir string, st *store.Store) Model {
	m := Model{
		ctx:             ctx,
		store:           st,
		state:           stateCreate,
		width:           ctx.Window.Width,
		height:          ctx.Window.Height,
		activeProcesses: make(map[int64]processRow),
		rateAllocations: make(map[string]float64),
		activeWindow:    projectWindowRun,
		aiStatus:        "Press r to generate an AI recon analysis after the run finishes, or press c to ask about recon.",
		aiInput:         buildAIChatInput(ctx),
	}

	m.inputs = buildInputs(ctx)
	m.resizeAIInput()

	if existingDir != "" {
		m.loadExistingProject(existingDir)
	}

	return m
}

// loadExistingProject transitions directly to stateProject using an existing dir.
func (m *Model) loadExistingProject(dir string) {
	pcfg, err := proj.LoadProject(dir)
	if err != nil {
		m.formErr = fmt.Errorf("load project: %w", err)
		return
	}
	m.projectDir = dir
	m.projCfg = &pcfg
	m.state = stateProject
	m.rebuildNodes()
	m.loadMetricsFromStore()
}

// loadMetricsFromStore hydrates the TUI counts from the database on resume.
func (m *Model) loadMetricsFromStore() {
	if m.store == nil || m.store.DB() == nil {
		return
	}
	ctx := gocontext.Background()
	var count int

	_ = m.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM domains").Scan(&count)
	m.domainsCount = count

	_ = m.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM domains WHERE source = 'dnsx-live'").Scan(&count)
	m.liveCount = count

	_ = m.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM urls").Scan(&count)
	m.urlsCount = count

	_ = m.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM findings").Scan(&count)
	m.findingsCount = count

	for i, n := range m.nodes {
		var outCount int
		_ = m.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM pipeline_state WHERE node_id = ? AND status = 'completed'", n.ID).Scan(&outCount)
		m.nodes[i].ItemsOut = outCount

		if outCount > 0 {
			m.nodes[i].Status = engine.NodeStatusCompleted
		}
	}
}

// rebuildNodes syncs m.nodes from ctx.Pipeline.Nodes.
func (m *Model) rebuildNodes() {
	if m.ctx.Pipeline == nil {
		return
	}
	m.nodes = make([]nodeRow, len(m.ctx.Pipeline.Nodes))
	for i, n := range m.ctx.Pipeline.Nodes {
		m.nodes[i] = nodeRow{
			ID:     n.ID,
			Name:   n.Name,
			Tool:   n.Tool,
			Status: engine.NodeStatusIdle,
		}
	}
}

// nodeIndex returns the index into m.nodes for the given ID, or -1.
func (m *Model) nodeIndex(id string) int {
	for i, n := range m.nodes {
		if n.ID == id {
			return i
		}
	}
	return -1
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	aiChatWasFocused := m.aiChatFocused()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeAIInput()
		m.clampProjectScrolls()
		m.clampAIScroll()
		return m, nil

	case tea.KeyMsg:
		cmds = append(cmds, m.handleKey(msg)...)

	case msgEngineEvent:
		m.applyEvent(msg.event)
		// Re-arm the listener so the next event is forwarded.
		cmds = append(cmds, waitEvent(m.eventCh, m.doneCh))

	case msgAIAnalysisDone:
		m.aiRunning = false
		m.aiScroll = 0
		if msg.err != nil {
			m.aiStatus = "AI analysis failed."
			m.aiText = "AI analysis failed: " + msg.err.Error()
		} else {
			m.aiStatus = "AI analysis complete. Saved to ai-analysis.md. Press c to ask follow-up questions."
			m.aiText = msg.text
		}
		m.clampAIScroll()

	case msgAIChatDone:
		m.aiQuestionRunning = false
		if msg.err != nil {
			m.aiStatus = "AI chat failed."
			m.aiChat = append(m.aiChat, ai.ChatMessage{Role: "assistant", Content: "AI chat failed: " + msg.err.Error()})
		} else {
			m.aiStatus = "AI answered. Press c to ask another question."
			m.aiChat = append(m.aiChat, ai.ChatMessage{Role: "assistant", Content: msg.answer})
		}
		m.scrollAIToBottom()

	case msgRunCompleted:
		m.running = false
		m.finished = msg.err == nil
		m.activeProcesses = make(map[int64]processRow)
		m.clearRunningNodeStates(msg.err != nil)
		if msg.err != nil {
			m.runErr = msg.err
			m.appendLog("ERROR: " + msg.err.Error())
		} else {
			m.appendLog("Pipeline completed.")
			m.aiStatus = "Run finished. Switch to AI and press r to analyze recon results, or press c to ask a question."
		}
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
	}

	// Forward to text inputs when in create form.
	if m.state == stateCreate {
		for i := range m.inputs {
			t, cmd := m.inputs[i].Update(msg)
			m.inputs[i] = t
			cmds = append(cmds, cmd)
		}
	}

	if m.aiChatFocused() {
		_, isKey := msg.(tea.KeyMsg)
		if !isKey || aiChatWasFocused {
			t, cmd := m.aiInput.Update(msg)
			m.aiInput = t
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleKey(msg tea.KeyMsg) []tea.Cmd {
	switch m.state {
	case stateCreate:
		return m.handleCreateKey(msg)
	case stateEditScope:
		return m.handleEditScopeKey(msg)
	case stateProject:
		return m.handleProjectKey(msg)
	}
	return nil
}

func (m *Model) handleProjectKey(msg tea.KeyMsg) []tea.Cmd {
	if m.showNodeLogs {
		switch msg.String() {
		case "ctrl+a":
			m.showNodeLogs = false
			m.toggleProjectWindow()
		case "esc", "enter", "q":
			m.showNodeLogs = false
		case "up", "k":
			m.nodeLogsScroll--
		case "down", "j":
			m.nodeLogsScroll++
		case "pgup", "pageup", "b":
			m.nodeLogsScroll -= max(1, m.modalBodyHeight())
		case "pgdown", "pagedown", " ":
			m.nodeLogsScroll += max(1, m.modalBodyHeight())
		case "home", "g":
			m.nodeLogsScroll = 0
		case "end", "G":
			m.nodeLogsScroll = m.maxNodeLogsScroll()
		}
		m.clampNodeLogsScroll()
		return nil
	}

	if msg.String() == "ctrl+a" {
		m.toggleProjectWindow()
		return nil
	}

	if m.activeWindow == projectWindowAI {
		return m.handleAIWindowKey(msg)
	}
	return m.handleRunWindowKey(msg)
}

func (m *Model) handleRunWindowKey(msg tea.KeyMsg) []tea.Cmd {
	switch msg.String() {
	case "esc":
		if m.running {
			m.stopRun()
			return nil
		}
		return []tea.Cmd{func() tea.Msg { return MsgBack{} }}

	case "e":
		if !m.running && m.projCfg != nil {
			m.state = stateEditScope
			m.inputs = buildScopeInputs(m.ctx, m.projCfg)
			m.focus = 0
		}

	case "r", "ctrl+r":
		if !m.running {
			return []tea.Cmd{m.startRun()}
		}

	case "tab":
		m.activePanel = (m.activePanel + 1) % 3
		m.clampProjectScrolls()

	case "shift+tab":
		m.activePanel--
		if m.activePanel < 0 {
			m.activePanel = 2
		}
		m.clampProjectScrolls()

	case "down", "j":
		m.scrollActivePanel(1)

	case "up", "k":
		m.scrollActivePanel(-1)

	case "pgdown", "pagedown", " ":
		m.pageActivePanel(1)

	case "pgup", "pageup", "b":
		m.pageActivePanel(-1)

	case "home", "g":
		m.jumpActivePanel(false)

	case "end", "G":
		m.jumpActivePanel(true)

	case "enter":
		if m.activePanel == 0 && m.pipelineCursor < len(m.nodes) {
			node := m.nodes[m.pipelineCursor]
			m.showNodeLogs = true
			m.nodeLogsTitle = "Node Logs"
			m.nodeLogsText = m.loadNodeLogs(node.ID)
			m.nodeLogsScroll = 0
		} else if m.activePanel == 2 {
			if m.processCursor >= 0 && len(m.activeProcesses) > 0 {
				var procs []processRow
				for _, p := range m.activeProcesses {
					procs = append(procs, p)
				}
				sort.Slice(procs, func(i, j int) bool {
					return procs[i].ID < procs[j].ID
				})
				if m.processCursor < len(procs) {
					m.showNodeLogs = true
					m.nodeLogsTitle = "Node Logs"
					m.nodeLogsText = m.loadNodeLogs(procs[m.processCursor].NodeID)
					m.nodeLogsScroll = 0
				}
			}
		}
	}
	return nil
}

func (m *Model) handleAIWindowKey(msg tea.KeyMsg) []tea.Cmd {
	if m.aiInput.Focused() {
		switch msg.String() {
		case "enter":
			return m.submitAIChatQuestion()
		case "esc":
			m.aiInput.Blur()
			m.aiStatus = "Chat input blurred. Press c to ask another recon question."
		}
		return nil
	}

	switch msg.String() {
	case "esc":
		return []tea.Cmd{func() tea.Msg { return MsgBack{} }}
	case "c":
		return []tea.Cmd{m.focusAIChatInput()}
	case "r", "ctrl+r":
		return m.startAIAnalysisIfReady()
	case "up", "k":
		m.aiScroll--
	case "down", "j":
		m.aiScroll++
	case "pgup", "pageup", "b":
		m.aiScroll -= max(1, m.aiBodyHeight())
	case "pgdown", "pagedown", " ":
		m.aiScroll += max(1, m.aiBodyHeight())
	case "home", "g":
		m.aiScroll = 0
	case "end", "G":
		m.aiScroll = m.maxAIScroll()
	}
	m.clampAIScroll()
	return nil
}

func (m *Model) toggleProjectWindow() {
	if m.activeWindow == projectWindowAI {
		m.aiInput.Blur()
		m.activeWindow = projectWindowRun
		return
	}
	m.activeWindow = projectWindowAI
	m.resizeAIInput()
	if strings.TrimSpace(m.aiText) == "" && len(m.aiChat) == 0 && !m.aiRunning && !m.aiQuestionRunning {
		m.aiStatus = "Press r to generate an AI recon analysis after the run finishes, or press c to ask about recon."
	}
	m.clampAIScroll()
}

func (m *Model) startAIAnalysisIfReady() []tea.Cmd {
	if m.aiRunning {
		m.aiStatus = "AI analysis is already running."
		return nil
	}
	if m.aiQuestionRunning {
		m.aiStatus = "Wait for the current AI chat response before starting analysis."
		return nil
	}
	if m.projectDir == "" {
		m.aiStatus = "No project is loaded."
		return nil
	}
	if m.running || m.hasRunningNodes() {
		m.aiStatus = "AI analysis is locked until the pipeline is finished and no nodes are running."
		return nil
	}
	m.aiInput.Blur()
	m.aiRunning = true
	m.aiStatus = "Generating AI analysis with local Ollama. This can take a few minutes."
	if strings.TrimSpace(m.aiText) == "" {
		m.aiText = "Generating AI analysis with local Ollama..."
		m.aiScroll = 0
	}
	return []tea.Cmd{m.startAIAnalysis()}
}

func (m *Model) focusAIChatInput() tea.Cmd {
	m.activeWindow = projectWindowAI
	m.resizeAIInput()
	m.aiStatus = "Chat focused. Ask a question about the recon and press enter."
	return m.aiInput.Focus()
}

func (m *Model) submitAIChatQuestion() []tea.Cmd {
	question := strings.TrimSpace(m.aiInput.Value())
	if question == "" {
		m.aiStatus = "Type a recon question before sending."
		return nil
	}
	if m.aiQuestionRunning {
		m.aiStatus = "AI is already answering a question."
		return nil
	}
	if m.aiRunning {
		m.aiStatus = "Wait for the AI analysis to finish before chatting."
		return nil
	}
	if m.projectDir == "" {
		m.aiStatus = "No project is loaded."
		return nil
	}

	history := append([]ai.ChatMessage(nil), m.aiChat...)
	m.aiChat = append(m.aiChat, ai.ChatMessage{Role: "user", Content: question})
	m.aiInput.SetValue("")
	m.aiQuestionRunning = true
	if m.running || m.hasRunningNodes() {
		m.aiStatus = "Answering from the current recon snapshot while the run continues."
	} else {
		m.aiStatus = "Asking AI about the recon data."
	}
	m.scrollAIToBottom()
	return []tea.Cmd{m.startAIChatQuestion(question, history)}
}

func (m Model) hasRunningNodes() bool {
	if len(m.activeProcesses) > 0 {
		return true
	}
	for _, node := range m.nodes {
		if node.Status == engine.NodeStatusActive || node.Status == engine.NodeStatusDraining {
			return true
		}
	}
	return false
}

func (m *Model) clearRunningNodeStates(markError bool) {
	for i := range m.nodes {
		if m.nodes[i].Status != engine.NodeStatusActive && m.nodes[i].Status != engine.NodeStatusDraining {
			continue
		}
		if markError {
			m.nodes[i].Status = engine.NodeStatusError
		} else {
			m.nodes[i].Status = engine.NodeStatusCompleted
		}
	}
}

// appendLog adds a line to the log, capping at maxLogLines.
func (m *Model) appendLog(line string) {
	visibleBefore := m.eventsVisibleRows()
	wasAtBottom := m.eventScroll >= max(0, len(m.logLines)-visibleBefore)
	m.logLines = append(m.logLines, line)
	if overflow := len(m.logLines) - maxLogLines; overflow > 0 {
		m.logLines = m.logLines[overflow:]
		if !wasAtBottom {
			m.eventScroll -= overflow
		}
	}
	if wasAtBottom {
		visibleAfter := m.eventsVisibleRows()
		m.eventScroll = max(0, len(m.logLines)-visibleAfter)
	}
	m.clampEventScroll()
}

func (m Model) projectPanelRows() int {
	innerH := m.height - 3
	if innerH < 10 {
		innerH = 10
	}
	rows := innerH - 6
	if rows < 5 {
		rows = 5
	}
	return rows
}

func (m Model) pipelineVisibleRows() int {
	rows := max(1, m.projectPanelRows())
	if len(m.nodes) > rows {
		return max(1, rows-1)
	}
	return rows
}

func (m Model) eventsVisibleRows() int {
	rows := max(1, m.projectPanelRows())
	if len(m.logLines) > rows {
		return max(1, rows-1)
	}
	return rows
}

func (m *Model) scrollActivePanel(delta int) {
	switch m.activePanel {
	case 0:
		m.pipelineCursor += delta
		if m.pipelineCursor < 0 {
			m.pipelineCursor = 0
		}
		if m.pipelineCursor >= len(m.nodes) {
			m.pipelineCursor = max(0, len(m.nodes)-1)
		}
		m.adjustPipelineScroll()
	case 1:
		m.eventScroll += delta
		m.clampEventScroll()
	case 2:
		m.processCursor += delta
		if m.processCursor < 0 {
			m.processCursor = 0
		}
		if m.processCursor >= len(m.activeProcesses) {
			m.processCursor = max(0, len(m.activeProcesses)-1)
		}
	}
}

func (m *Model) pageActivePanel(direction int) {
	switch m.activePanel {
	case 0:
		rows := max(1, m.pipelineVisibleRows())
		m.pipelineCursor += direction * rows
		if m.pipelineCursor < 0 {
			m.pipelineCursor = 0
		}
		if m.pipelineCursor >= len(m.nodes) {
			m.pipelineCursor = max(0, len(m.nodes)-1)
		}
		m.adjustPipelineScroll()
	case 1:
		rows := max(1, m.eventsVisibleRows())
		m.eventScroll += direction * rows
		m.clampEventScroll()
	case 2:
		rows := max(1, m.projectPanelRows()-1)
		m.processCursor += direction * rows
		if m.processCursor < 0 {
			m.processCursor = 0
		}
		if m.processCursor >= len(m.activeProcesses) {
			m.processCursor = max(0, len(m.activeProcesses)-1)
		}
	}
}

func (m *Model) jumpActivePanel(end bool) {
	switch m.activePanel {
	case 0:
		if end {
			m.pipelineCursor = max(0, len(m.nodes)-1)
		} else {
			m.pipelineCursor = 0
		}
		m.adjustPipelineScroll()
	case 1:
		if end {
			m.eventScroll = max(0, len(m.logLines)-m.eventsVisibleRows())
		} else {
			m.eventScroll = 0
		}
		m.clampEventScroll()
	case 2:
		if end {
			m.processCursor = max(0, len(m.activeProcesses)-1)
		} else {
			m.processCursor = 0
		}
	}
}

func (m *Model) adjustPipelineScroll() {
	visible := max(1, m.pipelineVisibleRows())
	if m.pipelineCursor < m.pipelineScroll {
		m.pipelineScroll = m.pipelineCursor
	}
	if m.pipelineCursor >= m.pipelineScroll+visible {
		m.pipelineScroll = m.pipelineCursor - visible + 1
	}
	m.clampPipelineScroll()
}

func (m *Model) clampProjectScrolls() {
	m.clampPipelineScroll()
	m.clampEventScroll()
	m.clampNodeLogsScroll()
}

func (m *Model) clampPipelineScroll() {
	maxScroll := max(0, len(m.nodes)-max(1, m.pipelineVisibleRows()))
	if m.pipelineScroll < 0 {
		m.pipelineScroll = 0
	}
	if m.pipelineScroll > maxScroll {
		m.pipelineScroll = maxScroll
	}
}

func (m *Model) clampEventScroll() {
	maxScroll := max(0, len(m.logLines)-m.eventsVisibleRows())
	if m.eventScroll < 0 {
		m.eventScroll = 0
	}
	if m.eventScroll > maxScroll {
		m.eventScroll = maxScroll
	}
}

func (m *Model) clampNodeLogsScroll() {
	maxScroll := m.maxNodeLogsScroll()
	if m.nodeLogsScroll < 0 {
		m.nodeLogsScroll = 0
	}
	if m.nodeLogsScroll > maxScroll {
		m.nodeLogsScroll = maxScroll
	}
}

func (m Model) aiBodyWidth() int {
	w := m.width
	if w < 10 {
		w = 80
	}
	bodyW := w - 6
	if bodyW < 20 {
		bodyW = 20
	}
	return bodyW
}

func (m Model) aiBodyHeight() int {
	h := m.height - 3
	if h < 10 {
		h = 10
	}
	bodyH := h - 8
	if bodyH < 1 {
		bodyH = 1
	}
	return bodyH
}

func (m Model) maxAIScroll() int {
	return max(0, len(wrapPlainText(m.aiDisplayText(), m.aiBodyWidth()))-m.aiBodyHeight())
}

func (m *Model) clampAIScroll() {
	maxScroll := m.maxAIScroll()
	if m.aiScroll < 0 {
		m.aiScroll = 0
	}
	if m.aiScroll > maxScroll {
		m.aiScroll = maxScroll
	}
}

func (m *Model) scrollAIToBottom() {
	m.aiScroll = m.maxAIScroll()
	m.clampAIScroll()
}

func (m Model) aiChatFocused() bool {
	return m.state == stateProject && m.activeWindow == projectWindowAI && m.aiInput.Focused()
}

func (m *Model) resizeAIInput() {
	inputWidth := m.aiBodyWidth() - len(m.aiInput.Prompt)
	if inputWidth < 1 {
		inputWidth = 1
	}
	m.aiInput.SetWidth(inputWidth)
}

func (m Model) aiDisplayText() string {
	var sections []string
	if strings.TrimSpace(m.aiText) != "" {
		sections = append(sections, "Analysis\n\n"+strings.TrimSpace(m.aiText))
	}
	if len(m.aiChat) > 0 || m.aiQuestionRunning {
		sections = append(sections, m.aiChatDisplayText())
	}
	if len(sections) > 0 {
		return strings.Join(sections, "\n\n")
	}
	return "AI Assistant\n\nSwitch here with ctrl+a. Press r after the pipeline has finished to analyze all collected recon data. Press c to focus the chat box and ask questions about the current recon."
}

func (m Model) aiChatDisplayText() string {
	var b strings.Builder
	b.WriteString("Chat\n\n")
	for _, msg := range m.aiChat {
		role := "You"
		if strings.EqualFold(msg.Role, "assistant") {
			role = "Assistant"
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		b.WriteString(role)
		b.WriteString(":\n")
		b.WriteString(content)
		b.WriteString("\n\n")
	}
	if m.aiQuestionRunning {
		b.WriteString("Assistant:\nThinking...\n")
	}
	return strings.TrimSpace(b.String())
}

func (m *Model) loadNodeLogs(nodeID string) string {
	if m.projectDir == "" || nodeID == "" {
		return "No logs available."
	}
	dir := filepath.Join(m.projectDir, "logs", nodeID)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return "No logs found for node " + nodeID
	}

	// Just read the latest stderr/stdout
	var latestFile os.DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			latestFile = e
		}
	}
	if latestFile == nil {
		return "No logs found for node " + nodeID
	}

	path := filepath.Join(dir, latestFile.Name())
	content, err := os.ReadFile(path)
	if err != nil {
		return "Error reading log: " + err.Error()
	}
	return string(content)
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	switch m.state {
	case stateCreate:
		return m.viewCreate()
	case stateEditScope:
		return m.viewEditScope()
	case stateProject:
		return m.viewProject()
	}
	return ""
}
