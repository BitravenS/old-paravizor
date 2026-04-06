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
			Primary:   "#cdd6f4", // Catppuccin Mocha — Text
			Secondary: "#b4befe", // Catppuccin Mocha — Lavender
			Accent:    "#94e2d5", // Catppuccin Mocha — Teal
			Faint:     "#7f849c", // Catppuccin Mocha — Overlay 1
			Inverted:  "#1e1e2e", // Catppuccin Mocha — Base
			Success:   "#a6e3a1", // Catppuccin Mocha — Green
			Warning:   "#f9e2af", // Catppuccin Mocha — Yellow
			Error:     "#f38ba8", // Catppuccin Mocha — Red
		},
		Border: ThemeBorderConfig{
			Primary:   "#a6adc8", // Catppuccin Mocha — Subtext 0
			Secondary: "#cba6f7", // Catppuccin Mocha — Mauve
			Faint:     "#585b70", // Catppuccin Mocha — Surface 2
		},
		Background: ThemeBackgroundConfig{
			Selected: "#45475a", // Catppuccin Mocha — Surface 1
		},
	}

	return utils.WriteYAML(path, ThemeWrapper{Theme: cfg})
}
