package acceptance_test

import (
	"os"
	"strings"
	"testing"
)

func TestInspectRecordedAcquisitionTiming(t *testing.T) {
	binary := buildBinary(t)
	read := func(path string) []string {
		t.Helper()
		b, err := os.ReadFile("../fixtures/" + path)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
	legacy := read("codex-acquisition.jsonl")
	missing := append([]string{}, legacy...)
	missing[3] = strings.Replace(missing[3], `Wall time: 0.0000 seconds\n`, "", 1)
	pending := append([]string{}, legacy[:4]...)
	pending = append(pending, strings.ReplaceAll(legacy[2], "call-1", "call-2"))
	pending = append(pending, legacy[4:]...)
	for _, tc := range []struct {
		name, format string
		lines, want  []string
	}{
		{"real structured intervals", "codex", read("timing-evidence/codex-0.163.0.jsonl"), []string{"a1: 0.071249250 seconds", "a2: 0.000007292 seconds"}},
		{"recorded rounded zero", "codex", legacy, []string{"a1: 0.0000 seconds"}},
		{"codex missing", "codex", missing, []string{"a1: unavailable"}},
		{"claude missing", "claude-code", read("claude-acquisition.jsonl"), []string{"a1: unavailable"}},
		{"pending", "codex", pending, []string{"a1: 0.0000 seconds", "a2: unavailable", "Diagnostic: Observation missing for a2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := inspectRepeatedInput(t, binary, tc.format, tc.lines)
			for _, want := range append(tc.want, "Recorded tool-execution duration", "vendor execution/output-wait intervals") {
				if !strings.Contains(got, want) {
					t.Fatalf("missing %q:\n%s", want, got)
				}
			}
			if strings.Contains(got, "Total duration") {
				t.Fatalf("unsupported aggregate: %s", got)
			}
		})
	}
}
