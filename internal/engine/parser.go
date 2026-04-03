package engine

import (
	"github.com/bitravens/paravizor/v1/internal/utils"
)

// ParsePipelineConfig reads and validates a PipelineConfig from a YAML file.
func ParsePipelineConfig(path string) (PipelineConfig, error) {
	return utils.ParseYAML[PipelineConfig](path)
}

// WritePipelineConfig validates cfg and serializes it to a YAML file.
func WritePipelineConfig(path string, cfg PipelineConfig) error {
	return utils.WriteYAML(path, cfg)
}
