package tool

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitravens/paravizor/v1/internal/events"
	"github.com/bitravens/paravizor/v1/internal/items"
)

// RunResult contains the output of a single tool execution.
type RunResult struct {
	Items    []items.Item
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Error    error
}

// RateLimiter is implemented by the credit pool to throttle tool dispatches.
type RateLimiter interface {
	// Wait blocks until the node's budget allows another dispatch.
	Wait(nodeID string)
}

// noopRateLimiter is used when no rate limiter is configured.
type noopRateLimiter struct{}

func (noopRateLimiter) Wait(_ string) {}

// Runner executes external tool processes, wires I/O, publishes events, and
// saves log files.
type Runner struct {
	bus            *events.Bus
	logsDir        string
	rateLimiter    RateLimiter
	processManager *ProcessManager
}

// NewRunner creates a new Runner with the given event bus and log directory.
// Pass an empty logsDir to disable log saving.
func NewRunner(bus *events.Bus, logsDir string) *Runner {
	return &Runner{
		bus:         bus,
		logsDir:     logsDir,
		rateLimiter: noopRateLimiter{},
	}
}

// WithRateLimiter returns a copy of the runner that uses the given rate limiter.
func (r *Runner) WithRateLimiter(rl RateLimiter) *Runner {
	return &Runner{
		bus:            r.bus,
		logsDir:        r.logsDir,
		rateLimiter:    rl,
		processManager: r.processManager,
	}
}

// WithProcessManager returns a copy of the runner that uses the given process manager.
func (r *Runner) WithProcessManager(pm *ProcessManager) *Runner {
	return &Runner{
		bus:            r.bus,
		logsDir:        r.logsDir,
		rateLimiter:    r.rateLimiter,
		processManager: pm,
	}
}

// Run executes a tool with the given input strings and returns parsed output items.
// input is a slice of raw string values (domain names, URLs, …) to pass to the tool.
// nodeID is the pipeline node identifier used for event tagging and log paths.
func (r *Runner) Run(ctx context.Context, def *ToolConfig, input []string, nodeID string) (*RunResult, error) {
	if !def.Available {
		return nil, fmt.Errorf("tool %s is not available (binary %q not found)", def.Name, def.Binary)
	}

	// Acquire a concurrency slot if a process manager is wired in.
	if r.processManager != nil {
		if err := r.processManager.Acquire(ctx); err != nil {
			return nil, fmt.Errorf("acquire process slot: %w", err)
		}
		defer r.processManager.Release()
	}

	start := time.Now()
	result := &RunResult{}

	args := r.buildArgs(def, input)
	cmd := exec.CommandContext(ctx, def.BinaryPath, args...)

	// Inherit the current environment and append any tool-specific overrides.
	cmd.Env = os.Environ()
	for k, v := range def.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// --- I/O setup ---
	var stdoutBuf, stderrBuf bytes.Buffer
	var stdinPipe io.WriteCloser

	needsStdin := def.Input.Type == "stdin" ||
		(def.Input.Bulk.Type == "stdin" && len(input) > 1)
	if needsStdin {
		var err error
		stdinPipe, err = cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("create stdin pipe: %w", err)
		}
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	// --- Start process ---
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", def.Name, err)
	}

	if r.processManager != nil {
		r.processManager.Register(cmd, nodeID, def.Name)
		defer r.processManager.Unregister(cmd)
	}

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	r.bus.Publish(events.ProcessStarted{
		ToolName: def.Name,
		Command:  fmt.Sprintf("%s %s", def.BinaryPath, strings.Join(args, " ")),
		PID:      pid,
		Time:     time.Now(),
	})

	// Feed stdin in a goroutine so it doesn't block the main execution path.
	if stdinPipe != nil {
		go func() {
			defer stdinPipe.Close()
			for _, item := range input {
				r.rateLimiter.Wait(nodeID)
				fmt.Fprintln(stdinPipe, item)
			}
		}()
	}

	// Drain stderr concurrently; publish each line as a ProcessOutput event.
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line)
			stderrBuf.WriteByte('\n')
			r.bus.Publish(events.ProcessOutput{
				Stream: "stderr",
				Line:   line,
				Time:   time.Now(),
			})
		}
	}()

	// Read all stdout — we need the full buffer for parsing.
	_, _ = io.Copy(&stdoutBuf, stdoutPipe)
	<-stderrDone

	// Wait for the process to exit.
	waitErr := cmd.Wait()
	result.Duration = time.Since(start)
	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()

	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = waitErr
		}
	}

	r.bus.Publish(events.ProcessCompleted{
		ToolName: def.Name,
		ExitCode: result.ExitCode,
		Duration: result.Duration,
		Time:     time.Now(),
	})

	// Parse output into items if the tool declares a produces type.
	if def.Produces != "" {
		produces := items.ItemType(def.Produces)
		parsed, parseErr := ParseOutput(
			strings.NewReader(result.Stdout),
			def.Output,
			produces,
			def.Name,
		)
		if parseErr != nil {
			result.Error = fmt.Errorf("parse output: %w", parseErr)
			return result, nil
		}
		result.Items = parsed
	}

	// Persist stdout/stderr to disk for later inspection.
	if r.logsDir != "" {
		r.saveLogs(def.Name, nodeID, result)
	}

	return result, nil
}

