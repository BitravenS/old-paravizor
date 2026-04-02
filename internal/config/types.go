package config

type ProjectConfig struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	Scope         ScopeConfig       `yaml:"scope"`
	RateLimitMode string            `yaml:"rate_limit_mode" validate:"required,oneof=normal overdrive"`
	RateLimit     []RateLimitConfig `yaml:"rate_limit"`
	Pipeline      string            `yaml:"pipeline"`
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
