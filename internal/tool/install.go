package tool

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// InstallMissing attempts to install missing tool binaries using safe supported
// install hints. Currently supported forms are Go module paths and
// "go install <module>" commands.
func InstallMissing(ctx context.Context, defs map[string]*ToolConfig) error {
	var failures []string
	for name, def := range defs {
		hint := strings.TrimSpace(def.Install)
		if hint == "" {
			failures = append(failures, fmt.Sprintf("%s: no install hint", name))
			continue
		}

		var cmd *exec.Cmd
		parts := strings.Fields(hint)
		switch {
		case len(parts) == 1 && strings.Contains(parts[0], "/"):
			cmd = exec.CommandContext(ctx, "go", "install", parts[0])
		case len(parts) == 3 && parts[0] == "go" && parts[1] == "install":
			cmd = exec.CommandContext(ctx, "go", "install", parts[2])
		default:
			failures = append(failures, fmt.Sprintf("%s: unsupported install hint %q", name, hint))
			continue
		}

		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg != "" {
				failures = append(failures, fmt.Sprintf("%s: %v: %s", name, err, msg))
			} else {
				failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("install missing tools: %s", strings.Join(failures, "; "))
	}
	return nil
}
