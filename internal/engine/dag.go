package engine

import (
	"fmt"
)

// DAG represents the pipeline as a directed acyclic graph.
type DAG struct {
	Nodes        map[string]*NodeConfig
	Edges        map[string][]string // node_id -> list of downstream node_ids
	ReverseEdges map[string][]string // node_id -> list of upstream node_ids
	Stages       []StageConfig
	Order        []string       // topological order
	Weights      map[string]int // dependency weights for rate limiting
}

// BuildDAG constructs a DAG from a pipeline definition.
func BuildDAG(pipelineCfg *PipelineConfig) (*DAG, error) {
	dag := &DAG{
		Nodes:        make(map[string]*NodeConfig),
		Edges:        make(map[string][]string),
		ReverseEdges: make(map[string][]string),
		Stages:       pipelineCfg.Stages,
		Weights:      make(map[string]int),
	}

	// Register all nodes
	for i := range pipelineCfg.Nodes {
		node := &pipelineCfg.Nodes[i]
		if _, exists := dag.Nodes[node.ID]; exists {
			return nil, fmt.Errorf("duplicate node ID: %s", node.ID)
		}
		dag.Nodes[node.ID] = node
	}

	// Build edges from routes
	for _, node := range pipelineCfg.Nodes {
		for _, route := range node.Routes {
			if _, exists := dag.Nodes[route.To]; !exists {
				return nil, fmt.Errorf("node %s routes to unknown node %s", node.ID, route.To)
			}
			dag.Edges[node.ID] = append(dag.Edges[node.ID], route.To)
			dag.ReverseEdges[route.To] = append(dag.ReverseEdges[route.To], node.ID)
		}
	}

	// Topological sort
	order, err := dag.topologicalSort()
	if err != nil {
		return nil, err
	}
	dag.Order = order

	// Compute weights
	dag.computeWeights()

	return dag, nil
}

// topologicalSort performs a topological sort using Kahn's algorithm.
func (d *DAG) topologicalSort() ([]string, error) {
	inDegree := make(map[string]int)
	for id := range d.Nodes {
		inDegree[id] = 0
	}
	for _, targets := range d.Edges {
		for _, target := range targets {
			inDegree[target]++
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, target := range d.Edges[node] {
			inDegree[target]--
			if inDegree[target] == 0 {
				queue = append(queue, target)
			}
		}
	}

	if len(order) != len(d.Nodes) {
		return nil, fmt.Errorf("pipeline DAG contains a cycle")
	}

	return order, nil
}

// computeWeights computes the downstream dependency weight for each node.
// weight(node) = 1 + sum(weight(child) for child in direct_dependents)
// Computed in reverse topological order.
func (d *DAG) computeWeights() {
	// Initialize all weights to 1
	for id := range d.Nodes {
		d.Weights[id] = 1
	}

	// Process in reverse topological order
	for i := len(d.Order) - 1; i >= 0; i-- {
		nodeID := d.Order[i]
		for _, child := range d.Edges[nodeID] {
			d.Weights[nodeID] += d.Weights[child]
		}
	}
}

// RootNodes returns nodes with no incoming edges (entry points).
func (d *DAG) RootNodes() []string {
	var roots []string
	for id := range d.Nodes {
		if len(d.ReverseEdges[id]) == 0 {
			roots = append(roots, id)
		}
	}
	return roots
}

// TerminalNodes returns nodes with no outgoing edges (exit points).
func (d *DAG) TerminalNodes() []string {
	var terminals []string
	for id := range d.Nodes {
		if len(d.Edges[id]) == 0 {
			terminals = append(terminals, id)
		}
	}
	return terminals
}

// UpstreamCount returns the number of direct upstream nodes for a given node.
func (d *DAG) UpstreamCount(nodeID string) int {
	return len(d.ReverseEdges[nodeID])
}
