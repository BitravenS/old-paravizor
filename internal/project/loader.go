package project

import (
	"fmt"
	"os"
	"path/filepath"
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
//	    └── paravizor.db   (empty; Store.Open + Migrate will apply schema)
//
// Returns the absolute path of the created project directory.
// Returns an error if the directory already exists.
func InitProject(targetDir string, cfg ProjectConfig) (string, error) {
	dir := Dir(targetDir, cfg.Name)
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

	dbPath := filepath.Join(dir, ProjectDBFile)
	f, err := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("create db file: %w", err)
	}
	f.Close()

	return dir, nil
}

// LoadProject reads the project.yaml from projectDir and returns the parsed config.
func LoadProject(projectDir string) (ProjectConfig, error) {
	return LoadProjectConfig(projectDir)
}
