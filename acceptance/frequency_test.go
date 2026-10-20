package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Mutations below are synthetic variants of supported sanitized fixtures, not
// additional claims about vendor telemetry.
func TestInspectAcquisitionFrequency(t *testing.T) {
	binary := buildBinary(t)
	readLines := func(path string) []string {
		t.Helper()
		b, err := os.ReadFile("../fixtures/" + path)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
	codex := readLines("codex-acquisition.jsonl")
	pending := append([]string{}, codex[:4]...)
	pending = append(pending, strings.ReplaceAll(codex[2], "call-1", "call-2"))
	pending = append(pending, codex[4:]...)
	claude := readLines("tool-variants/claude-image-png.jsonl")
	imageError := append([]string{}, claude[:3]...)
	imageError = append(imageError, strings.ReplaceAll(claude[1], "call-1", "call-2"))
	imageError = append(imageError, `{"type":"user","sessionId":"session-1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call-2","is_error":true,"content":"Read failed"}]}}`, claude[3])
	empty := readLines("claude-acquisition.jsonl")
	empty[2] = strings.Replace(empty[2], `# Example\n`, "", 1)
	for _, tc := range []struct {
		name, format               string
		lines                      []string
		attempts, matched, missing int
		diagnostic                 string
	}{
		{"codex pending", "codex", pending, 2, 1, 1, "Diagnostic: Observation missing for a2"},
		{"claude image and error", "claude-code", imageError, 2, 2, 0, "Read failed"},
		{"claude empty", "claude-code", empty, 1, 1, 0, "(empty)"},
		{"claude interleaved", "claude-code", readLines("claude-repeated.jsonl"), 2, 2, 0, "Action act2:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := inspectRepeatedInput(t, binary, tc.format, tc.lines)
			want := fmt.Sprintf("Acquisition measurements:\n- Attempts: %d\n- Matched observations: %d\n- Missing observations: %d\n", tc.attempts, tc.matched, tc.missing)
			if !strings.Contains(got, want) {
				t.Fatalf("missing frequency section %q:\n%s", want, got)
			}
			if !strings.Contains(got, tc.diagnostic) {
				t.Fatalf("missing preserved evidence %q:\n%s", tc.diagnostic, got)
			}
		})
	}
	t.Run("no acquisitions unchanged", func(t *testing.T) {
		want, err := os.ReadFile("../fixtures/first-run.txt")
		if err != nil {
			t.Fatal(err)
		}
		got, err := exec.Command(binary, "inspect", "../fixtures/first-run.jsonl").CombinedOutput()
		if err != nil || string(got) != string(want) {
			t.Fatalf("%v\ngot: %s\nwant: %s", err, got, want)
		}
	})
}
