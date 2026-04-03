package theme

import (
	"github.com/bitravens/paravizor/v1/internal/utils"
	"os"
)

func WriteDefaultTheme(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}

	cfg := ThemeConfig{
		Text: ThemeTextConfig{
			Primary:   "#000000", // Will be converted to Adaptive internally but as an example
			Secondary: "#555555",
			Faint:     "#888888",
			Inverted:  "#ffffff",
			Success:   "#00ff00",
			Warning:   "#ffff00",
			Error:     "#ff0000",
		},
		Border: ThemeBorderConfig{
			Primary:   "#444444",
			Secondary: "#222222",
			Faint:     "#111111",
		},
		Background: ThemeBackgroundConfig{
			Selected: "#aaaaaa",
		},
	}

	return utils.WriteYAML(path, cfg)
}
