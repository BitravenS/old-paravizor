package constants

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	Up            key.Binding
	Down          key.Binding
	FirstItem     key.Binding
	LastItem      key.Binding
	TogglePreview key.Binding
	OpenGithub    key.Binding
	Refresh       key.Binding
	PageDown      key.Binding
	PageUp        key.Binding
	NextSection   key.Binding
	PrevSection   key.Binding
	Help          key.Binding
	Quit          key.Binding
}

type Dimensions struct {
	Width  int
	Height int
}

const (
	Logo = `█▀▀█ █▀▀█ █▀▀█ █▀▀█ █  █ ▀█▀ ▀▀▀█ █▀▀█ █▀▀█
█▀▀▀ █▀▀█ █▀▀▄ █▀▀█ █▄▀   █   ▄▀  █  █ █▀▀▄
▀    ▀  ▀ ▀  ▀ ▀  ▀ ▀    ▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀  ▀`
)
