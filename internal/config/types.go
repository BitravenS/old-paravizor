package config

import (
	"github.com/bitravens/paravizor/v1/internal/ai"
	"github.com/bitravens/paravizor/v1/internal/store"
)

const PrvzrDir = "paravizor"
const ConfigFileName = "config.yaml"
const DEFAULT_XDG_CONFIG_DIRNAME = ".config"

type configError struct {
	configDir string
	err       error
}

type Config struct {
	APIKeys             map[string]string `yaml:"api_keys"`
	Theme               string            `yaml:"theme,omitempty"`
	DefaultPipeline     string            `yaml:"default_pipeline,omitempty"`
	RecentProjects      []string          `yaml:"recent_projects,omitempty"`
	MaxProcesses        int               `yaml:"max_concurrent_processes,omitempty" validate:"omitempty,gte=1"`
	HealthCheckInterval int               `yaml:"process_healthcheck_interval,omitempty" validate:"omitempty,gte=1"`
	DBConfig            *store.DBConfig   `yaml:"db,omitempty"`
	LogLevel            string            `yaml:"log_level,omitempty" validate:"omitempty,oneof=debug info warn error"`
	AIConfig            *ai.AIConfig      `yaml:"ai,omitempty"`
}

type ConfigWrapper struct {
	Config Config `yaml:"paravizor"`
}
