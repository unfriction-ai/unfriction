package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/unfriction-ai/unfriction/internal/claude"
	"github.com/unfriction-ai/unfriction/internal/codex"
	"github.com/unfriction-ai/unfriction/internal/compatreport"
	"github.com/unfriction-ai/unfriction/internal/diagnostic"
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
			"the entire trace. Unsupported variants fail without retrying another adapter.\n\n" +
			"Reports context acquisition and attempted file modifications in source order.\n" +
			"Supported modifications: Codex single-file Add File or Update File patches;\n" +
			"Claude Edit replacement or recorded offset/new_text attempts. Read results\n" +
			"may be text or one base64 PNG/JPEG image; image content is not interpreted.\n" +
			"Codex reads support cat <path> and sed -n '1,220p' <path>; Claude uses Read.\n" +
			"Only the documented reduced trace variants are supported. Inspection never\n" +
			"executes commands or applies changes. Completion is not verified task success.\n\n" +
			"Compatibility errors offer a prefilled public GitHub issue URL. The CLI makes\n" +
			"no network request and submits nothing. Review the draft and redact any trace\n" +
			"you choose to attach; only bounded, sanitized metadata is prefilled.",
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
			capture := compatreport.NewCapture(file)
			selected := format
			var input io.Reader = capture
			if selected == "auto" {
				selected, input, err = traceformat.Detect(capture)
				if err != nil {
					return compatreport.Append(err, version, "", "unknown", false)
				}
			}
			var r run.Run
			if selected == "claude-code" {
				r, err = claude.Parse(input)
			} else {
				r, err = codex.Parse(input)
			}
			if err != nil {
				agentVersion := "unknown"
				var failure *diagnostic.Failure
				if errors.As(err, &failure) {
					agentVersion = capture.AgentVersion(selected, failure.Line)
				}
				return compatreport.Append(fmt.Errorf("parse trace: %w", err), version, selected, agentVersion, format != "auto")
			}
			return report.Write(cmd.OutOrStdout(), r)
		},
	}
	inspect.Flags().StringVar(&format, "format", "auto", "Input format: auto, codex, or claude-code (explicit family bypasses detection)")
	root.AddCommand(inspect)
	return root
}
