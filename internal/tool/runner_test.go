package tool

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bitravens/paravizor/v1/internal/events"
)

func TestRunnerStopsToolWhenContextDeadlineExpires(t *testing.T) {
	runner := NewRunner(events.NewBus(), t.TempDir())
	def := &ToolConfig{
		Name:       "sleeper",
		Binary:     "sh",
		BinaryPath: "/bin/sh",
		Available:  true,
		Flags:      []string{"-c", "sleep 2; echo late"},
		Input:      InputConfig{Type: "none"},
		Output:     OutputConfig{Type: "stdout", Format: "line"},
		Consumes:   "domain",
		Produces:   "domain",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	result, err := runner.Run(ctx, def, nil, "timeout-node")
	if err != nil {
		t.Fatalf("Run returned setup error: %v", err)
	}
	if result == nil {
		t.Fatal("Run returned nil result")
	}
	if result.Error == nil {
		t.Fatal("result.Error is nil, want deadline error")
	}
	if !strings.Contains(result.Error.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("result.Error = %v, want deadline exceeded", result.Error)
	}
	if result.ExitCode != -1 {
		t.Fatalf("ExitCode = %d, want -1", result.ExitCode)
	}
	if time.Since(started) > time.Second {
		t.Fatalf("Run did not stop promptly; duration=%s", time.Since(started))
	}
}
