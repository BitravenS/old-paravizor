package engine

import (
	"strings"
	"testing"
)

func TestValidatePipelineAcceptsValidPipeline(t *testing.T) {
	cfg := validPipeline()

	if err := ValidatePipeline(&cfg); err != nil {
		t.Fatalf("ValidatePipeline returned error: %v", err)
	}
}

func TestValidatePipelineRejectsDuplicateStageID(t *testing.T) {
	cfg := validPipeline()
	cfg.Stages = append(cfg.Stages, StageConfig{ID: 1, Name: "Duplicate"})

	err := ValidatePipeline(&cfg)
	assertErrorContains(t, err, "stage 1 is defined more than once")
}

func TestValidatePipelineRejectsDuplicateNodeID(t *testing.T) {
	cfg := validPipeline()
	cfg.Nodes = append(cfg.Nodes, cfg.Nodes[0])

	err := ValidatePipeline(&cfg)
	assertErrorContains(t, err, `node "seed" is defined more than once`)
}

func TestValidatePipelineRejectsMissingStage(t *testing.T) {
	cfg := validPipeline()
	cfg.Nodes[0].Stage = 99

	err := ValidatePipeline(&cfg)
	assertErrorContains(t, err, `node "seed": stage 99 is not defined`)
}

func TestValidatePipelineRejectsMissingRouteTarget(t *testing.T) {
	cfg := validPipeline()
	cfg.Nodes[0].Routes = []RouteConfig{{To: "missing"}}

	err := ValidatePipeline(&cfg)
	assertErrorContains(t, err, `route target "missing" does not exist`)
}

func TestValidatePipelineRejectsInvalidItemType(t *testing.T) {
	cfg := validPipeline()
	cfg.Nodes[0].Consumes = "unknown"

	err := ValidatePipeline(&cfg)
	assertErrorContains(t, err, `consumes "unknown" is not a valid item type`)
}

func TestValidatePipelineRejectsInvalidInitNode(t *testing.T) {
	cfg := validPipeline()
	cfg.Init[0].Node = "missing"

	err := ValidatePipeline(&cfg)
	assertErrorContains(t, err, `init[0]: node "missing" does not exist`)
}

func TestValidatePipelineRejectsCycles(t *testing.T) {
	cfg := validPipeline()
	cfg.Nodes[2].Routes = []RouteConfig{{To: "seed"}}

	err := ValidatePipeline(&cfg)
	assertErrorContains(t, err, "pipeline DAG contains a cycle")
}

func validPipeline() PipelineConfig {
	return PipelineConfig{
		Name: "valid",
		Init: []InitConfig{
			{Scope: "exact", Node: "seed", ItemType: "domain"},
		},
		Stages: []StageConfig{
			{ID: 0, Name: "Input"},
			{ID: 1, Name: "Process"},
		},
		Nodes: []NodeConfig{
			{
				ID:       "seed",
				Name:     "Seed",
				Stage:    0,
				Consumes: "domain",
				Produces: "domain",
				Routes:   []RouteConfig{{To: "probe"}},
			},
			{
				ID:       "probe",
				Name:     "Probe",
				Stage:    1,
				Consumes: "domain",
				Produces: "url",
				Routes:   []RouteConfig{{To: "archive"}},
			},
			{
				ID:       "archive",
				Name:     "Archive",
				Stage:    1,
				Consumes: "url",
				Produces: "url",
			},
		},
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("error = nil, want substring %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want substring %q", err, want)
	}
}
