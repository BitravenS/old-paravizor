package config

import (
	"github.com/bitravens/paravizor/v1/internal/ai"
	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/store"
	"github.com/knadh/koanf/v2"
)

const PrvzrDir = "aravizor"
const ConfigFileName = "config.yaml"
const DEFAULT_XDG_CONFIG_DIRNAME = ".config"

var conf = koanf.Conf{
	Delim:       ".",
	StrictMerge: true,
}

type configError struct {
	configDir string
	err       error
}

type ConfigParser struct {
	k *koanf.Koanf
}

type Config struct {
	APIKeys             map[string]string      `yaml:"api_keys"`
	Theme               *ThemeConfig           `yaml:"theme,omitempty"`
	DefaultPipeline     *engine.PipelineConfig `yaml:"default_pipeline,omitempty"`
	MaxProcesses        int                    `yaml:"max_concurrent_processes,omitempty" validate:"omitempty,gte=1"`
	HealthCheckInterval int                    `yaml:"process_healthcheck_interval,omitempty" validate:"omitempty,gte=1"`
	DBConfig            *store.DBConfig        `yaml:"db,omitempty"`
	LogLevel            string                 `yaml:"log_level,omitempty" validate:"omitempty,oneof=debug info warn error"`
	AIConfig            *ai.AIConfig           `yaml:"ai,omitempty"`
}
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
