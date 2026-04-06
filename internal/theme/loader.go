package theme

import (
	"os"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

// LoadExternalTheme loads a ThemeConfig from ~/.config/paravizor/themes/<name>.yaml.
// If the name is empty or "default", or if loading fails, it returns a nil ThemeConfig (falling back to DefaultTheme)
// and an error indicating what failed.
func LoadExternalTheme(name string) (*ThemeConfig, error) {
	if name == "" || name == "default" {
		return nil, nil
	}

	if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
		name += ".yaml"
	}

	prvzrDir, err := utils.PrvzrConfigDir()
	if err != nil {
		log.Warn("Failed to resolve paravizor config dir for theme", "err", err)
		return nil, err
	}

	themePath := filepath.Join(prvzrDir, "themes", name)
	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		log.Warn("Theme file not found, falling back to default", "path", themePath)
		return nil, err
	}

	w, err := utils.ParseYAML[ThemeWrapper](themePath)
	if err != nil {
		log.Error("Failed to parse theme, falling back to default", "path", themePath, "err", err)
		return nil, err
	}

	return &w.Theme, nil
}
