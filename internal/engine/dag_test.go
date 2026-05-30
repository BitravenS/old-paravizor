package engine

import (
	"reflect"
	"testing"
)

func TestBuildDAGReturnsTopologyRootsAndTerminals(t *testing.T) {
	cfg := validPipeline()

	dag, err := BuildDAG(&cfg)
	if err != nil {
		t.Fatalf("BuildDAG returned error: %v", err)
	}

	if want := []string{"seed", "probe", "archive"}; !reflect.DeepEqual(dag.Order, want) {
		t.Fatalf("Order = %#v, want %#v", dag.Order, want)
	}
	if want := []string{"seed"}; !reflect.DeepEqual(dag.RootNodes(), want) {
		t.Fatalf("RootNodes = %#v, want %#v", dag.RootNodes(), want)
	}
	if want := []string{"archive"}; !reflect.DeepEqual(dag.TerminalNodes(), want) {
		t.Fatalf("TerminalNodes = %#v, want %#v", dag.TerminalNodes(), want)
	}
	if got := dag.UpstreamCount("archive"); got != 1 {
		t.Fatalf("UpstreamCount(archive) = %d, want 1", got)
	}
	if got := dag.Weights["seed"]; got != 3 {
		t.Fatalf("Weights[seed] = %d, want 3", got)
	}
}
