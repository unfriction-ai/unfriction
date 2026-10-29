package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/unfriction-ai/unfriction/internal/recordings"
	"github.com/unfriction-ai/unfriction/internal/selection"
)

type exitError struct {
	code  int
	cause error
}

func (e exitError) Error() string {
	if e.cause == nil {
		return ""
	}
	return e.cause.Error()
}
func (e exitError) Unwrap() error { return e.cause }
func ExitCode(err error) int {
	var e exitError
	if errors.As(err, &e) {
		return e.code
	}
	return 1
}
func selectCommand() *cobra.Command {
	format, output := "auto", "posix"
	cmd := &cobra.Command{Use: "select", Short: "Find a recorded turn and emit an inspect command", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if output != "posix" && output != "zsh" {
			return fmt.Errorf("unsupported output %q; use posix or zsh", output)
		}
		row, err := pickRecordedTurn(cmd, format, "use")
		if errors.Is(err, selection.ErrCancelled) {
			return exitError{code: 130}
		}
		if err != nil {
			return err
		}
		command, err := selection.Command(row, output)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), command)
		return err
	}}
	cmd.Flags().StringVar(&format, "format", "auto", "Recording family: auto, codex, or claude-code")
	cmd.Flags().StringVar(&output, "output", "posix", "Quoted command syntax: posix or zsh")
	return cmd
}

// Both standalone selection and pathless inspection use the same typed picker.
func pickRecordedTurn(cmd *cobra.Command, format, acceptLabel string) (selection.Row, error) {
	if format != "auto" && format != "codex" && format != "claude-code" {
		return selection.Row{}, fmt.Errorf("unsupported format %q; use auto, codex, or claude-code", format)
	}
	cfg, err := recordings.DefaultConfig()
	if err != nil {
		return selection.Row{}, err
	}
	cfg.Format = format
	cwd, err := os.Getwd()
	if err != nil {
		return selection.Row{}, err
	}
	row, err := selection.Run(cmd.Context(), selection.Options{CWD: cwd, Sources: cfg, AcceptLabel: acceptLabel})
	if errors.Is(err, selection.ErrNoTerminal) {
		return selection.Row{}, exitError{code: 2, cause: err}
	}
	return row, err
}
