package engine

// PipelineConfig represents the configuration for a recon pipeline, including its name, description, and the stages it consists of.
type PipelineConfig struct {
	Name        string        `yaml:"name" validate:"required"`
	Description string        `yaml:"description"`
	Init        []InitConfig  `yaml:"init,omitempty" validate:"omitempty,dive"`
	Stages      []StageConfig `yaml:"stages" validate:"required,dive"`
	Nodes       []NodeConfig  `yaml:"nodes" validate:"required,dive"`
}

// InitConfig defines where a scope item should enter the pipeline.
// Example scopes: exact domain, wildcard domain, URL/path target.
type InitConfig struct {
	Scope    string `yaml:"scope" validate:"required,oneof=exact wildcard path"`
	Node     string `yaml:"node" validate:"required"`
	ItemType string `yaml:"item_type" validate:"required,oneof=domain url"`
}

// StageConfig represents the configuration for a stage in the recon pipeline (e.g., subdomain enumeration, port scanning).
type StageConfig struct {
	ID   int    `yaml:"id" validate:"required"`
	Name string `yaml:"name" validate:"required"`
}

// NodeConfig represents the configuration for a node in the recon pipeline.
type NodeConfig struct {
	ID        string        `yaml:"id" validate:"required"`
	Name      string        `yaml:"name" validate:"required"`
	Stage     int           `yaml:"stage" validate:"required"`
	Tool      string        `yaml:"tool" validate:"omitempty"`
	Consumes  string        `yaml:"consumes" validate:"required"`
	Produces  string        `yaml:"produces,omitempty"`
	Batch     BatchConfig   `yaml:"batch"`
	Routes    []RouteConfig `yaml:"routes"`
	Filter    string        `yaml:"filter,omitempty" validate:"omitempty,regex"`
	FilterCfg FilterConfig  `yaml:"filter_cfg,omitempty"`
}

type FilterConfig struct {
	ExcludeExtensions []string `yaml:"exclude_extensions,omitempty"`
	IncludeExtensions []string `yaml:"include_extensions,omitempty"`
	Match             string   `yaml:"match,omitempty" validate:"omitempty,regex"`
}

type BatchConfig struct {
	Size         int    `yaml:"size,omitempty" validate:"omitempty,gte=0"`
	Timeout      string `yaml:"timeout,omitempty"`                             // Timeout duration string e.g. "10s"
	MinSize      int    `yaml:"min_size,omitempty" validate:"omitempty,gte=0"` // Minimum batch size to trigger processing
	WaitForPeers bool   `yaml:"wait_for_peers,omitempty" validate:"boolean"`   // Whether to wait for upstream nodes to produce data before processing the batch
}

type RouteConfig struct {
	To        string `yaml:"to" validate:"required"`               // ID of the downstream node to route results to
	Condition string `yaml:"condition,omitempty" validate:"regex"` // Optional condition to determine if results should be routed (e.g., based on output content)
}

type PipelineWrapper struct {
	Pipeline PipelineConfig `yaml:"pipeline"`
}
