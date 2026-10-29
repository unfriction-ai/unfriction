package cli

import (
	"github.com/spf13/cobra"
	"github.com/unfriction-ai/unfriction/internal/inspection"
	"github.com/unfriction-ai/unfriction/internal/report"
)

func NewCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use: "unfriction", Short: "Inspect recorded coding-agent runs",
		Version:       version,
		SilenceErrors: true, SilenceUsage: true,
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	format := "auto"
	selector := ""
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
			"you choose to attach; only bounded, sanitized metadata is prefilled.\n\n" +
			"Use --turn codex:<turn-id> or --turn claude-code:<user-uuid> to select a\n" +
			"supported recorded turn from a session. Selection preserves physical source\n" +
			"lines and separates explanatory context from measured activity. Without\n" +
			"--turn, prepared-trace validation is unchanged; --format alone does not\n" +
			"enable session import.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := inspection.Read(args[0], format, selector, version)
			if err != nil {
				return err
			}
			return report.Write(cmd.OutOrStdout(), r)
		},
	}
	inspect.Flags().StringVar(&format, "format", "auto", "Input format: auto, codex, or claude-code (explicit family bypasses detection)")
	inspect.Flags().StringVar(&selector, "turn", "", "Select one recorded turn: codex:<turn-id> or claude-code:<user-uuid>")
	root.AddCommand(inspect, selectCommand())
	return root
}
