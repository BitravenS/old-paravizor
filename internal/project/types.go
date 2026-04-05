package project

type ProjectConfig struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description,omitempty"`
	Scope         ScopeConfig       `yaml:"scope,omitempty"`
	RateLimitMode string            `yaml:"rate_limit_mode,omitempty" validate:"oneof=normal overdrive"`
	RateLimit     []RateLimitConfig `yaml:"rate_limit" validate:"dive"`
	Pipeline      string            `yaml:"pipeline,omitempty"`
}

type ScopeConfig struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude,omitempty"`
}

type RateLimitConfig struct {
	Nodes                  []string `yaml:"nodes" validate:"required"`
	Budget                 int      `yaml:"budget" validate:"required,gte=1"`
	BurstReservePercentage int      `yaml:"burst_reserve_percentage" validate:"required,gte=0,lte=100"`
	BurstReserveMin        int      `yaml:"burst_reserve_min" validate:"required,gte=0"`
}

type ProjectWrapper struct {
	Project ProjectConfig `yaml:"project"`
}
