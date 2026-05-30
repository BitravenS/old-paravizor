package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitravens/paravizor/v1/internal/store"
)

const (
	ProjectConfigFile   = "project.yaml"
	ProjectDBFile       = "paravizor.db"
	ProjectOverrideFile = "config.yaml"
)

// Dir returns the absolute path of a project directory rooted at targetDir.
func Dir(targetDir, projectName string) string {
	return filepath.Join(targetDir, projectName)
}

// DBPath returns the absolute path to the SQLite database for the project.
func DBPath(projectDir string) string {
	return filepath.Join(projectDir, ProjectDBFile)
}

// CreateProject constructs a ProjectConfig with sensible defaults.
func CreateProject(name, description, dir, pipeline, rlmode string, rl []RateLimitConfig, scope ScopeConfig) (*ProjectConfig, error) {
	name = strings.TrimSpace(name)
	if err := ValidateProjectName(name); err != nil {
		return nil, err
	}
	if rlmode == "" {
		rlmode = "normal"
	}
	if pipeline == "" {
		pipeline = "default"
	}
	cfg := ProjectConfig{
		Name:          name,
		Description:   description,
		Scope:         scope,
		RateLimitMode: rlmode,
		RateLimit:     rl,
		Pipeline:      pipeline,
	}
	return &cfg, nil
}

// InitProject creates the project directory structure on disk:
//
//	targetDir/
//	└── projectName/
//	    ├── project.yaml   (written from cfg)
//	    └── paravizor.db   (initialized with the project DB schema)
//
// Returns the absolute path of the created project directory.
// Returns an error if the directory already exists.
func InitProject(targetDir string, cfg ProjectConfig) (string, error) {
	cfg.Name = strings.TrimSpace(cfg.Name)
	if err := ValidateProjectName(cfg.Name); err != nil {
		return "", err
	}
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return "", fmt.Errorf("resolve target dir: %w", err)
	}

	dir := Dir(absTargetDir, cfg.Name)
	if _, err := os.Stat(dir); err == nil {
		return "", fmt.Errorf("project %q already exists at %s", cfg.Name, dir)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create project dir: %w", err)
	}

	if err := WriteProjectConfig(dir, cfg); err != nil {
		os.RemoveAll(dir)
		return "", err
	}

	if err := initProjectDB(filepath.Join(dir, ProjectDBFile)); err != nil {
		os.RemoveAll(dir)
		return "", err
	}

	return dir, nil
}

// LoadProject reads the project.yaml from projectDir and returns the parsed config.
func LoadProject(projectDir string) (ProjectConfig, error) {
	return LoadProjectConfig(projectDir)
}

func ValidateProjectName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("project name is required")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("project name %q is not allowed", name)
	}
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("project name must be a folder name, not a path: %q", name)
	}
	return nil
}

func initProjectDB(dbPath string) error {
	s, err := store.Open(context.Background(), dbPath, store.DBConfig{})
	if err != nil {
		return fmt.Errorf("initialize project database: %w", err)
	}
	if err := s.Close(); err != nil {
		return fmt.Errorf("close project database: %w", err)
	}
	return nil
}
