package acceptance_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectCodexAction(t *testing.T) { checkAction(t, "codex", "codex") }

func checkAction(t *testing.T, vendor, format string) {
	t.Helper()
	binary := buildBinary(t)
	fixture, err := filepath.Abs(filepath.Join("..", "fixtures", vendor+"-action.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "fixtures", vendor+"-action.txt"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"README.md", "GUIDE.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# Example\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"auto", format} {
		cmd := exec.Command(binary, "inspect", "--format", mode, fixture)
		cmd.Dir = dir
		got, err := cmd.CombinedOutput()
		if err != nil || string(got) != string(want) {
			t.Fatalf("%s: %v\ngot:\n%s\nwant:\n%s", mode, err, got, want)
		}
		for _, name := range []string{"README.md", "GUIDE.md"} {
			got, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil || string(got) != "# Example\n" {
				t.Fatalf("inspection modified %s: %q %v", name, got, err)
			}
		}
	}
}

func TestInspectClaudeAction(t *testing.T) { checkAction(t, "claude", "claude-code") }

func TestInspectActionEdgeScenarios(t *testing.T) {
	binary := buildBinary(t)
	for _, vendor := range []string{"codex", "claude"} {
		t.Run(vendor, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("..", "fixtures", vendor+"-action.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(b)), "\n")
			read, action, result := 2, 4, 5
			errorInput := strings.Replace(string(b), `Exit code: 0`, `Exit code: 1`, 1)
			if vendor == "claude" {
				read, action, result = 1, 3, 4
				errorInput = strings.Replace(string(b), `"tool_use_id": "edit-1"`, `"tool_use_id": "edit-1", "is_error": true`, 1)
			}
			join := func(a, b []string) string { return strings.Join(append(append([]string{}, a...), b...), "\n") }
			for _, tc := range []struct {
				name, input, want string
				invalid           bool
			}{
				{"action only", join(lines[:read], lines[action:]), "1. Action act1: Modify README.md", false},
				{"tool error", errorInput, "Tool reported an error.", false},
				{"missing result", join(lines[:result], lines[result+1:]), "Diagnostic: Result missing for act1", false},
				{"orphan result", join(lines[:action], lines[result:]), "unmatched", true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					path := filepath.Join(t.TempDir(), "trace.jsonl")
					if err := os.WriteFile(path, []byte(tc.input), 0600); err != nil {
						t.Fatal(err)
					}
					cmd := exec.Command(binary, "inspect", path)
					var stdout, stderr bytes.Buffer
					cmd.Stdout = &stdout
					cmd.Stderr = &stderr
					err := cmd.Run()
					if tc.invalid {
						if err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.want) || !strings.Contains(stderr.String(), "line ") {
							t.Fatalf("%v stdout=%s stderr=%s", err, &stdout, &stderr)
						}
						return
					}
					if err != nil || stderr.Len() != 0 || !strings.Contains(stdout.String(), tc.want) || !strings.Contains(stdout.String(), "Execution state: COMPLETED") || !strings.Contains(stdout.String(), "not verified task success") {
						t.Fatalf("%v stdout=%s stderr=%s", err, &stdout, &stderr)
					}
					if tc.name == "action only" && strings.Contains(stdout.String(), "Acquisition") {
						t.Fatal(stdout.String())
					}
				})
			}
		})
	}
}
