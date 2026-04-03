package theme

import (
	"charm.land/lipgloss/v2/compat"
)

type Color string

type ThemeTextConfig struct {
	Primary   Color `yaml:"primary,omitempty" validate:"omitzero,omitempty,color"`
	Secondary Color `yaml:"secondary"                  validate:"omitempty,color"`
	Inverted  Color `yaml:"inverted"                   validate:"omitempty,color"`
	Faint     Color `yaml:"faint"                      validate:"omitempty,color"`
	Warning   Color `yaml:"warning"                    validate:"omitempty,color"`
	Success   Color `yaml:"success"                    validate:"omitempty,color"`
	Error     Color `yaml:"error"                      validate:"omitempty,color"`
}

type ThemeBorderConfig struct {
	Primary   Color `yaml:"primary"   validate:"omitempty,color"`
	Secondary Color `yaml:"secondary" validate:"omitempty,color"`
	Faint     Color `yaml:"faint"     validate:"omitempty,color"`
}

type ThemeBackgroundConfig struct {
	Selected Color `yaml:"selected" validate:"omitempty,color"`
}

type ThemeConfig struct {
	Text       ThemeTextConfig       `yaml:"text,omitempty"       validate:"required,omitempty"`
	Background ThemeBackgroundConfig `yaml:"background,omitempty" validate:"required,omitempty"`
	Border     ThemeBorderConfig     `yaml:"border,omitempty"     validate:"required,omitempty"`
}

type Theme struct {
	SelectedBackground compat.AdaptiveColor
	PrimaryBorder      compat.AdaptiveColor
	FaintBorder        compat.AdaptiveColor
	SecondaryBorder    compat.AdaptiveColor
	FaintText          compat.AdaptiveColor
	PrimaryText        compat.AdaptiveColor
	SecondaryText      compat.AdaptiveColor
	InvertedText       compat.AdaptiveColor
	SuccessText        compat.AdaptiveColor
	WarningText        compat.AdaptiveColor
	ErrorText          compat.AdaptiveColor
}
