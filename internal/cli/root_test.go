package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func executeTestCommand(args ...string) (stdout string, stderr string, bootstraps int, err error) {
	originalBootstrap := bootstrapInit
	defer func() {
		bootstrapInit = originalBootstrap
	}()

	bootstrapInit = func() error {
		bootstraps++
		return nil
	}

	cmd := newRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err = cmd.Execute()
	return out.String(), errOut.String(), bootstraps, err
}

func executeRunnableTestCommand(bootstrap func() error) (bootstraps int, ran bool, err error) {
	originalBootstrap := bootstrapInit
	defer func() {
		bootstrapInit = originalBootstrap
	}()

	bootstrapInit = func() error {
		bootstraps++
		return bootstrap()
	}

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.AddCommand(&cobra.Command{
		Use: "smoke",
		Run: func(*cobra.Command, []string) {
			ran = true
		},
	})
	cmd.SetArgs([]string{"smoke"})

	err = cmd.Execute()
	return bootstraps, ran, err
}

func TestHelpDoesNotBootstrapOrRunTUI(t *testing.T) {
	stdout, _, bootstraps, err := executeTestCommand("--help")
	if err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	if bootstraps != 0 {
		t.Fatalf("help bootstrapped %d times, want 0", bootstraps)
	}
	for _, want := range []string{"Automated bug bounty recon pipeline.", "init", "run", "tools"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help output missing %q:\n%s", want, stdout)
		}
	}
}

func TestVersionDoesNotBootstrapOrRunTUI(t *testing.T) {
	stdout, _, bootstraps, err := executeTestCommand("--version")
	if err != nil {
		t.Fatalf("version returned error: %v", err)
	}
	if bootstraps != 0 {
		t.Fatalf("version bootstrapped %d times, want 0", bootstraps)
	}
	if !strings.Contains(stdout, "paravizor version 0.1.0") {
		t.Fatalf("unexpected version output: %q", stdout)
	}
}

func TestRunnableCommandBootstrapsOnce(t *testing.T) {
	bootstraps, ran, err := executeRunnableTestCommand(func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("smoke command returned error: %v", err)
	}
	if bootstraps != 1 {
		t.Fatalf("smoke command bootstrapped %d times, want 1", bootstraps)
	}
	if !ran {
		t.Fatal("smoke command did not run")
	}
}

func TestBootstrapErrorStopsRunnableCommand(t *testing.T) {
	wantErr := errors.New("bootstrap failed")
	bootstraps, ran, err := executeRunnableTestCommand(func() error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if bootstraps != 1 {
		t.Fatalf("smoke command bootstrapped %d times, want 1", bootstraps)
	}
	if ran {
		t.Fatal("smoke command ran after bootstrap error")
	}
}
