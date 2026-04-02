package config

type Color string

func (c Color) String() string {
	return string(c)
}

func (c Color) IsZero() bool {
	return c.String() == ""
}

type ColorThemeIcon struct {
	NewContributor Color `yaml:"newcontributor" validate:"omitempty,color"`
	Contributor    Color `yaml:"contributor"    validate:"omitempty,color"`
	Collaborator   Color `yaml:"collaborator"   validate:"omitempty,color"`
	Member         Color `yaml:"member"         validate:"omitempty,color"`
	Owner          Color `yaml:"owner"          validate:"omitempty,color"`
	UnknownRole    Color `yaml:"unknownrole"    validate:"omitempty,color"`
}

type ColorThemeText struct {
	Primary   Color `yaml:"primary,omitzero,omitempty" validate:"omitzero,omitempty,color"`
	Secondary Color `yaml:"secondary"                  validate:"omitempty,color"`
	Inverted  Color `yaml:"inverted"                   validate:"omitempty,color"`
	Faint     Color `yaml:"faint"                      validate:"omitempty,color"`
	Warning   Color `yaml:"warning"                    validate:"omitempty,color"`
	Success   Color `yaml:"success"                    validate:"omitempty,color"`
	Error     Color `yaml:"error"                      validate:"omitempty,color"`
	Actor     Color `yaml:"actor"                      validate:"omitempty,color"`
}

type ColorThemeBorder struct {
	Primary   Color `yaml:"primary"   validate:"omitempty,color"`
	Secondary Color `yaml:"secondary" validate:"omitempty,color"`
	Faint     Color `yaml:"faint"     validate:"omitempty,color"`
}

type ColorThemeBackground struct {
	Selected Color `yaml:"selected" validate:"omitempty,color"`
}

type ColorTheme struct {
	Icon       ColorThemeIcon       `yaml:"icon,omitempty"       validate:"required,omitempty"`
	Text       ColorThemeText       `yaml:"text,omitempty"       validate:"required,omitempty"`
	Background ColorThemeBackground `yaml:"background,omitempty" validate:"required,omitempty"`
	Border     ColorThemeBorder     `yaml:"border,omitempty"     validate:"required,omitempty"`
}

// TODO: Implement
type ColorThemeConfig struct {
}
type UIThemeConfig struct {
}

type ThemeConfig struct {
	UI     UIThemeConfig     `yaml:"ui,omitempty"     validate:"omitempty"`
	Colors *ColorThemeConfig `yaml:"colors,omitempty" validate:"omitempty"`
}
