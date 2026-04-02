package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/bitravens/paravizor/v1/internal/config"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var dir string
	var scopeConfig config.ScopeConfig

	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Initialize a new Paravizor project",
		Args:  cobra.MaximumNArgs(3),
		Run: func(_ *cobra.Command, args []string) {
			if len(args) == 0 {
				cliInitProject()
				return
			}
			projectName := args[0]
			base := dir
			if base == "" {
				base = "."
			}
			projectPath := filepath.Join(base, projectName)
			if err := os.MkdirAll(projectPath, 0755); err != nil {
				log.Fatal("Failed to create project directory", err)
			}
			if err := os.MkdirAll(filepath.Join(projectPath, "tools"), 0755); err != nil {
				log.Fatal("Failed to create tools directory", err)
			}
			// TODO: Build project file from args
			cfgContent := fmt.Sprintf("")
			if err := os.WriteFile(filepath.Join(projectPath, "config.yaml"), []byte(cfgContent), 0644); err != nil {
				log.Fatal("Failed to write config file", err)
			}
			fmt.Printf("Initialized new Paravizor project at %s\n", projectPath)
		}}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Base directory to create the project in (default: current directory)")
	cmd.Flags().StringSliceVarP(&scopeConfig.Include, "include", "i", []string{}, "Domains (comma-separated regex) to include in scope")
	cmd.Flags().StringSliceVarP(&scopeConfig.Exclude, "exclude", "e", []string{}, "Domains (comma-separated regex) to exclude from scope")
	return cmd
}

func cliInitProject() {

}
