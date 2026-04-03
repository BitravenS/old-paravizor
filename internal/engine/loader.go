package engine

import (
	"os"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
)

// LoadExternalPipeline loads a PipelineConfig from ~/.config/paravizor/pipelines/<name>.yaml.
// If the file is not found or fails validation, it falls back to parsing default.yaml,
// and returns an error indicating what failed so it can be logged.
func LoadExternalPipeline(name string) (*PipelineConfig, error) {
	if name == "" {
		name = "default"
	}

	if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
		name += ".yaml"
	}

	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configDir = filepath.Join(homeDir, ".config")
		}
	}

	pipelinePath := filepath.Join(configDir, "paravizor", "pipelines", name)

	cfg, err := ParsePipelineConfig(pipelinePath)
	if err != nil {
		if name != "default.yaml" {
			log.Warn("Failed to load pipeline, falling back to default", "pipeline", name, "err", err)
			defaultPath := filepath.Join(configDir, "paravizor", "pipelines", "default.yaml")
			defCfg, defErr := ParsePipelineConfig(defaultPath)
			if defErr != nil {
				return nil, err // Return original error if fallback also fails
			}
			return &defCfg, err // Return fallback cfg, but still return error so caller knows
		}
		return nil, err
	}

	return &cfg, nil
}