// buildArgs constructs the full argument slice for the tool process.
func (r *Runner) buildArgs(def *ToolConfig, input []string) []string {
	return r.buildArgsWithScope(def, input, nil, nil)
}

// buildArgsWithScope constructs arguments including optional scope flag injection.
// includePatterns and excludePatterns are injected when the tool declares scope flags.
func (r *Runner) buildArgsWithScope(def *ToolConfig, input []string, includePatterns, excludePatterns []string) []string {
	var args []string

	args = append(args, def.Flags...)
	args = append(args, def.UserFlags...)

	if def.Timeout.Flag != "" && def.Timeout.Default > 0 {
		args = append(args, def.Timeout.Flag, fmt.Sprintf("%d", def.Timeout.Default))
	}

	// Inject scope patterns if the tool supports them.
	if def.ScopeFlags.Include != "" && len(includePatterns) > 0 {
		for _, p := range includePatterns {
			args = append(args, def.ScopeFlags.Include, p)
		}
	}
	if def.ScopeFlags.Exclude != "" && len(excludePatterns) > 0 {
		for _, p := range excludePatterns {
			args = append(args, def.ScopeFlags.Exclude, p)
		}
	}

	// Handle input delivery.
	switch {
	case len(input) > 1 && def.Input.Bulk.Type == "file":
		tmpFile := r.writeTempInput(input, def.Input.Bulk.Separator)
		if tmpFile != "" {
			args = append(args, def.Input.Bulk.Flag, tmpFile)
		}
	case len(input) > 1 && def.Input.Bulk.Type == "stdin":
		// stdin is handled via stdinPipe in Run().
	case len(input) == 1 && def.Input.Type == "arg":
		if def.Input.Flag != "" {
			args = append(args, def.Input.Flag, input[0])
		} else {
			args = append(args, input[0])
		}
	}

	// Output file flag.
	if def.Output.Type == "file" && def.Output.Flag != "" && def.Output.Path != "" {
		args = append(args, def.Output.Flag, def.Output.Path)
	}

	return args
}

// writeTempInput writes input items to a temporary file and returns its path.
func (r *Runner) writeTempInput(input []string, separator string) string {
	if separator == "" {
		separator = "\n"
	}
	tmpFile, err := os.CreateTemp("", "paravizor-input-*")
	if err != nil {
		return ""
	}
	defer tmpFile.Close()

	for _, item := range input {
		fmt.Fprint(tmpFile, item+separator)
	}
	return tmpFile.Name()
}

// saveLogs persists stdout and stderr to the configured log directory.
func (r *Runner) saveLogs(toolName, nodeID string, result *RunResult) {
	dir := filepath.Join(r.logsDir, nodeID)
	_ = os.MkdirAll(dir, 0755)

	ts := time.Now().Format("20060102-150405")
	base := fmt.Sprintf("%s-%s", toolName, ts)

	if result.Stdout != "" {
		_ = os.WriteFile(filepath.Join(dir, base+".stdout"), []byte(result.Stdout), 0644)
	}
	if result.Stderr != "" {
		_ = os.WriteFile(filepath.Join(dir, base+".stderr"), []byte(result.Stderr), 0644)
	}
}
