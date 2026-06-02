package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/bitravens/paravizor/v1/internal/store/db"
)

func TestQueryCommandRunsReadOnlySQL(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	projectDir := createCLIProject(t)

	cmd := newRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{
		"query",
		"-d",
		projectDir,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'domains'`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("query command returned error: %v\nstderr:\n%s", err, errOut.String())
	}
	if !strings.Contains(out.String(), "domains") {
		t.Fatalf("query output = %q, want domains table", out.String())
	}
}

func TestExportArtifactsWritesProjectData(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	projectDir := createCLIProject(t)
	seedExportData(t, projectDir)

	cmd := newRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"export", "artifacts", "-d", projectDir, "-o", "exports/test"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export artifacts returned error: %v\nstderr:\n%s", err, errOut.String())
	}

	outputDir := filepath.Join(projectDir, "exports", "test")
	assertFileContains(t, filepath.Join(outputDir, "subdomains.txt"), "example.com")
	assertFileContains(t, filepath.Join(outputDir, "urls.txt"), "https://example.com/login")
	assertFileContains(t, filepath.Join(outputDir, "findings", "high.txt"), "Example finding")
}

func createCLIProject(t *testing.T) string {
	t.Helper()

	cfg, err := project.CreateProject("demo", "Demo project", "", "", "", nil, project.ScopeConfig{})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	projectDir, err := project.InitProject(t.TempDir(), *cfg)
	if err != nil {
		t.Fatalf("InitProject returned error: %v", err)
	}
	return projectDir
}

func seedExportData(t *testing.T, projectDir string) {
	t.Helper()

	st, err := store.Open(context.Background(), project.DBPath(projectDir), store.DBConfig{})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	if _, err := st.InsertDomain(context.Background(), "example.com", "test", nil); err != nil {
		t.Fatalf("insert domain: %v", err)
	}
	if _, err := st.InsertURL(context.Background(), "https://example.com/login", "test", nil, nil); err != nil {
		t.Fatalf("insert url: %v", err)
	}
	severity := "high"
	if _, err := st.InsertFinding(context.Background(), &db.Finding{
		Scanner:  "test",
		Severity: &severity,
		Title:    "Example finding",
	}); err != nil {
		t.Fatalf("insert finding: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s = %q, want %q", path, string(data), want)
	}
}
