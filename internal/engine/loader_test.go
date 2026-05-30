package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadExternalPipelineLoadsNamedPipeline(t *testing.T) {
	configHome, pipelinesDir := makePipelineConfigHome(t)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg := validPipeline()
	cfg.Name = "custom"
	if err := WritePipelineConfig(filepath.Join(pipelinesDir, "custom.yaml"), cfg); err != nil {
		t.Fatalf("write custom pipeline: %v", err)
	}

	loaded, err := LoadExternalPipeline("custom")
	if err != nil {
		t.Fatalf("LoadExternalPipeline returned error: %v", err)
	}
	if loaded.Name != "custom" {
		t.Fatalf("loaded pipeline name = %q, want custom", loaded.Name)
	}
}

func TestLoadExternalPipelineFallsBackToDefault(t *testing.T) {
	configHome, pipelinesDir := makePipelineConfigHome(t)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg := validPipeline()
	cfg.Name = "default"
	if err := WritePipelineConfig(filepath.Join(pipelinesDir, "default.yaml"), cfg); err != nil {
		t.Fatalf("write default pipeline: %v", err)
	}

	loaded, err := LoadExternalPipeline("missing")
	if err == nil {
		t.Fatal("LoadExternalPipeline returned nil error, want warning error for fallback")
	}
	if !strings.Contains(err.Error(), "using default pipeline") {
		t.Fatalf("fallback error = %q, want using default pipeline", err)
	}
	if loaded == nil || loaded.Name != "default" {
		t.Fatalf("loaded pipeline = %#v, want default fallback", loaded)
	}
}

func makePipelineConfigHome(t *testing.T) (configHome string, pipelinesDir string) {
	t.Helper()

	configHome = t.TempDir()
	pipelinesDir = filepath.Join(configHome, "paravizor", "pipelines")
	if err := os.MkdirAll(pipelinesDir, 0o755); err != nil {
		t.Fatalf("create pipelines dir: %v", err)
	}
	return configHome, pipelinesDir
}
