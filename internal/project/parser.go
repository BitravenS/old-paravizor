package project

import (
	"fmt"
	"github.com/bitravens/paravizor/v1/internal/utils"
	"path/filepath"
)

// LoadProjectConfig reads and validates project.yaml from the given project directory.
// The file must have a top-level "project:" key (ProjectWrapper format).
func LoadProjectConfig(projectDir string) (ProjectConfig, error) {
	path := filepath.Join(projectDir, ProjectConfigFile)
	wrapper, err := utils.ParseYAML[ProjectWrapper](path)
	if err != nil {
		return ProjectConfig{}, fmt.Errorf("load project config %q: %w", path, err)
	}
	return wrapper.Project, nil
}

// WriteProjectConfig validates cfg and serializes it to project.yaml inside projectDir.
// Output is wrapped under a top-level "project:" key.
func WriteProjectConfig(projectDir string, cfg ProjectConfig) error {
	return utils.WriteYAML(filepath.Join(projectDir, ProjectConfigFile), ProjectWrapper{Project: cfg})
}
