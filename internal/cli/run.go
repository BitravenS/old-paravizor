package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitravens/paravizor/v1/internal/project"
)

func newRunCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Open a project in the TUI",
		Long:  "Open the Paravizor TUI focused on the specified project directory.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectDir, err := resolveRunProjectDir(dir)
			if err != nil {
				return err
			}
			if _, err := project.LoadProject(projectDir); err != nil {
				return fmt.Errorf("load project from %q: %w", projectDir, err)
			}
			return startTUI(cmd, projectDir)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Project directory to open in the TUI")
	return cmd
}

func resolveRunProjectDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	projectDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve project dir %q: %w", dir, err)
	}
	return projectDir, nil
}
