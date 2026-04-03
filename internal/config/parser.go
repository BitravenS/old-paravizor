package config

import (
	"fmt"
	"os"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

type parsingError struct {
	path string
	err  error
}

func (e parsingError) Error() string {
	return fmt.Sprintf("failed parsing config at path %s: %v", e.path, e.err)
}

// LoadConfig loads the application configuration with graceful fallbacks.
func LoadConfig(projectDir string) Config {
	defCfg := getDefaultConfig()

	// If PRVZR_CONFIG is set, we bypass standard resolution and strictly use that.
	envPath := os.Getenv("PRVZR_CONFIG")
	if envPath != "" {
		k, err := utils.LoadYAMLFile(envPath)
		if err == nil {
			cfg, err := utils.UnmarshalKoanf(k, defCfg)
			if err == nil {
				log.Info("Loaded config from PRVZR_CONFIG", "path", envPath)
				return cfg
			}
			log.Error("Failed to unmarshal PRVZR_CONFIG, using default config", "err", err)
		} else {
			log.Error("Failed to load PRVZR_CONFIG, using default config", "err", err)
		}
		return defCfg
	}

	globalPath, err := GetGlobalConfigPath()
	if err != nil {
		log.Error("Failed to resolve global config path, using default config", "err", err)
		return defCfg
	}

	// Try merging global + project override if projectDir is provided
	if projectDir != "" {
		overridePath := filepath.Join(projectDir, ConfigFileName)
		if _, err := os.Stat(overridePath); err == nil {
			k, err := utils.MergeYAMLFiles(true, globalPath, overridePath)
			if err == nil {
				cfg, err := utils.UnmarshalKoanf(k, defCfg)
				if err == nil {
					log.Info("Loaded config with project override", "global", globalPath, "override", overridePath)
					return cfg
				}
				log.Error("Failed to unmarshal merged config, falling back to global config", "err", err)
			} else {
				log.Error("Failed to merge project config, falling back to global config", "err", err)
			}
		}
	}

	// Fallback to loading just the global config
	k, err := utils.LoadYAMLFile(globalPath)
	if err == nil {
		cfg, err := utils.UnmarshalKoanf(k, defCfg)
		if err == nil {
			log.Info("Loaded global config", "path", globalPath)
			return cfg
		}
		log.Error("Failed to unmarshal global config, using default config", "err", err)
	} else {
		log.Error("Failed to load global config, using default config", "err", err)
	}

	return defCfg
}
