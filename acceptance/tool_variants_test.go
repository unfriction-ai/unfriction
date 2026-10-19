package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInspectRecordedToolVariants(t *testing.T) {
	binary := buildBinary(t)
	for _, tc := range []struct{ name, format string }{
		{"codex-create", "codex"}, {"codex-delete-lines", "codex"}, {"codex-rejected", "codex"},
		{"claude-image-png", "claude-code"}, {"claude-image-jpeg", "claude-code"}, {"claude-offset-edit", "claude-code"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture, err := filepath.Abs(filepath.Join("..", "fixtures", "tool-variants", tc.name+".jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("..", "fixtures", "tool-variants", tc.name+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			target := filepath.Join(dir, "README.md")
			original := []byte("# Example\nObsolete line\nOld line\n")
			if err := os.WriteFile(target, original, 0600); err != nil {
				t.Fatal(err)
			}
			for _, format := range []string{"auto", tc.format} {
				cmd := exec.Command(binary, "inspect", "--format", format, fixture)
				cmd.Dir = dir
				got, err := cmd.CombinedOutput()
				if err != nil || string(got) != string(want) {
					t.Fatalf("%s: %v\ngot:\n%s\nwant:\n%s", format, err, got, want)
				}
				unchanged, err := os.ReadFile(target)
				if err != nil || string(unchanged) != string(original) {
					t.Fatalf("inspection changed target: %v", err)
				}
				if _, err := os.Stat(filepath.Join(dir, "CREATED.md")); !os.IsNotExist(err) {
					t.Fatalf("inspection created recorded target: %v", err)
				}
			}
		})
	}
}
