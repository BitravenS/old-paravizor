package engine

import (
	"github.com/bitravens/paravizor/v1/internal/utils"
)

// ParsePipelineConfig reads and validates a PipelineConfig from a YAML file.
// The file must have a top-level "pipeline:" key (PipelineWrapper format).
func ParsePipelineConfig(path string) (PipelineConfig, error) {
	wrapper, err := utils.ParseYAML[PipelineWrapper](path)
	if err != nil {
		return PipelineConfig{}, err
	}
	return wrapper.Pipeline, nil
}

// WritePipelineConfig validates cfg and serializes it to a YAML file.
// Output is wrapped under a top-level "pipeline:" key.
func WritePipelineConfig(path string, cfg PipelineConfig) error {
	return utils.WriteYAML(path, PipelineWrapper{Pipeline: cfg})
}
