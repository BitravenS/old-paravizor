// Package cli contains the command-line interface for the Paravizor application.
package cli

import (
	"context"
	"fmt"
	slog "log"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/bitravens/paravizor/v1/internal/bootstrap"
	"github.com/bitravens/paravizor/v1/internal/tui"
	"github.com/bitravens/paravizor/v1/internal/tui/constants"
	pctx "github.com/bitravens/paravizor/v1/internal/tui/context"
)

var (
	version = "0.1.0"

	logo = lipgloss.NewStyle().Foreground(pctx.LogoColor).MarginBottom(1).SetString(constants.Logo)

	bootstrapInit = bootstrap.Init
	startTUI      = func(cmd *cobra.Command, location string) error {
		debug, err := cmd.Root().Flags().GetBool("debug")
		if err != nil {
			return fmt.Errorf("parse debug flag: %w", err)
		}

		model, logger := createModel(location, debug)
		if logger != nil {
			defer logger.Close()
		}

		p := tea.NewProgram(model)
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("start TUI: %w", err)
		}
		return nil
	}
)

func setDebugLogLevel() {
	switch os.Getenv("LOG_LEVEL") {
	case "debug", "":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	}
}

func Execute() error {
	return ExecuteContext(context.Background(), os.Args[1:])
}

func ExecuteContext(ctx context.Context, args []string) error {
	cobra.MousetrapHelpText = ""

	cmd := newRootCmd()
	cmd.SetArgs(args)

	return fang.Execute(
		ctx,
		cmd,
		fang.WithVersion(cmd.Version),
		fang.WithoutCompletions(),
		fang.WithoutManpage(),
	)
}

func runBootstrap(cmd *cobra.Command) error {
	if err := bootstrapInit(); err != nil {
		if issues, ok := bootstrap.NonFatalIssues(err); ok {
			for _, issue := range issues {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", issue)
			}
		} else {
			return err
		}
	}
	return nil
}

func createModel(location string, debug bool) (tui.Model, *os.File) {
	var loggerFile *os.File

	if debug {
		var fileErr error
		currentTime := time.Now().Format("2006-01-02_15-04-05:000")
		logfileName := fmt.Sprintf("%s.log", currentTime)
		logfilePath := fmt.Sprintf("%s/.config/paravizor/logs/%s", os.Getenv("HOME"), logfileName)
		if err := os.MkdirAll(fmt.Sprintf("%s/.config/paravizor/logs", os.Getenv("HOME")), 0o755); err != nil {
			slog.Printf("Failed creating log directory: %v", err)
		}
		// TODO: don't hardcode the log path
		loggerFile, fileErr = os.OpenFile(logfilePath,
			os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
		if fileErr == nil {
			log.SetOutput(loggerFile)
			log.SetTimeFormat(time.Kitchen)
			log.SetReportCaller(true)
			setDebugLogLevel()
			log.Info("Logging to file", "path", logfilePath)
			if location != "" {
				log.Info("Running in project", "project", location)
			}
		} else {
			loggerFile, _ = tea.LogToFile(logfilePath, "debug")
			slog.Print("Failed setting up logging", fileErr)
		}
	} else {
		log.SetOutput(os.Stderr)
		log.SetLevel(log.FatalLevel)
	}

	return tui.NewModel(location, version, nil), loggerFile
}

func newRootCmd() *cobra.Command {
	cobra.MousetrapHelpText = ""

	cmd := &cobra.Command{
		Use: "paravizor",
		Long: lipgloss.JoinVertical(lipgloss.Left, logo.Render(),
			"Automated bug bounty recon pipeline.",
			"Give us a star on GitHub if you like the project! https://github.com/bitravens/paravizor"),
		Short:   "Automated bug bounty recon pipeline.",
		Version: version,
		Example: `
# Running without arguments will start the application TUI
paravizor

# Show help menu and available commands
paravizor -h

# Run with debug logging
paravizor --debug

# Print version
paravizor -v
	`,
		Args: cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return runBootstrap(cmd)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return startTUI(cmd, "")
		},
	}
	cmd.SetVersionTemplate(`paravizor {{printf "version %s\n" .Version}}`)

	cmd.Flags().Bool(
		"debug",
		false,
		"Debug output to ~/.config/paravizor/logs",
	)

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newQueryCmd())
	cmd.AddCommand(newToolsCmd())
	return cmd
}
