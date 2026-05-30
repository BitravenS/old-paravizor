package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitravens/paravizor/v1/internal/utils"
)

func TestGetGlobalConfigPathWritesValidDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := GetGlobalConfigPath()
	if err != nil {
		t.Fatalf("GetGlobalConfigPath returned error: %v", err)
	}

	wrapper, err := utils.ParseYAML[ConfigWrapper](path)
	if err != nil {
		t.Fatalf("default config did not parse: %v", err)
	}

	cfg := wrapper.Config
	if cfg.Theme != "default" {
		t.Fatalf("Theme = %q, want default", cfg.Theme)
	}
	if cfg.DefaultPipeline != "default" {
		t.Fatalf("DefaultPipeline = %q, want default", cfg.DefaultPipeline)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func TestDocsConfigMatchesCurrentSchema(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "config.yaml")
	if _, err := utils.ParseYAML[ConfigWrapper](path); err != nil {
		t.Fatalf("docs config does not match current config schema: %v", err)
	}
}

func TestLoadConfigAppliesProjectOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PRVZR_CONFIG", "")

	projectDir := t.TempDir()
	override := []byte("paravizor:\n  log_level: debug\n  max_concurrent_processes: 3\n")
	if err := os.WriteFile(filepath.Join(projectDir, ConfigFileName), override, 0o644); err != nil {
		t.Fatalf("write project override: %v", err)
	}

	cfg := LoadConfig(projectDir)
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want debug", cfg.LogLevel)
	}
	if cfg.MaxProcesses != 3 {
		t.Fatalf("MaxProcesses = %d, want 3", cfg.MaxProcesses)
	}
	if cfg.Theme != "default" {
		t.Fatalf("Theme = %q, want default inherited from global config", cfg.Theme)
	}
}
