package utils

import (
	"os"
	"path/filepath"
)

// XDGConfigDir returns the XDG_CONFIG_HOME directory, defaulting to ~/.config.
func XDGConfigDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// PrvzrConfigDir returns the paravizor config directory under XDG_CONFIG_HOME.
// Equivalent to: $XDG_CONFIG_HOME/paravizor
func PrvzrConfigDir() (string, error) {
	base, err := XDGConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "paravizor"), nil
}

// EnsureDir creates dir and all parent directories if they do not exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}
