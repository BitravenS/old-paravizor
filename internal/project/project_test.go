package project

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCreateProjectFillsDefaults(t *testing.T) {
	cfg, err := CreateProject("demo", "Demo project", "", "", "", nil, ScopeConfig{
		Include: []string{"example.com"},
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	if cfg.Name != "demo" {
		t.Fatalf("Name = %q, want demo", cfg.Name)
	}
	if cfg.Pipeline != "default" {
		t.Fatalf("Pipeline = %q, want default", cfg.Pipeline)
	}
	if cfg.RateLimitMode != "normal" {
		t.Fatalf("RateLimitMode = %q, want normal", cfg.RateLimitMode)
	}
}

func TestValidateProjectNameRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{
		"",
		"   ",
		".",
		"..",
		"../bad",
		`..\bad`,
		"nested/project",
		`nested\project`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateProjectName(name); err == nil {
				t.Fatalf("ValidateProjectName(%q) returned nil error", name)
			}
		})
	}
}

func TestInitProjectCreatesLoadableMigratedProject(t *testing.T) {
	scope := ScopeConfig{
		Include: []string{"example.com"},
		Exclude: []string{"dev.example.com"},
	}
	cfg, err := CreateProject("demo", "Demo project", "", "", "", nil, scope)
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	projectDir, err := InitProject(t.TempDir(), *cfg)
	if err != nil {
		t.Fatalf("InitProject returned error: %v", err)
	}
	if !filepath.IsAbs(projectDir) {
		t.Fatalf("InitProject returned non-absolute path %q", projectDir)
	}
	if !fileExists(filepath.Join(projectDir, ProjectConfigFile)) {
		t.Fatalf("missing %s", ProjectConfigFile)
	}
	if !fileExists(DBPath(projectDir)) {
		t.Fatalf("missing %s", ProjectDBFile)
	}

	loaded, err := LoadProject(projectDir)
	if err != nil {
		t.Fatalf("LoadProject returned error: %v", err)
	}
	if loaded.Name != cfg.Name ||
		loaded.Description != cfg.Description ||
		loaded.RateLimitMode != cfg.RateLimitMode ||
		loaded.Pipeline != cfg.Pipeline ||
		!reflect.DeepEqual(loaded.Scope, cfg.Scope) ||
		len(loaded.RateLimit) != 0 {
		t.Fatalf("loaded config = %#v, want equivalent to %#v", loaded, *cfg)
	}

	assertMigratedDB(t, DBPath(projectDir))
}

func TestInitProjectExistingProjectFails(t *testing.T) {
	cfg, err := CreateProject("demo", "Demo project", "", "", "", nil, ScopeConfig{})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	root := t.TempDir()
	if _, err := InitProject(root, *cfg); err != nil {
		t.Fatalf("first InitProject returned error: %v", err)
	}

	_, err = InitProject(root, *cfg)
	if err == nil {
		t.Fatal("second InitProject returned nil error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %q, want already exists", err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func assertMigratedDB(t *testing.T, dbPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	var name string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'scope_rules'`).Scan(&name)
	if err != nil {
		t.Fatalf("project database was not migrated: %v", err)
	}
	if name != "scope_rules" {
		t.Fatalf("schema table = %q, want scope_rules", name)
	}
}
