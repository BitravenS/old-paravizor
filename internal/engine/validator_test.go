package engine

import (
	"strings"
	"testing"
)

func TestValidatePipelineTypeMismatch(t *testing.T) {
	cfg := &PipelineConfig{
		Stages: []StageConfig{{ID: 1, Name: "Recon"}},
		Nodes: []NodeConfig{
			{
				ID:       "node1",
				Stage:    1,
				Consumes: "domain",
				Produces: "domain",
				Routes:   []RouteConfig{{To: "node2"}},
			},
			{
				ID:       "node2",
				Stage:    1,
				Consumes: "url", // Type mismatch! node1 produces domain, but node2 consumes url.
				Produces: "url",
			},
		},
	}

	err := ValidatePipeline(cfg)
	if err == nil {
		t.Fatal("expected validation error for type mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "type mismatch") {
		t.Errorf("expected error to mention 'type mismatch', got: %v", err)
	}
}
