package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitravens/paravizor/v1/internal/config"
	"github.com/bitravens/paravizor/v1/internal/engine"
	"github.com/bitravens/paravizor/v1/internal/theme"
	"github.com/bitravens/paravizor/v1/internal/tool"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

// NonFatalError represents bootstrap validation issues that should be surfaced
// to the user but do not require terminating startup.
type NonFatalError struct {
	Issues []string
}

func (e *NonFatalError) Error() string {
	return fmt.Sprintf("bootstrap validation warnings (%d): %s", len(e.Issues), strings.Join(e.Issues, "; "))
}

// IsNonFatal reports whether err contains bootstrap warnings only.
func IsNonFatal(err error) bool {
	var nf *NonFatalError
	return errors.As(err, &nf)
}

// NonFatalIssues extracts warning issues from a non-fatal bootstrap error.
func NonFatalIssues(err error) ([]string, bool) {
	var nf *NonFatalError
	if errors.As(err, &nf) {
		return nf.Issues, true
	}
	return nil, false
}

// Init verifies the configuration directory, creates missing defaults,
// and ensures all configurations (themes, pipelines, tools) are valid.
func Init() error {
	issues := make([]string, 0)

	dir, err := utils.PrvzrConfigDir()
	if err != nil {
		return fmt.Errorf("failed to resolve config dir: %w (suggested fix: set XDG_CONFIG_HOME to a writable directory)", err)
	}

	if err := utils.EnsureDir(dir); err != nil {
		return fmt.Errorf("failed to create config dir %q: %w (suggested fix: create the directory manually and verify permissions)", dir, err)
	}

	// 1. Config
	configPath, err := config.GetGlobalConfigPath()
	if err != nil {
		return fmt.Errorf("fatal config bootstrap error: %w (suggested fix: remove broken config and rerun `paravizor`, or fix YAML at %s)", err, filepath.Join(dir, config.ConfigFileName))
	}
	if _, err := utils.ParseYAML[config.ConfigWrapper](configPath); err != nil {
		issues = append(issues, fmt.Sprintf("invalid config file %q: %v (suggested fix: validate YAML under top-level key 'paravizor')", configPath, err))
	}

	// 2. Themes
	themesDir := filepath.Join(dir, "themes")
	if err := utils.EnsureDir(themesDir); err != nil {
		return fmt.Errorf("failed to create themes dir %q: %w (suggested fix: check directory permissions)", themesDir, err)
	}
	defaultThemePath := filepath.Join(themesDir, "default.yaml")
	if _, err := os.Stat(defaultThemePath); os.IsNotExist(err) {
		if err := theme.WriteDefaultTheme(defaultThemePath); err != nil {
			return fmt.Errorf("failed to rebuild missing default theme at %q: %w (suggested fix: create a valid theme file there)", defaultThemePath, err)
		}
	}
	if warn, err := validateDir(themesDir, func(path string) error {
		_, err := utils.ParseYAML[theme.ThemeConfig](path)
		return err
	}); err != nil {
		return fmt.Errorf("failed to validate themes directory %q: %w (suggested fix: ensure the directory is readable)", themesDir, err)
	} else {
		issues = append(issues, warn...)
	}

	// 3. Pipelines
	pipelinesDir := filepath.Join(dir, "pipelines")
	if err := utils.EnsureDir(pipelinesDir); err != nil {
		return fmt.Errorf("failed to create pipelines dir %q: %w (suggested fix: check directory permissions)", pipelinesDir, err)
	}
	defaultPipelinePath := filepath.Join(pipelinesDir, "default.yaml")
	if _, err := os.Stat(defaultPipelinePath); os.IsNotExist(err) {
		if err := engine.WriteDefaultPipeline(defaultPipelinePath); err != nil {
			return fmt.Errorf("failed to rebuild missing default pipeline at %q: %w (suggested fix: create a valid pipeline file there)", defaultPipelinePath, err)
		}
	}
	if warn, err := validateDir(pipelinesDir, func(path string) error {
		_, err := utils.ParseYAML[engine.PipelineWrapper](path)
		return err
	}); err != nil {
		return fmt.Errorf("failed to validate pipelines directory %q: %w (suggested fix: ensure the directory is readable)", pipelinesDir, err)
	} else {
		issues = append(issues, warn...)
	}

	// 4. Tools
	toolsDir := filepath.Join(dir, "tools")
	if err := utils.EnsureDir(toolsDir); err != nil {
		return fmt.Errorf("failed to create tools dir %q: %w (suggested fix: check directory permissions)", toolsDir, err)
	}
	if err := tool.WriteDefaultTools(toolsDir); err != nil {
		return fmt.Errorf("failed to rebuild missing default tools in %q: %w (suggested fix: create valid tool yaml files named <toolname>.yaml)", toolsDir, err)
	}
	if warn, err := validateDir(toolsDir, func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		wrappers, err := utils.ParseYAMLBytesMultiDoc[tool.ToolWrapper](data)
		if err != nil {
			return err
		}
		for _, w := range wrappers {
			if w.Tool.Name == "" {
				continue
			}
			if err := utils.Validator.Struct(w.Tool); err != nil {
				return fmt.Errorf("invalid tool %q: %w", w.Tool.Name, err)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to validate tools directory %q: %w (suggested fix: ensure the directory is readable)", toolsDir, err)
	} else {
		issues = append(issues, warn...)
	}

	if len(issues) > 0 {
		return &NonFatalError{Issues: issues}
	}

	return nil
}

func validateDir(dir string, validateFn func(path string) error) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	issues := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := validateFn(path); err != nil {
			issues = append(issues, fmt.Sprintf("invalid file %q: %v", path, err))
		}
	}

	return issues, nil
}
