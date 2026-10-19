package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/unfriction-ai/unfriction/internal/claude"
	"github.com/unfriction-ai/unfriction/internal/codex"
	"github.com/unfriction-ai/unfriction/internal/report"
	"github.com/unfriction-ai/unfriction/internal/run"
	"github.com/unfriction-ai/unfriction/internal/traceformat"
)

func NewCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use: "unfriction", Short: "Inspect recorded coding-agent runs",
		Version:       version,
		SilenceErrors: true, SilenceUsage: true,
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	format := "auto"
	inspect := &cobra.Command{
		Use: "inspect <trace.jsonl>", Short: "Inspect one recorded task run",
		Long: "Inspect one recorded task run. Automatically detect Codex or Claude Code from\n" +
			"structural evidence in the first 64 KiB and 64 nonblank records. Unknown or\n" +
			"conflicting evidence fails without guessing. Use --format codex or\n" +
			"--format claude-code to bypass detection; the selected adapter still validates\n" +
			"the entire trace. Unsupported variants fail without retrying another adapter.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "auto" && format != "codex" && format != "claude-code" {
				return fmt.Errorf("unsupported format %q; expected auto, codex, or claude-code", format)
			}
			file, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("open trace: %w", err)
			}
			defer file.Close()
			selected := format
			var input io.Reader = file
			if selected == "auto" {
				selected, input, err = traceformat.Detect(file)
				if err != nil {
					return err
				}
			}
			var r run.Run
			if selected == "claude-code" {
				r, err = claude.Parse(input)
			} else {
				r, err = codex.Parse(input)
			}
			if err != nil {
				return fmt.Errorf("parse trace: %w", err)
			}
			return report.Write(cmd.OutOrStdout(), r)
		},
	}
	inspect.Flags().StringVar(&format, "format", "auto", "Input format: auto, codex, or claude-code (explicit family bypasses detection)")
	root.AddCommand(inspect)
	return root
}
