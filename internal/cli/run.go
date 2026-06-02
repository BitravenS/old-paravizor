package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitravens/paravizor/v1/internal/config"
	"github.com/bitravens/paravizor/v1/internal/events"
	"github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/runtime"
)

func newRunCmd() *cobra.Command {
	var dir string
	var installMissing bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the recon pipeline",
		Long:  "Run or resume the recon pipeline for a project directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectDir := dir
			if projectDir == "" {
				projectDir = "."
			}
			absDir, err := filepath.Abs(projectDir)
			if err != nil {
				return fmt.Errorf("resolve project dir: %w", err)
			}
			projCfg, err := project.LoadProject(absDir)
			if err != nil {
				return fmt.Errorf("load project: %w", err)
			}

			cfg := config.LoadConfig(absDir)
			eventCh := make(chan events.Event, 512)
			done := make(chan struct{})
			go printRunEvents(cmd, eventCh, done)

			runErr := runtime.RunPipeline(context.Background(), runtime.Options{
				ProjectDir:     absDir,
				Config:         cfg,
				Project:        &projCfg,
				InstallMissing: installMissing,
				EventCh:        eventCh,
			})
			close(eventCh)
			<-done
			if runErr != nil {
				return runErr
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Finished: pipeline completed.")
			return err
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Project directory to run the recon pipeline in")
	cmd.Flags().BoolVar(&installMissing, "install", false, "Attempt to install missing pipeline tools before running")
	return cmd
}

func printRunEvents(cmd *cobra.Command, eventCh <-chan events.Event, done chan<- struct{}) {
	defer close(done)
	out := cmd.OutOrStdout()
	for event := range eventCh {
		switch e := event.(type) {
		case events.PipelineStarted:
			_, _ = fmt.Fprintf(out, "Pipeline started: %d nodes\n", e.NodeCount)
		case events.NodeStarted:
			_, _ = fmt.Fprintf(out, "[%s] node started: %s\n", e.Timestamp().Format(time.Kitchen), e.NodeID)
		case events.NodeCompleted:
			_, _ = fmt.Fprintf(out, "[%s] node completed: %s in=%d out=%d\n", e.Timestamp().Format(time.Kitchen), e.NodeID, e.ItemsIn, e.ItemsOut)
		case events.NodeError:
			_, _ = fmt.Fprintf(os.Stderr, "[%s] node error: %s: %v\n", e.Timestamp().Format(time.Kitchen), e.NodeID, e.Err)
		case events.ProcessStarted:
			_, _ = fmt.Fprintf(out, "  exec %s pid=%d node=%s\n", e.ToolName, e.PID, e.NodeID)
		case events.FindingDiscovered:
			_, _ = fmt.Fprintf(out, "  finding [%s] %s\n", e.Severity, e.Title)
		case events.PipelineCompleted:
			_, _ = fmt.Fprintf(out, "Pipeline done: items=%d errors=%d duration=%s\n", e.TotalItems, e.TotalErrors, e.Duration.Round(time.Millisecond))
		case events.LogMessage:
			_, _ = fmt.Fprintf(out, "%s: %s\n", e.Level, e.Message)
		}
	}
}
