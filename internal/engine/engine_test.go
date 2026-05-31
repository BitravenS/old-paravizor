package engine

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitravens/paravizor/v1/internal/events"
	"github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/bitravens/paravizor/v1/internal/store/db"
	"github.com/bitravens/paravizor/v1/internal/tool"
)

func setupTestStore(t *testing.T) *store.Store {
	cfg := store.DBConfig{}
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(context.Background(), dbPath, cfg)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	return st
}

func TestEngineParallelExecutionAndRouting(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	cfg := &PipelineConfig{
		Stages: []StageConfig{{ID: 1, Name: "Test Stage"}},
		Nodes: []NodeConfig{
			{
				ID:       "node1",
				Stage:    1,
				Consumes: "domain",
				Produces: "domain",
				Routes:   []RouteConfig{{To: "node2"}, {To: "node3"}},
			},
			{
				ID:       "node2",
				Stage:    1,
				Consumes: "domain",
				Produces: "domain",
			},
			{
				ID:       "node3",
				Stage:    1,
				Consumes: "domain",
				Produces: "domain",
			},
		},
	}

	dag, err := BuildDAG(cfg)
	if err != nil {
		t.Fatalf("failed to build DAG: %v", err)
	}

	bus := events.NewBus()
	reg := tool.NewRegistry()
	runner := tool.NewRunner(bus, t.TempDir())
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scope := NewScopeEngine(project.ScopeConfig{Include: []string{"example.com"}})

	eng := NewEngine(dag, st, bus, reg, runner, logger, scope)

	// Seed input for node1
	ctx := context.Background()
	domainID, err := st.InsertDomain(ctx, "example.com", "seed", nil)
	if err != nil {
		t.Fatalf("failed to insert domain: %v", err)
	}
	err = st.SetPipelineState(ctx, &db.PipelineState{
		ItemType: "domain",
		ItemID:   domainID,
		NodeID:   "node1",
		Status:   "pending",
	})
	if err != nil {
		t.Fatalf("failed to set pending state: %v", err)
	}

	// Run engine
	err = eng.Run(ctx)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}

	// Verify all nodes completed
	if status := eng.GetNodeStatus("node1"); status != NodeStatusCompleted {
		t.Errorf("node1 status = %v, want completed", status)
	}
	if status := eng.GetNodeStatus("node2"); status != NodeStatusCompleted {
		t.Errorf("node2 status = %v, want completed", status)
	}
	if status := eng.GetNodeStatus("node3"); status != NodeStatusCompleted {
		t.Errorf("node3 status = %v, want completed", status)
	}

	// Verify items were routed
	stats2 := eng.GetNodeStats("node2")
	if stats2.ItemsIn != 1 {
		t.Errorf("node2 items in = %d, want 1", stats2.ItemsIn)
	}
	stats3 := eng.GetNodeStats("node3")
	if stats3.ItemsIn != 1 {
		t.Errorf("node3 items in = %d, want 1", stats3.ItemsIn)
	}
}

func TestEngineMissingToolSkip(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	cfg := &PipelineConfig{
		Stages: []StageConfig{{ID: 1, Name: "Test Stage"}},
		Nodes: []NodeConfig{
			{
				ID:       "node1",
				Stage:    1,
				Tool:     "missing_tool", // This tool doesn't exist
				Consumes: "domain",
				Produces: "domain",
			},
		},
	}

	dag, err := BuildDAG(cfg)
	if err != nil {
		t.Fatalf("failed to build DAG: %v", err)
	}

	bus := events.NewBus()
	reg := tool.NewRegistry() // Empty registry
	runner := tool.NewRunner(bus, t.TempDir())
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	eng := NewEngine(dag, st, bus, reg, runner, logger, nil)

	ctx := context.Background()
	err = eng.Run(ctx)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}

	if status := eng.GetNodeStatus("node1"); status != NodeStatusSkipped {
		t.Errorf("node1 status = %v, want skipped", status)
	}
}

func TestEngineWaitForPeers(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	cfg := &PipelineConfig{
		Stages: []StageConfig{{ID: 1, Name: "Test Stage"}},
		Nodes: []NodeConfig{
			{
				ID:       "node1",
				Stage:    1,
				Consumes: "domain",
				Produces: "domain",
				Routes:   []RouteConfig{{To: "sync_node"}},
			},
			{
				ID:       "node2",
				Stage:    1,
				Consumes: "domain",
				Produces: "domain",
				Routes:   []RouteConfig{{To: "sync_node"}},
			},
			{
				ID:       "sync_node",
				Stage:    1,
				Consumes: "domain",
				Produces: "domain",
				Batch:    BatchConfig{WaitForPeers: true},
			},
		},
	}

	dag, err := BuildDAG(cfg)
	if err != nil {
		t.Fatalf("failed to build DAG: %v", err)
	}

	bus := events.NewBus()
	reg := tool.NewRegistry()
	runner := tool.NewRunner(bus, t.TempDir())
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	eng := NewEngine(dag, st, bus, reg, runner, logger, nil)

	ctx := context.Background()

	// Seed input for node1 and node2
	domainID1, _ := st.InsertDomain(ctx, "example1.com", "seed", nil)
	st.SetPipelineState(ctx, &db.PipelineState{
		ItemType: "domain",
		ItemID:   domainID1,
		NodeID:   "node1",
		Status:   "pending",
	})

	domainID2, _ := st.InsertDomain(ctx, "example2.com", "seed", nil)
	st.SetPipelineState(ctx, &db.PipelineState{
		ItemType: "domain",
		ItemID:   domainID2,
		NodeID:   "node2",
		Status:   "pending",
	})

	err = eng.Run(ctx)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}

	stats := eng.GetNodeStats("sync_node")
	if stats.ItemsIn != 2 {
		t.Errorf("sync_node items in = %d, want 2 (wait for peers should collect both)", stats.ItemsIn)
	}
}
