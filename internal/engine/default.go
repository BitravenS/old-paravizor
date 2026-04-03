package engine

import (
	"github.com/bitravens/paravizor/v1/internal/utils"
	"os"
)

func WriteDefaultPipeline(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	cfg := PipelineConfig{
		Name:        "default",
		Description: "Default Paravizor Recon Pipeline",
		Stages: []StageConfig{
			{ID: 1, Name: "Discovery"},
		},
		Nodes: []NodeConfig{
			{
				ID:       "subfinder",
				Name:     "Subfinder",
				Stage:    1,
				Tool:     "subfinder",
				Consumes: "domain",
				Produces: "subdomain",
			},
		},
	}

	return utils.WriteYAML(path, cfg)
}
