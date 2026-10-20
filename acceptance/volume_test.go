package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestInspectAcquisitionVolume(t *testing.T) {
	binary := buildBinary(t)
	readLines := func(path string) []string {
		t.Helper()
		b, err := os.ReadFile("../fixtures/" + path)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
	// These edge cases are synthetic mutations of supported sanitized records.
	codex := readLines("codex-action.jsonl")
	codex[3] = strings.Replace(codex[3], `# Example\n`, `é🙂\n`, 1)
	claude := readLines("tool-variants/claude-image-png.jsonl")
	mixed := append([]string{}, claude[:3]...)
	mixed = append(mixed, strings.ReplaceAll(claude[1], "call-1", "call-2"))
	mixed = append(mixed, `{"type":"user","sessionId":"session-1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call-2","content":"é🙂\n"}]}}`, claude[3])
	empty := readLines("claude-acquisition.jsonl")
	pendingError := append([]string{}, empty[:2]...)
	pendingError = append(pendingError, strings.Replace(empty[2], `# Example\n`, "", 1))
	pendingError = append(pendingError, strings.ReplaceAll(empty[1], "call-1", "call-2"))
	pendingError = append(pendingError, strings.ReplaceAll(empty[1], "call-1", "call-3"))
	pendingError = append(pendingError, `{"type":"user","sessionId":"session-1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call-3","is_error":true,"content":"error\n"}]}}`, empty[3])
	for _, tc := range []struct {
		name, format                            string
		lines                                   []string
		text, image, attempts, matched, missing int
	}{
		{"codex utf8 newline and action exclusion", "codex", codex, 7, 0, 1, 1, 0},
		{"claude text and image", "claude-code", mixed, 7, 73, 2, 2, 0},
		{"claude empty missing and error", "claude-code", pendingError, 6, 0, 3, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := inspectRepeatedInput(t, binary, tc.format, tc.lines)
			want := fmt.Sprintf("- Observed text: %d bytes (UTF-8)\n- Observed images: %d bytes (decoded)\n", tc.text, tc.image)
			if !strings.Contains(got, want) {
				t.Fatalf("missing volume %q:\n%s", want, got)
			}
			counts := fmt.Sprintf("- Attempts: %d\n- Matched observations: %d\n- Missing observations: %d\n", tc.attempts, tc.matched, tc.missing)
			if !strings.Contains(got, counts) {
				t.Fatalf("frequency changed: %s", got)
			}
			if !strings.Contains(got, "normalized observed result payload") {
				t.Fatalf("missing volume scope: %s", got)
			}
			if tc.missing > 0 && !strings.Contains(got, "Diagnostic: Observation missing for a2") {
				t.Fatalf("missing diagnostic: %s", got)
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
			t.Fatalf("%v\ngot %s\nwant %s", err, got, want)
		}
	})
}
