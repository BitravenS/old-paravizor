package engine

// PipelineConfig represents the configuration for a recon pipeline, including its name, description, and the stages it consists of.
type PipelineConfig struct {
	Name        string        `yaml:"name" validate:"required"`
	Description string        `yaml:"description"`
	Stages      []StageConfig `yaml:"stages" validate:"required,dive"`
	Nodes       []NodeConfig  `yaml:"nodes" validate:"required,dive"`
}

// StageConfig represents the configuration for a stage in the recon pipeline (e.g., subdomain enumeration, port scanning).
type StageConfig struct {
	ID   int    `yaml:"id" validate:"required"`
	Name string `yaml:"name" validate:"required"`
}

// NodeConfig represents the configuration for a node in the recon pipeline.
type NodeConfig struct {
	ID       string        `yaml:"id" validate:"required"`
	Name     string        `yaml:"name" validate:"required"`
	Stage    int           `yaml:"stage" validate:"required"`
	Tool     string        `yaml:"tool" validate:"required"`
	Consumes string        `yaml:"consumes" validate:"required"`
	Produces string        `yaml:"produces,omitempty"`
	Batch    BatchConfig   `yaml:"batch"`
	Routes   []RouteConfig `yaml:"routes"`
	Filter   string        `yaml:"filter,omitempty" validate:"regex"` // Optional filter (regex) to apply to the node's output before routing to downstream nodes
}

type BatchConfig struct {
	Timeout      int  `yaml:"timeout,omitempty"`        // Timeout in seconds before processing the batch
	MinSize      int  `yaml:"min_size,omitempty"`       // Minimum batch size to trigger processing
	WaitForPeers bool `yaml:"wait_for_peers,omitempty"` // Whether to wait for upstream nodes to produce data before processing the batch
}

type RouteConfig struct {
	To        string `yaml:"to" validate:"required"`               // ID of the downstream node to route results to
	Condition string `yaml:"condition,omitempty" validate:"regex"` // Optional condition to determine if results should be routed (e.g., based on output content)
}
