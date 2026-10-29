package acceptance_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Catch selecting the wrong turn, rewriting a confirmation prompt, or counting
// earlier tool activity as part of the selected turn. Fixtures are composites.
func TestInspectSelectedTurn(t *testing.T) {
	binary := buildBinary(t)
	for _, tc := range []struct{ family, fixture, line string }{
		{"codex", "codex.jsonl", "10"},
		{"claude-code", "claude.jsonl", "6"},
	} {
		t.Run(tc.family, func(t *testing.T) {
			path := filepath.Join("..", "fixtures", "sessions", tc.fixture)
			output, err := exec.Command(binary, "inspect", "--format", tc.family, "--turn", tc.family+":second", "--", path).CombinedOutput()
			if err != nil {
				t.Fatalf("select second turn: %v\n%s", err, output)
			}
			for _, want := range []string{"Task:\nGo\n", "SELECTED_PAYLOAD", "Turn 2 of 2", "Previous agent:", "I will read selected.md to check validation.", "Earlier user:", "- Attempts: 1", "source line " + tc.line} {
				if !strings.Contains(string(output), want) {
					t.Errorf("missing %q:\n%s", want, output)
				}
			}
			if strings.Contains(string(output), "EARLIER_PAYLOAD") {
				t.Fatalf("leaked earlier measured activity:\n%s", output)
			}
			for _, selector := range []string{"2", "foreign:second", tc.family + ":", tc.family + ":absent"} {
				output, err := exec.Command(binary, "inspect", "--format", tc.family, "--turn", selector, "--", path).CombinedOutput()
				if err == nil || strings.Contains(string(output), "Execution state:") {
					t.Fatalf("invalid selector %q produced a report: %v\n%s", selector, err, output)
				}
			}
			output, err = exec.Command(binary, "inspect", "--format", tc.family, path).CombinedOutput()
			if err == nil || strings.Contains(string(output), "Execution state:") {
				t.Fatalf("whole session must still fail without selector: %v\n%s", err, output)
			}
		})
	}
}

// Current complete recordings must work without pre-reducing their history.
func TestInspectCurrentSession(t *testing.T) {
	binary := buildBinary(t)
	for _, tc := range []struct{ family, file, selector, prompt, final string }{
		{"codex", "codex-current.jsonl", "codex:redacted-1272", "redacted-1276", "redacted-1283"},
		{"claude-code", "claude-current.jsonl", "claude-code:redacted-14", "redacted-17", "redacted-523"},
	} {
		t.Run(tc.family, func(t *testing.T) {
			path := filepath.Join("..", "fixtures", "sessions", tc.file)
			out, err := exec.Command(binary, "inspect", "--format", tc.family, "--turn", tc.selector, "--", path).CombinedOutput()
			if err != nil {
				t.Fatalf("current session: %v\n%s", err, out)
			}
			for _, want := range []string{"Task:\n" + tc.prompt + "\n", "Final response:\n" + tc.final + "\n", "Execution state: COMPLETED"} {
				if !strings.Contains(string(out), want) {
					t.Errorf("missing %q:\n%s", want, out)
				}
			}
			if strings.Contains(string(out), "Acquisition measurements:") {
				t.Errorf("selected no-tool turn inherited measurements:\n%s", out)
			}
			out, err = exec.Command(binary, "inspect", "--format", tc.family, path).CombinedOutput()
			if err == nil || strings.Contains(string(out), "Execution state:") {
				t.Fatalf("prepared-path policy changed: %v\n%s", err, out)
			}
		})
	}
}
