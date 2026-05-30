package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
	"github.com/bitravens/paravizor/v1/internal/utils"
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

	prvzrDir, err := utils.PrvzrConfigDir()
	if err != nil {
		return nil, err
	}

	pipelinePath := filepath.Join(prvzrDir, "pipelines", name)

	cfg, err := ParsePipelineConfig(pipelinePath)
	if err != nil {
		if name != "default.yaml" {
			log.Warn("Failed to load pipeline, falling back to default", "pipeline", name, "err", err)
			defaultPath := filepath.Join(prvzrDir, "pipelines", "default.yaml")
			defCfg, defErr := ParsePipelineConfig(defaultPath)
			if defErr != nil {
				return nil, fmt.Errorf("load pipeline %q: %w; fallback default pipeline %q also failed: %v", name, err, defaultPath, defErr)
			}
			return &defCfg, fmt.Errorf("load pipeline %q: %w; using default pipeline", name, err)
		}
		return nil, err
	}

	return &cfg, nil
}
