package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"charm.land/log/v2"

	"github.com/knadh/koanf/v2"
	yamler "gopkg.in/yaml.v3"

	"github.com/bitravens/paravizor/v1/internal/ai"
	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/store"
)

var conf = koanf.Conf{
	Delim:       ".",
	StrictMerge: true,
}

const PrvzrDir = "aravizor"
const ConfigFileName = "config.yaml"
const DEFAULT_XDG_CONFIG_DIRNAME = ".config"

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
	ToolPaths           map[string]string      `yaml:"tool_paths,omitempty"`
	DefaultPipeline     *engine.PipelineConfig `yaml:"default_pipeline,omitempty"`
	MaxProcesses        int                    `yaml:"max_processes,omitempty" validate:"omitempty,gte=10"`
	HealthCheckInterval int                    `yaml:"health_check_interval,omitempty" validate:"omitempty,gte=10"`
	DBConfig            *store.DBConfig        `yaml:"db_config,omitempty"`
	LogLevel            string                 `yaml:"log_level,omitempty" validate:"omitempty,oneof=debug info warn error"`
	AIConfig            *ai.AIConfig           `yaml:"ai_config,omitempty"`
}

// TODO: Make it actually work
func (parser ConfigParser) getDefaultConfig() Config {
	return Config{
		APIKeys:             make(map[string]string),
		Theme:               nil,
		ToolPaths:           make(map[string]string),
		DefaultPipeline:     nil,
		MaxProcesses:        10,
		HealthCheckInterval: 10,
		DBConfig:            nil,
		LogLevel:            "info",
		AIConfig:            nil,
	}
}

func (parser ConfigParser) getDefaultConfigYamlContents() (string, error) {
	defaultConfig := parser.getDefaultConfig()
	log.Debug("loading default config yaml contents")

	b, err := yamler.Marshal(defaultConfig)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (e configError) Error() string {
	// TODO: Link docs
	return fmt.Sprintf(
		`Couldn't find a config.yaml configuration file.
Create one under: %s

For more info, go to https://github.com/bitravens/paravizor
press q to exit.

Original error: %v`,
		path.Join(e.configDir, PrvzrDir, ConfigFileName),
		e.err,
	)
}

func (parser ConfigParser) writeDefaultConfigContents(
	newConfigFile *os.File,
) error {
	content, err := parser.getDefaultConfigYamlContents()
	if err != nil {
		return err
	}
	_, err = newConfigFile.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}

func (parser ConfigParser) createConfigFileIfMissing(
	configFilePath string,
) error {
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Info("default config doesn't exist - writing", "path", configFilePath, "err", err)

		newConfigFile, err := os.OpenFile(
			configFilePath,
			os.O_RDWR|os.O_CREATE|os.O_EXCL,
			0o666,
		)
		if err != nil {
			return err
		}

		defer newConfigFile.Close()
		return parser.writeDefaultConfigContents(newConfigFile)
	}

	return nil
}

func (parser ConfigParser) getGlobalConfigPathOrCreateIfMissing() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, DEFAULT_XDG_CONFIG_DIRNAME)
	}

	configFilePath := filepath.Join(configDir, PrvzrDir, ConfigFileName)
	log.Debug("using global config path", "path", configFilePath)

	// Ensure directory exists before attempting to create file
	configDir = filepath.Dir(configFilePath)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err = os.MkdirAll(configDir, os.ModePerm); err != nil {
			return "", configError{
				configDir: configDir,
				err:       err,
			}
		}
	}

	if err := parser.createConfigFileIfMissing(configFilePath); err != nil {
		return "", configError{configDir: configDir, err: err}
	}

	return configFilePath, nil
}
