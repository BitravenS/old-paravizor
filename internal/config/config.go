package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/bitravens/paravizor/v1/internal/ai"
	"github.com/bitravens/paravizor/v1/internal/utils"
	yamler "gopkg.in/yaml.v3"
)

// prvzrConfigDir returns the paravizor config directory ($XDG_CONFIG_HOME/paravizor).
// Kept as a thin wrapper here to avoid coupling callers to utils directly.
func prvzrConfigDir() (string, error) {
	return utils.PrvzrConfigDir()
}

func GetDefaultConfig() Config {
	aiCfg := ai.DefaultConfig()
	return Config{
		APIKeys:             make(map[string]string),
		Theme:               "default",
		DefaultPipeline:     "default",
		MaxProcesses:        10,
		HealthCheckInterval: 10,
		DBConfig:            nil,
		LogLevel:            "info",
		AIConfig:            &aiCfg,
	}
}

func (e configError) Error() string {
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

// GetGlobalConfigPath resolves the XDG global config path,
// creates the directory if needed, and writes a default config if the file
// does not yet exist.
func GetGlobalConfigPath() (string, error) {
	dir, err := prvzrConfigDir()
	if err != nil {
		return "", err
	}

	if err := utils.EnsureDir(dir); err != nil {
		return "", configError{configDir: dir, err: err}
	}

	configFilePath := filepath.Join(dir, ConfigFileName)
	log.Debug("using global config path", "path", configFilePath)

	if err := createConfigFileIfMissing(configFilePath); err != nil {
		return "", configError{configDir: dir, err: err}
	}

	return configFilePath, nil
}

func createConfigFileIfMissing(configFilePath string) error {
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Info("default config doesn't exist - writing", "path", configFilePath)

		f, err := os.OpenFile(configFilePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
		if err != nil {
			return err
		}
		defer f.Close()
		return writeDefaultConfigContents(f)
	}
	return nil
}

func writeDefaultConfigContents(f *os.File) error {
	data, err := yamler.Marshal(ConfigWrapper{Config: GetDefaultConfig()})
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// WriteConfig serializes cfg to the given path using an atomic write.
// The output YAML is wrapped under the "paravizor:" top-level key.
func WriteConfig(cfgPath string, cfg Config) error {
	return utils.WriteYAML(cfgPath, ConfigWrapper{Config: cfg})
}
