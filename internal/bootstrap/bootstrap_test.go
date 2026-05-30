package bootstrap_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitravens/paravizor/v1/internal/bootstrap"
	"github.com/bitravens/paravizor/v1/internal/tool"
)

func TestInitCreatesDefaultConfigAssets(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	if err := bootstrap.Init(); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	configDir := filepath.Join(configHome, "paravizor")
	for _, path := range []string{
		filepath.Join(configDir, "config.yaml"),
		filepath.Join(configDir, "themes", "default.yaml"),
		filepath.Join(configDir, "pipelines", "default.yaml"),
	} {
		if !fileExists(path) {
			t.Fatalf("expected bootstrap file %s to exist", path)
		}
	}

	toolFiles, err := filepath.Glob(filepath.Join(configDir, "tools", "*.yaml"))
	if err != nil {
		t.Fatalf("glob tool files: %v", err)
	}
	if len(toolFiles) != len(tool.DefaultTools) {
		t.Fatalf("tool file count = %d, want %d", len(toolFiles), len(tool.DefaultTools))
	}
}

func TestInitReturnsNonFatalIssueForBadGlobalConfig(t *testing.T) {
	configHome := t.TempDir()
	configDir := filepath.Join(configHome, "paravizor")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("paravizor:\n  log_level: loud\n"), 0o644); err != nil {
		t.Fatalf("write bad config: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", configHome)

	err := bootstrap.Init()
	issues, ok := bootstrap.NonFatalIssues(err)
	if !ok {
		t.Fatalf("Init error = %v, want non-fatal issues", err)
	}
	if len(issues) == 0 || !strings.Contains(issues[0], "invalid config file") {
		t.Fatalf("issues = %#v, want invalid config issue", issues)
	}

	if !fileExists(filepath.Join(configDir, "pipelines", "default.yaml")) {
		t.Fatal("bootstrap did not continue to create default pipeline")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
