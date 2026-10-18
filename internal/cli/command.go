package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/unfriction-ai/unfriction/internal/codex"
	"github.com/unfriction-ai/unfriction/internal/report"
)

func NewCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use: "unfriction", Short: "Inspect recorded coding-agent runs",
		Version:       version,
		SilenceErrors: true, SilenceUsage: true,
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.AddCommand(&cobra.Command{
		Use: "inspect <trace.jsonl>", Short: "Inspect one Codex task run without tool calls",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("open trace: %w", err)
			}
			defer file.Close()
			r, err := codex.Parse(file)
			if err != nil {
				return fmt.Errorf("parse trace: %w", err)
			}
			return report.Write(cmd.OutOrStdout(), r)
		},
	})
	return root
}
