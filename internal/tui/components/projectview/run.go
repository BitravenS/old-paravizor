package projectview

import (
	gocontext "context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	log "charm.land/log/v2"

	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/events"
	"github.com/bitravens/paravizor/v1/internal/runner"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

// startRun wires up the engine and runs it in a goroutine.
func (m *Model) startRun() tea.Cmd {
	m.running = true
	m.runErr = nil
	for i := range m.nodes {
		m.nodes[i].Status = engine.NodeStatusIdle
		m.nodes[i].ItemsIn = 0
		m.nodes[i].ItemsOut = 0
	}

	m.eventCh = make(chan events.Event, 256)
	m.doneCh = make(chan error, 1)

	ctx, cancel := gocontext.WithCancel(gocontext.Background())
	m.cancel = cancel

	eventCh := m.eventCh
	doneCh := m.doneCh
	projectDir := m.projectDir
	pipeline := m.ctx.Pipeline
	cfgDB := m.ctx.Config.DBConfig
	includeTargets := []string{}
	if m.projCfg != nil {
		includeTargets = append(includeTargets, m.projCfg.Scope.Include...)
	}
	configDir, _ := utils.PrvzrConfigDir()

	log.Info("pipeline run requested",
		"project_dir", projectDir,
		"pipeline", pipeline.Name,
		"targets", len(includeTargets),
	)

	go func() {
		doneCh <- runner.Run(ctx, eventCh, projectDir, pipeline, cfgDB, includeTargets, configDir)
	}()

	m.appendLog("Starting pipeline run...")
	return waitEvent(m.eventCh, m.doneCh)
}

// stopRun cancels the running pipeline.
func (m *Model) stopRun() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.running = false
	m.appendLog("Run cancelled.")
}

// waitEvent is the BubbleTea bridge: waits for either an event or completion.
func waitEvent(eventCh <-chan events.Event, doneCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case e, ok := <-eventCh:
			if !ok {
				return msgRunCompleted{}
			}
			return msgEngineEvent{event: e}
		case err := <-doneCh:
			return msgRunCompleted{err: err}
		}
	}
}

// applyEvent updates node statuses and log lines from engine events.
func (m *Model) applyEvent(e events.Event) {
	ts := e.Timestamp().Format("15:04:05")

	switch v := e.(type) {
	case events.PipelineStarted:
		log.Info("event pipeline.started", "nodes", v.NodeCount, "pipeline_id", v.PipelineID)
		m.appendLog(fmt.Sprintf("[%s] Pipeline started (%d nodes)", ts, v.NodeCount))

	case events.PipelineCompleted:
		log.Info("event pipeline.completed", "duration", v.Duration, "total_items", v.TotalItems, "total_errors", v.TotalErrors)
		m.appendLog(fmt.Sprintf("[%s] Pipeline done — %d items, %d errors, %s",
			ts, v.TotalItems, v.TotalErrors, v.Duration.Round(time.Millisecond)))

	case events.NodeStarted:
		log.Info("event node.started", "node_id", v.NodeID)
		if i := m.nodeIndex(v.NodeID); i >= 0 {
			m.nodes[i].Status = engine.NodeStatusActive
		}
		m.appendLog(fmt.Sprintf("[%s] Node started: %s", ts, v.NodeID))

	case events.NodeCompleted:
		log.Info("event node.completed", "node_id", v.NodeID, "items_in", v.ItemsIn, "items_out", v.ItemsOut, "duration", v.Duration)
		if i := m.nodeIndex(v.NodeID); i >= 0 {
			m.nodes[i].Status = engine.NodeStatusCompleted
			m.nodes[i].ItemsIn = v.ItemsIn
			m.nodes[i].ItemsOut = v.ItemsOut
		}
		m.appendLog(fmt.Sprintf("[%s] Node done: %s  in=%d out=%d  (%s)",
			ts, v.NodeID, v.ItemsIn, v.ItemsOut, v.Duration.Round(time.Millisecond)))

	case events.NodeError:
		log.Error("event node.error", "node_id", v.NodeID, "fatal", v.Fatal, "err", v.Err)
		if i := m.nodeIndex(v.NodeID); i >= 0 {
			m.nodes[i].Status = engine.NodeStatusError
		}
		m.appendLog(fmt.Sprintf("[%s] Node error: %s — %v", ts, v.NodeID, v.Err))

	case events.FindingDiscovered:
		log.Info("event item.finding.discovered", "severity", v.Severity, "title", v.Title, "node_id", v.NodeID)
		m.appendLog(fmt.Sprintf("[%s] finding [%s] %s  (by: %s)", ts, v.Severity, v.Title, v.Scanner))

	case events.ProcessStarted:
		log.Info("event process.started", "tool", v.ToolName, "pid", v.PID)
		m.appendLog(fmt.Sprintf("[%s] exec: %s (pid %d)", ts, v.ToolName, v.PID))

	case events.ProcessOutput:
		log.Debug("event process.output", "stream", v.Stream, "line", v.Line)
		if v.Stream == "stderr" {
			m.appendLog(fmt.Sprintf("[%s] stderr: %s", ts, v.Line))
		}

	case events.ProcessCompleted:
		log.Info("event process.completed", "tool", v.ToolName, "exit_code", v.ExitCode, "duration", v.Duration)

	default:
		log.Debug("event other", "type", e.EventType())
	}
}
