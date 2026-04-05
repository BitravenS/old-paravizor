package engine

import (
	"os"

	"github.com/bitravens/paravizor/v1/internal/utils"
)

func WriteDefaultPipeline(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	cfg := PipelineConfig{
		Name:        "default",
		Description: "Default Paravizor Recon Pipeline",
		Init: []InitConfig{
			{Scope: "wildcard", Node: "subfinder", ItemType: "domain"},
		},
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
				Produces: "domain",
			},
		},
	}

	return utils.WriteYAML(path, PipelineWrapper{Pipeline: cfg})
}
