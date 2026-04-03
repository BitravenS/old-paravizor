package project

import (
	"path/filepath"

	"github.com/bitravens/paravizor/v1/internal/utils"
)

// LoadProjectConfig reads and validates project.yaml from the given project directory.
func LoadProjectConfig(projectDir string) (ProjectConfig, error) {
	return utils.ParseYAML[ProjectConfig](filepath.Join(projectDir, ProjectConfigFile))
}

// WriteProjectConfig validates cfg and serializes it to project.yaml inside projectDir.
func WriteProjectConfig(projectDir string, cfg ProjectConfig) error {
	return utils.WriteYAML(filepath.Join(projectDir, ProjectConfigFile), cfg)
}
