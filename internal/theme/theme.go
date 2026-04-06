package theme

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

// LoadTheme creates a new Theme starting from DefaultTheme, and applies
// any non-zero color overrides defined in the ThemeConfig.
func LoadTheme(cfg *ThemeConfig) *Theme {
	t := *DefaultTheme // Copy defaults
	if cfg == nil {
		return &t
	}

	// Helper to override AdaptiveColor if color string is provided
	override := func(target *compat.AdaptiveColor, c Color) {
		if !c.IsZero() {
			val := lipgloss.Color(string(c))
			target.Light = val
			target.Dark = val
		}
	}

	override(&t.PrimaryText, cfg.Text.Primary)
	override(&t.SecondaryText, cfg.Text.Secondary)
	override(&t.FaintText, cfg.Text.Faint)
	override(&t.InvertedText, cfg.Text.Inverted)
	override(&t.SuccessText, cfg.Text.Success)
	override(&t.WarningText, cfg.Text.Warning)
	override(&t.ErrorText, cfg.Text.Error)
	override(&t.AccentText, cfg.Text.Accent)
	override(&t.PrimaryBorder, cfg.Border.Primary)
	override(&t.SecondaryBorder, cfg.Border.Secondary)
	override(&t.FaintBorder, cfg.Border.Faint)

	override(&t.SelectedBackground, cfg.Background.Selected)

	return &t
}

func (c Color) String() string {
	return string(c)
}

func (c Color) IsZero() bool {
	return c.String() == ""
}

// Light theme is based on Catpuccin Latte, dark theme is based on Catpuccin Mocha
var DefaultTheme = &Theme{
	// Subtext 0
	PrimaryBorder: compat.AdaptiveColor{
		Light: lipgloss.Color("#6c6f85"),
		Dark:  lipgloss.Color("#a6adc8"),
	},
	// Mauve
	SecondaryBorder: compat.AdaptiveColor{
		Light: lipgloss.Color("#8839ef"),
		Dark:  lipgloss.Color("#cba6f7"),
	},
	// Surface 1
	SelectedBackground: compat.AdaptiveColor{
		Light: lipgloss.Color("#bcc0cc"),
		Dark:  lipgloss.Color("#45475a"),
	},
	//Surface 2
	FaintBorder: compat.AdaptiveColor{
		Light: lipgloss.Color("#acb0be"),
		Dark:  lipgloss.Color("#585b70"),
	},
	// Text
	PrimaryText: compat.AdaptiveColor{
		Light: lipgloss.Color("#4c4f69"),
		Dark:  lipgloss.Color("#cdd6f4"),
	},
	// Lavender
	SecondaryText: compat.AdaptiveColor{
		Light: lipgloss.Color("#7287fd"),
		Dark:  lipgloss.Color("#b4befe"),
	},
	// Teal
	AccentText: compat.AdaptiveColor{
		Light: lipgloss.Color("#179299"),
		Dark:  lipgloss.Color("#94e2d5"),
	},
	// overlay 1
	FaintText: compat.AdaptiveColor{
		Light: lipgloss.Color("#8c8fa1"),
		Dark:  lipgloss.Color("#7f849c"),
	},
	// Base
	InvertedText: compat.AdaptiveColor{
		Light: lipgloss.Color("#eff1f5"),
		Dark:  lipgloss.Color("#1e1e2e"),
	},
	// Green
	SuccessText: compat.AdaptiveColor{
		Light: lipgloss.Color("#40a02b"),
		Dark:  lipgloss.Color("#a6e3a1"),
	},
	// Yellow
	WarningText: compat.AdaptiveColor{
		Light: lipgloss.Color("#df8e1d"),
		Dark:  lipgloss.Color("#f9e2af"),
	},
	// Red
	ErrorText: compat.AdaptiveColor{
		Light: lipgloss.Color("#d20f39"),
		Dark:  lipgloss.Color("#f38ba8"),
	},
}
