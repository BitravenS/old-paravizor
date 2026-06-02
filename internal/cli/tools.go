package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bitravens/paravizor/v1/internal/tool"
	"github.com/bitravens/paravizor/v1/internal/utils"
)

func newToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Manage external tools used in the recon pipeline",
		Args:  cobra.NoArgs,
		RunE:  listTools,
	}
	cmd.AddCommand(newToolsListCmd())
	cmd.AddCommand(newToolsShowCmd())
	return cmd
}

func listTools(cmd *cobra.Command, args []string) error {
	reg, toolsDir, err := loadToolRegistry()
	if err != nil {
		return err
	}

	defs := reg.All()
	names := sortedToolNames(defs)
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "Tools directory: %s\n\n", toolsDir)

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tNAME\tBINARY\tIO")
	for _, name := range names {
		def := defs[name]
		status := "missing"
		if def.Available {
			status = "ok"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s -> %s\n", status, def.Name, def.Binary, def.Consumes, def.Produces)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nTool definitions: %d, available: %d, missing: %d\n", len(defs), len(reg.Available()), len(reg.Missing()))
	return nil
}

func newToolsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured tools",
		Args:  cobra.NoArgs,
		RunE:  listTools,
	}
	return cmd
}

func newToolsShowCmd() *cobra.Command {
	var toolName string
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show details for a specific tool",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if toolName == "" && len(args) > 0 {
				toolName = args[0]
			}
			toolName = strings.TrimSpace(toolName)
			if toolName == "" {
				return fmt.Errorf("tool name is required")
			}

			reg, toolsDir, err := loadToolRegistry()
			if err != nil {
				return err
			}
			def, ok := reg.Get(toolName)
			if !ok {
				return fmt.Errorf("tool %q is not configured in %s", toolName, toolsDir)
			}

			printToolDetails(cmd, def, toolsDir)
			return nil
		},
	}
	cmd.Flags().StringVarP(&toolName, "name", "n", "", "Name of the tool to show details for")
	return cmd
}

func loadToolRegistry() (*tool.Registry, string, error) {
	configDir, err := utils.PrvzrConfigDir()
	if err != nil {
		return nil, "", fmt.Errorf("resolve config dir: %w", err)
	}
	toolsDir := filepath.Join(configDir, "tools")
	reg := tool.NewRegistry()
	if err := reg.LoadDir(toolsDir); err != nil {
		return nil, "", fmt.Errorf("load tools from %q: %w", toolsDir, err)
	}
	reg.CheckAvailability(nil)
	return reg, toolsDir, nil
}

func sortedToolNames(defs map[string]*tool.ToolConfig) []string {
	names := make([]string, 0, len(defs))
	for name := range defs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func printToolDetails(cmd *cobra.Command, def *tool.ToolConfig, toolsDir string) {
	out := cmd.OutOrStdout()
	availability := "missing"
	if def.Available {
		availability = "available"
	}

	fmt.Fprintf(out, "Name: %s\n", def.Name)
	fmt.Fprintf(out, "Description: %s\n", def.Description)
	fmt.Fprintf(out, "Config directory: %s\n", toolsDir)
	fmt.Fprintf(out, "Binary: %s\n", def.Binary)
	fmt.Fprintf(out, "Availability: %s\n", availability)
	if def.BinaryPath != "" {
		fmt.Fprintf(out, "Binary path: %s\n", def.BinaryPath)
	}
	if def.VersionCmd != "" {
		fmt.Fprintf(out, "Version command: %s\n", def.VersionCmd)
	}
	if def.Install != "" {
		fmt.Fprintf(out, "Install: %s\n", def.Install)
	}
	fmt.Fprintf(out, "Consumes: %s\n", def.Consumes)
	fmt.Fprintf(out, "Produces: %s\n", def.Produces)
	fmt.Fprintf(out, "Input: %s", def.Input.Type)
	if def.Input.Flag != "" {
		fmt.Fprintf(out, " %s", def.Input.Flag)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Output: %s %s\n", def.Output.Type, def.Output.Format)
	if len(def.Flags) > 0 {
		fmt.Fprintf(out, "Default flags: %s\n", strings.Join(def.Flags, " "))
	}
	if len(def.UserFlags) > 0 {
		fmt.Fprintf(out, "User flags: %s\n", strings.Join(def.UserFlags, " "))
	}
}
