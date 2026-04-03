package tool

type ToolConfig struct {
	Name         string          `yaml:"name" validate:"required"`              // Unique identifier for the tool, referenced by pipeline nodes
	Description  string          `yaml:"description" validate:"required"`       // Description of the tool's functionality
	Binary       string          `yaml:"binary" validate:"required"`            // Executable name or full path for this tool
	InstallHints []string        `yaml:"install,omitempty" validate:"required"` // Installation hints to display when the tool is missing
	Input        InputConfig     `yaml:"input" validate:"required"`             // Configuration for how the tool consumes input
	Output       OutputConfig    `yaml:"output" validate:"required"`            // Configuration for how the tool produces output
	Flags        []string        `yaml:"flags,omitempty"`                       // Additional flags to pass when executing the tool
	Timeout      TimeoutConfig   `yaml:"timeout,omitempty"`                     // Optional timeout configuration for the tool execution
	RateLimit    RateLimitConfig `yaml:"rate_limit,omitempty"`                  // Optional rate limit configuration for the tool execution
	Consumes     string          `yaml:"consumes" validate:"required"`          // Item type consumed by the tool
	Produces     string          `yaml:"produces" validate:"required"`          // Item type produced by the tool
	Scope        ScopeConfig     `yaml:"scope,omitempty"`                       // Optional scope configuration flags for the tool
}

type InputConfig struct {
	Type string `yaml:"type" validate:"required,oneof=stdin file arg none"`
	Flag string `yaml:"flag,omitempty" validate:"required_if=Type file"`
}

type OutputConfig struct {
	Type    string            `yaml:"type" validate:"required,oneof=stdout file directory"`
	Format  string            `yaml:"format" validate:"required,oneof=line json jsonl csv regex xml"`
	Flag    string            `yaml:"flag,omitempty" validate:"required_if=Type file required_if=Type directory"`
	Pattern string            `yaml:"pattern,omitempty" validate:"required_if=Format regex,regex"`
	Fields  map[string]string `yaml:"fields,omitempty"`
}

type TimeoutConfig struct {
	Flag    string `yaml:"flag" validate:"required"`
	Default int    `yaml:"default,omitempty" validate:"gte=0"`
}

type RateLimitConfig struct {
	Flag string `yaml:"flag" validate:"required"`
	Unit string `yaml:"unit,omitempty" validate:"oneof=second minute hour"`
}

type ScopeConfig struct {
	IncludeFlag string `yaml:"include,omitempty"`
	ExcludeFlag string `yaml:"exclude,omitempty"`
	Separator   string `yaml:"separator"`
}
