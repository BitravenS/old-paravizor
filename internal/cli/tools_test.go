package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolsListPrintsConfiguredTools(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))

	cmd := newRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"tools", "list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("tools list returned error: %v\nstderr:\n%s", err, errOut.String())
	}

	stdout := out.String()
	for _, want := range []string{
		"Tools directory:",
		"STATUS",
		"subfinder",
		"Tool definitions",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("tools list output missing %q:\n%s", want, stdout)
		}
	}
}

func TestToolsShowPrintsToolDetails(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))

	cmd := newRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"tools", "show", "subfinder"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("tools show returned error: %v\nstderr:\n%s", err, errOut.String())
	}

	stdout := out.String()
	for _, want := range []string{
		"Name: subfinder",
		"Binary: subfinder",
		"Consumes: domain",
		"Produces: domain",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("tools show output missing %q:\n%s", want, stdout)
		}
	}
}

func TestToolsShowRejectsUnknownTool(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"tools", "show", "missing-tool"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("tools show returned nil error")
	}
	if !strings.Contains(err.Error(), `tool "missing-tool" is not configured`) {
		t.Fatalf("error = %q, want missing tool error", err)
	}
}
