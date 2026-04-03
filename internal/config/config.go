package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/theme"
	"github.com/bitravens/paravizor/v1/internal/utils"
	yamler "gopkg.in/yaml.v3"
)

func getDefaultConfig() Config {
	return Config{
		APIKeys:             make(map[string]string),
		Theme:               "default",
		DefaultPipeline:     "default",
		MaxProcesses:        10,
		HealthCheckInterval: 10,
		DBConfig:            nil,
		LogLevel:            "info",
		AIConfig:            nil,
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

	dir := filepath.Dir(configFilePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			return "", configError{configDir: dir, err: err}
		}
	}

	if err := createConfigFileIfMissing(configFilePath); err != nil {
		return "", configError{configDir: dir, err: err}
	}

	if err := initExternalDirs(dir); err != nil {
		log.Error("Failed to initialize external resource directories", "err", err)
	}

	return configFilePath, nil
}

func initExternalDirs(baseDir string) error {
	themesDir := filepath.Join(baseDir, "themes")
	if err := os.MkdirAll(themesDir, os.ModePerm); err != nil {
		return err
	}
	if err := theme.WriteDefaultTheme(filepath.Join(themesDir, "default.yaml")); err != nil {
		log.Warn("Failed writing default theme", "err", err)
	}

	pipelinesDir := filepath.Join(baseDir, "pipelines")
	if err := os.MkdirAll(pipelinesDir, os.ModePerm); err != nil {
		return err
	}
	if err := engine.WriteDefaultPipeline(filepath.Join(pipelinesDir, "default.yaml")); err != nil {
		log.Warn("Failed writing default pipeline", "err", err)
	}

	return nil
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
	data, err := yamler.Marshal(getDefaultConfig())
	if err != nil {
		return err
	}
	// Use utils.WriteYAML pattern but we already have the file handle.
	_, err = f.Write(data)
	return err
}

// WriteConfig serializes cfg to the given path using an atomic write.
func WriteConfig(cfgPath string, cfg Config) error {
	return utils.WriteYAML(cfgPath, cfg)
}
