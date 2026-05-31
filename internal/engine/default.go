package engine

import (
	"os"

	"github.com/bitravens/paravizor/v1/internal/assets"
)

// WriteDefaultPipeline writes the default recon pipeline to path if it does not
// already exist. It extracts the bundled YAML configuration.
func WriteDefaultPipeline(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // File exists
	}

	data, err := assets.ReadFile("pipelines/default.yaml")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
