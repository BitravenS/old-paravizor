package engine

import (
	"fmt"

	"github.com/bitravens/paravizor/v1/internal/utils"
)

// ParsePipelineConfig reads, struct-validates, and semantically validates a
// PipelineConfig from a YAML file.
// The file must have a top-level "pipeline:" key (PipelineWrapper format).
func ParsePipelineConfig(path string) (PipelineConfig, error) {
	wrapper, err := utils.ParseYAML[PipelineWrapper](path)
	if err != nil {
		return PipelineConfig{}, err
	}
	cfg := wrapper.Pipeline
	if err := ValidatePipeline(&cfg); err != nil {
		return PipelineConfig{}, fmt.Errorf("pipeline %q failed validation: %w", path, err)
	}
	return cfg, nil
}

// WritePipelineConfig validates cfg and serializes it to a YAML file.
// Output is wrapped under a top-level "pipeline:" key.
func WritePipelineConfig(path string, cfg PipelineConfig) error {
	return utils.WriteYAML(path, PipelineWrapper{Pipeline: cfg})
}
