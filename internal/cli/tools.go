package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/bitravens/paravizor/v1/internal/tool"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

func newToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Manage external tools used in the recon pipeline",
		RunE:  listTools,
	}
	cmd.AddCommand(newToolsListCmd())
	cmd.AddCommand(newToolsCheckCmd())
	cmd.AddCommand(newToolsShowCmd())
	return cmd
}

func listTools(cmd *cobra.Command, args []string) error {
	reg, _, err := loadToolRegistry()
	if err != nil {
		return err
	}
	rows := reg.All()
	var names []string
	for name := range rows {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		def := rows[name]
		status := "missing"
		if def.Available {
			status = "ok"
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-28s %-8s %s\n", name, status, def.Binary)
	}
	return nil
}

func newToolsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured tools",
		RunE:  listTools,
	}
}

func newToolsCheckCmd() *cobra.Command {
	var install bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check tool availability and optionally install missing tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _, err := loadToolRegistry()
			if err != nil {
				return err
			}
			missing := reg.Missing()
			if len(missing) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "All configured tools are available.")
				return nil
			}
			if install {
				if err := tool.InstallMissing(context.Background(), missing); err != nil {
					return err
				}
				reg.CheckAvailability(nil)
				missing = reg.Missing()
			}
			if len(missing) > 0 {
				var names []string
				for name := range missing {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					def := missing[name]
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "missing %-28s binary=%s install=%s\n", name, def.Binary, tool.InstallHint(def))
				}
				return fmt.Errorf("%d tools are missing", len(missing))
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "All configured tools are available.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&install, "install", false, "Attempt to install missing tools using supported install hints")
	return cmd
}

func newToolsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show details for a specific tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _, err := loadToolRegistry()
			if err != nil {
				return err
			}
			def, ok := reg.Get(args[0])
			if !ok {
				return fmt.Errorf("tool %q is not configured", args[0])
			}
			check := tool.Check(def)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\nBinary: %s\nAvailable: %t\nPath: %s\nConsumes: %s\nProduces: %s\nInstall: %s\nDescription: %s\n", def.Name, def.Binary, check.Available, check.Path, def.Consumes, def.Produces, tool.InstallHint(def), def.Description)
			if check.Version != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Version: %s\n", check.Version)
			}
			return nil
		},
	}
	return cmd
}

func loadToolRegistry() (*tool.Registry, string, error) {
	configDir, err := utils.PrvzrConfigDir()
	if err != nil {
		return nil, "", err
	}
	toolsDir := filepath.Join(configDir, "tools")
	reg := tool.NewRegistry()
	if err := reg.LoadDir(toolsDir); err != nil {
		return nil, toolsDir, err
	}
	reg.CheckAvailability(nil)
	return reg, toolsDir, nil
}
