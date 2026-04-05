package tool

type ToolConfig struct {
	Name        string            `yaml:"name" validate:"required"`
	Binary      string            `yaml:"binary" validate:"required"`
	Description string            `yaml:"description"`
	VersionCmd  string            `yaml:"version_cmd,omitempty"`
	Install     string            `yaml:"install,omitempty"`
	Input       InputConfig       `yaml:"input" validate:"required"`
	Output      OutputConfig      `yaml:"output" validate:"required"`
	Flags       []string          `yaml:"flags,omitempty"`
	UserFlags   []string          `yaml:"user_flags,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
	RateLimit   RateLimitFlag     `yaml:"rate_limit,omitempty"`
	Timeout     TimeoutConfig     `yaml:"timeout,omitempty"`
	Consumes    string            `yaml:"consumes" validate:"required"`
	Produces    string            `yaml:"produces" validate:"required"`
	ScopeFlags  ScopeFlagConfig   `yaml:"scope_flags,omitempty"`

	// Runtime state — not serialised from YAML.
	Available  bool   `yaml:"-"`
	BinaryPath string `yaml:"-"`
}

type InputConfig struct {
	Type string    `yaml:"type" validate:"required,oneof=arg stdin file none"`
	Flag string    `yaml:"flag,omitempty" validate:"required_if=Type file"`
	Bulk BulkInput `yaml:"bulk,omitempty"`
}

type BulkInput struct {
	Type      string `yaml:"type,omitempty" validate:"omitempty,oneof=file stdin"`
	Flag      string `yaml:"flag,omitempty"`
	Separator string `yaml:"separator,omitempty"`
}

type OutputConfig struct {
	Type    string            `yaml:"type" validate:"required,oneof=stdout file directory"`
	Format  string            `yaml:"format" validate:"required,oneof=line json jsonl csv regex xml"`
	Path    string            `yaml:"path,omitempty"`
	Flag    string            `yaml:"flag,omitempty" validate:"required_if=Type file required_if=Type directory"`
	Pattern string            `yaml:"pattern,omitempty" validate:"required_if=Format regex,regex"`
	Fields  map[string]string `yaml:"fields,omitempty"`
}

type RateLimitFlag struct {
	Flag string `yaml:"flag,omitempty"`
	Unit string `yaml:"unit,omitempty" validate:"omitempty,oneof=second minute hour"`
}

type TimeoutConfig struct {
	Flag    string `yaml:"flag,omitempty"`
	Default int    `yaml:"default,omitempty" validate:"gte=0"`
}

// ScopeFlagConfig holds the tool's CLI flags for scope injection.
type ScopeFlagConfig struct {
	Include string `yaml:"include,omitempty"`
	Exclude string `yaml:"exclude,omitempty"`
}

type ToolWrapper struct {
	Tool ToolConfig `yaml:"tool"`
}
