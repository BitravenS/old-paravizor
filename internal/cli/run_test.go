package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bitravens/paravizor/v1/internal/project"
)

func TestRunCommandOpensProjectInTUI(t *testing.T) {
	originalBootstrap := bootstrapInit
	originalStartTUI := startTUI
	defer func() {
		bootstrapInit = originalBootstrap
		startTUI = originalStartTUI
	}()

	bootstraps := 0
	bootstrapInit = func() error {
		bootstraps++
		return nil
	}

	var tuiLocation string
	startTUI = func(cmd *cobra.Command, location string) error {
		tuiLocation = location
		return nil
	}

	cfg, err := project.CreateProject("demo", "Demo project", "", "", "", nil, project.ScopeConfig{})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	projectDir, err := project.InitProject(t.TempDir(), *cfg)
	if err != nil {
		t.Fatalf("InitProject returned error: %v", err)
	}

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"run", "-d", projectDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run command returned error: %v", err)
	}
	if bootstraps != 1 {
		t.Fatalf("run command bootstrapped %d times, want 1", bootstraps)
	}
	if tuiLocation != projectDir {
		t.Fatalf("TUI location = %q, want %q", tuiLocation, projectDir)
	}
}

func TestRunCommandRejectsMissingProject(t *testing.T) {
	originalBootstrap := bootstrapInit
	originalStartTUI := startTUI
	defer func() {
		bootstrapInit = originalBootstrap
		startTUI = originalStartTUI
	}()

	bootstrapInit = func() error {
		return nil
	}
	startTUI = func(cmd *cobra.Command, location string) error {
		t.Fatal("startTUI should not be called for a missing project")
		return nil
	}

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"run", "-d", filepath.Join(t.TempDir(), "missing")})

	if err := cmd.Execute(); err == nil {
		t.Fatal("run command returned nil error")
	}
}
