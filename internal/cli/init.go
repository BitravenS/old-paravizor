package cli

import (
	"fmt"

	"charm.land/log/v2"
	"github.com/bitravens/paravizor/v1/internal/project"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var dir string
	var scopeConfig project.ScopeConfig

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

			// Generate a default project config
			cfg, err := project.CreateProject(
				projectName,
				"New Paravizor project",
				base,
				"", // use default pipeline
				"", // use normal rate limit
				nil,
				scopeConfig,
			)
			if err != nil {
				log.Fatal("Failed to create project config", "err", err)
			}

			projectPath, err := project.InitProject(base, *cfg)
			if err != nil {
				log.Fatal("Failed to initialize project", "err", err)
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
