package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitravens/paravizor/v1/internal/project"
)

func TestInitCommandCreatesProjectAndRunsBootstrap(t *testing.T) {
	originalBootstrap := bootstrapInit
	defer func() {
		bootstrapInit = originalBootstrap
	}()

	bootstraps := 0
	bootstrapInit = func() error {
		bootstraps++
		return nil
	}

	projectRoot := t.TempDir()
	cmd := newRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{
		"init",
		"demo",
		"-d",
		projectRoot,
		"--include",
		"example.com",
		"--exclude",
		"dev.example.com",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command returned error: %v\nstderr:\n%s", err, errOut.String())
	}
	if bootstraps != 1 {
		t.Fatalf("init command bootstrapped %d times, want 1", bootstraps)
	}

	projectDir := filepath.Join(projectRoot, "demo")
	if _, err := project.LoadProject(projectDir); err != nil {
		t.Fatalf("created project did not load: %v", err)
	}
	if !strings.Contains(out.String(), "Initialized new Paravizor project at") {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}

func TestInitCommandRejectsInvalidProjectName(t *testing.T) {
	originalBootstrap := bootstrapInit
	defer func() {
		bootstrapInit = originalBootstrap
	}()

	bootstrapInit = func() error {
		return nil
	}

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"init", "../bad", "-d", t.TempDir()})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("init command returned nil error")
	}
	if !strings.Contains(err.Error(), "project name must be a folder name") {
		t.Fatalf("error = %q, want invalid project name", err)
	}
}
