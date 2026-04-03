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

var DefaultTheme = &Theme{
	PrimaryBorder: compat.AdaptiveColor{
		Light: lipgloss.ANSIColor(8),
		Dark:  lipgloss.ANSIColor(8),
	},
	SecondaryBorder: compat.AdaptiveColor{
		Light: lipgloss.ANSIColor(8),
		Dark:  lipgloss.ANSIColor(7),
	},
	SelectedBackground: compat.AdaptiveColor{
		Light: lipgloss.ANSIColor(7),
		Dark:  lipgloss.ANSIColor(236),
	},
	FaintBorder: compat.AdaptiveColor{
		Light: lipgloss.ANSIColor(254),
		Dark:  lipgloss.ANSIColor(234),
	},
	PrimaryText: compat.AdaptiveColor{
		Light: lipgloss.ANSIColor(0),
		Dark:  lipgloss.ANSIColor(15),
	},
	SecondaryText: compat.AdaptiveColor{
		Light: lipgloss.ANSIColor(244),
		Dark:  lipgloss.ANSIColor(251),
	},
	FaintText: compat.AdaptiveColor{
		Light: lipgloss.ANSIColor(7),
		Dark:  lipgloss.ANSIColor(245),
	},
	InvertedText: compat.AdaptiveColor{
		Light: lipgloss.ANSIColor(15),
		Dark:  lipgloss.ANSIColor(236),
	},
	SuccessText: compat.AdaptiveColor{
		Light: lipgloss.ANSIColor(10),
		Dark:  lipgloss.ANSIColor(10),
	},
	WarningText: compat.AdaptiveColor{
		Light: lipgloss.ANSIColor(11),
		Dark:  lipgloss.ANSIColor(11),
	},
	ErrorText: compat.AdaptiveColor{
		Light: lipgloss.ANSIColor(1),
		Dark:  lipgloss.ANSIColor(9),
	},
}
