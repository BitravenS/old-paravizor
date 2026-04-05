package tool

import (
	"os/exec"
	"strings"
)

// CheckResult describes the availability status of a tool binary.
type CheckResult struct {
	Name      string
	Binary    string
	Available bool
	Path      string
	Version   string
	Install   string
}

// Check performs a detailed availability check for a tool definition.
// It resolves the binary on PATH and, if a version command is configured,
// runs it to capture the version string.
func Check(def *ToolConfig) CheckResult {
	result := CheckResult{
		Name:    def.Name,
		Binary:  def.Binary,
		Install: def.Install,
	}

	path, err := exec.LookPath(def.Binary)
	if err != nil {
		result.Available = false
		return result
	}

	result.Available = true
	result.Path = path

	// Attempt to capture the version string.
	if def.VersionCmd != "" {
		parts := strings.Fields(def.VersionCmd)
		if len(parts) > 0 {
			cmd := exec.Command(parts[0], parts[1:]...)
			out, err := cmd.Output()
			if err == nil {
				result.Version = strings.TrimSpace(string(out))
			}
		}
	}

	return result
}

// InstallHint returns a human-readable installation instruction for a tool.
func InstallHint(def *ToolConfig) string {
	if def.Install != "" {
		return def.Install
	}
	return "no install instructions available"
}
