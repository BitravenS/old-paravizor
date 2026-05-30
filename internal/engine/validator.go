package engine

import (
	"errors"
	"fmt"

	"github.com/bitravens/paravizor/v1/internal/tool"
)

// validItemTypes is the canonical set of item type strings the engine handles.
var validItemTypes = map[string]bool{
	"domain":     true,
	"url":        true,
	"ip":         true,
	"port":       true,
	"dns_record": true,
	"finding":    true,
	"file":       true,
}

// ValidatePipeline performs semantic validation of a PipelineConfig beyond
// struct tags. It catches problems that would only surface at run time:
//
//   - Node stage references a stage that doesn't exist
//   - Node consumes/produces are unknown item types
//   - Route targets reference nodes that don't exist
//   - A routed node has no produces type (nothing to route)
//   - Produces/consumes mismatch along every route edge
//   - Init entries reference nodes that don't exist
//   - Init item_type mismatches the target node's consumes
//
// Returns a joined non-nil error if any check fails, nil otherwise.
func ValidatePipeline(cfg *PipelineConfig) error {
	if cfg == nil {
		return fmt.Errorf("pipeline config is nil")
	}

	var errs []error
	graphReady := true

	// Build lookup sets.
	stageIDs := make(map[int]bool, len(cfg.Stages))
	for _, s := range cfg.Stages {
		if stageIDs[s.ID] {
			errs = append(errs, fmt.Errorf("stage %d is defined more than once", s.ID))
		}
		stageIDs[s.ID] = true
	}

	nodeByID := make(map[string]*NodeConfig, len(cfg.Nodes))
	for i := range cfg.Nodes {
		n := &cfg.Nodes[i]
		if _, exists := nodeByID[n.ID]; exists {
			errs = append(errs, fmt.Errorf("node %q is defined more than once", n.ID))
			graphReady = false
			continue
		}
		nodeByID[n.ID] = n
	}

	// Validate each node.
	for _, node := range cfg.Nodes {
		// Stage reference
		if !stageIDs[node.Stage] {
			errs = append(errs, fmt.Errorf("node %q: stage %d is not defined in stages", node.ID, node.Stage))
		}

		// Item type validity
		if !validItemTypes[node.Consumes] {
			errs = append(errs, fmt.Errorf("node %q: consumes %q is not a valid item type (%s)", node.ID, node.Consumes, itemTypeList()))
		}
		if node.Produces != "" && !validItemTypes[node.Produces] {
			errs = append(errs, fmt.Errorf("node %q: produces %q is not a valid item type (%s)", node.ID, node.Produces, itemTypeList()))
		}

		// Routing
		if len(node.Routes) > 0 {
			if node.Produces == "" {
				errs = append(errs, fmt.Errorf("node %q: has routes but produces is not set — nothing to route", node.ID))
			}
			for _, route := range node.Routes {
				downstream, ok := nodeByID[route.To]
				if !ok {
					errs = append(errs, fmt.Errorf("node %q: route target %q does not exist", node.ID, route.To))
					graphReady = false
					continue
				}
				// Produces must match downstream's consumes.
				if node.Produces != "" && downstream.Consumes != node.Produces {
					errs = append(errs, fmt.Errorf("node %q → %q: type mismatch — %q produces %q but %q consumes %q",
						node.ID, route.To, node.ID, node.Produces, route.To, downstream.Consumes))
				}
			}
		}
	}

	// Validate init entries.
	for i, init := range cfg.Init {
		n, ok := nodeByID[init.Node]
		if !ok {
			errs = append(errs, fmt.Errorf("init[%d]: node %q does not exist in the pipeline", i, init.Node))
			graphReady = false
			continue
		}
		if n.Consumes != init.ItemType {
			errs = append(errs, fmt.Errorf("init[%d]: scope %q sends item_type %q to node %q which consumes %q",
				i, init.Scope, init.ItemType, init.Node, n.Consumes))
		}
	}

	if graphReady {
		if _, err := BuildDAG(cfg); err != nil {
			errs = append(errs, fmt.Errorf("pipeline graph is invalid: %w", err))
		}
	}

	return errors.Join(errs...)
}

// ValidatePipelineAgainstRegistry checks that every node's tool exists in the
// registry and that declared consumes/produces types are consistent with the
// tool's own configuration. Tool availability (binary present) is a warning,
// not an error — the engine already handles missing binaries gracefully.
//
// Returns a joined non-nil error if any structural inconsistency is found.
func ValidatePipelineAgainstRegistry(cfg *PipelineConfig, reg *tool.Registry) error {
	var errs []error

	for _, node := range cfg.Nodes {
		if node.Tool == "" {
			continue // passthrough node — no tool to check
		}

		def, ok := reg.Get(node.Tool)
		if !ok {
			errs = append(errs, fmt.Errorf("node %q: tool %q is not registered (check tools directory)", node.ID, node.Tool))
			continue
		}

		// consumes must agree
		if def.Consumes != node.Consumes {
			errs = append(errs, fmt.Errorf("node %q: tool %q consumes %q but node declares consumes %q",
				node.ID, node.Tool, def.Consumes, node.Consumes))
		}

		// produces must agree when the node declares one
		if node.Produces != "" && def.Produces != node.Produces {
			errs = append(errs, fmt.Errorf("node %q: tool %q produces %q but node declares produces %q",
				node.ID, node.Tool, def.Produces, node.Produces))
		}
	}

	return errors.Join(errs...)
}

func itemTypeList() string {
	return "domain, url, ip, port, dns_record, finding, file"
}
