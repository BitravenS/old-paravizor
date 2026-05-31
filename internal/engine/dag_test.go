package engine

import (
	"testing"
)

func TestBuildDAG(t *testing.T) {
	cfg := &PipelineConfig{
		Nodes: []NodeConfig{
			{ID: "node1", Routes: []RouteConfig{{To: "node2"}}},
			{ID: "node2", Routes: []RouteConfig{{To: "node3"}}},
			{ID: "node3"},
		},
	}

	dag, err := BuildDAG(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(dag.Order) != 3 {
		t.Errorf("expected 3 nodes in order, got %d", len(dag.Order))
	}
	if dag.Order[0] != "node1" || dag.Order[2] != "node3" {
		t.Errorf("unexpected order: %v", dag.Order)
	}
}

func TestDAGCycleDetection(t *testing.T) {
	cfg := &PipelineConfig{
		Nodes: []NodeConfig{
			{ID: "node1", Routes: []RouteConfig{{To: "node2"}}},
			{ID: "node2", Routes: []RouteConfig{{To: "node3"}}},
			{ID: "node3", Routes: []RouteConfig{{To: "node1"}}},
		},
	}

	_, err := BuildDAG(cfg)
	if err == nil {
		t.Fatal("expected error for cyclic DAG, got nil")
	}
}

func TestDAGRouteValidation(t *testing.T) {
	cfg := &PipelineConfig{
		Nodes: []NodeConfig{
			{ID: "node1", Routes: []RouteConfig{{To: "non_existent_node"}}},
		},
	}

	_, err := BuildDAG(cfg)
	if err == nil {
		t.Fatal("expected error for non-existent route target, got nil")
	}
}
