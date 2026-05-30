package theme

import (
	"image/color"
)

type Color string

type ThemeTextConfig struct {
	Primary   Color `yaml:"primary,omitempty" validate:"omitzero,omitempty,color"`
	Secondary Color `yaml:"secondary"                  validate:"omitempty,color"`
	Accent    Color `yaml:"accent"                     validate:"omitempty,color"`
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

type ThemeWrapper struct {
	Theme ThemeConfig `yaml:"theme"`
}

type AdaptiveColor struct {
	Light color.Color
	Dark  color.Color
}

func (c AdaptiveColor) RGBA() (uint32, uint32, uint32, uint32) {
	if c.Dark != nil {
		return c.Dark.RGBA()
	}
	if c.Light != nil {
		return c.Light.RGBA()
	}
	return 0, 0, 0, 0xffff
}

type Theme struct {
	SelectedBackground AdaptiveColor
	PrimaryBorder      AdaptiveColor
	FaintBorder        AdaptiveColor
	SecondaryBorder    AdaptiveColor
	FaintText          AdaptiveColor
	PrimaryText        AdaptiveColor
	SecondaryText      AdaptiveColor
	AccentText         AdaptiveColor
	InvertedText       AdaptiveColor
	SuccessText        AdaptiveColor
	WarningText        AdaptiveColor
	ErrorText          AdaptiveColor
}
