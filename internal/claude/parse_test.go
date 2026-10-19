package claude

import (
	"github.com/unfriction-ai/unfriction/internal/run"
	"os"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/claude-acquisition.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	fixture := string(b)
	lines := strings.Split(strings.TrimSpace(fixture), "\n")
	r, err := Parse(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if r.ExecutionState != run.Completed || len(r.Events) != 2 || r.Events[1].Text != "# Example\n" || r.Events[0].AcquisitionID != r.Events[1].AcquisitionID {
		t.Fatalf("%+v", r)
	}
	for _, tc := range []struct{ name, input, want string }{
		{"malformed", "{", "invalid event"},
		{"orphan", lines[0] + "\n" + lines[2], "unmatched observation"},
		{"duplicate", strings.Join(lines[:3], "\n") + "\n" + lines[2], "duplicate observation"},
		{"unsupported", strings.Replace(fixture, `"Read"`, `"Bash"`, 1), "unsupported acquisition"},
		{"multiple tasks", fixture + lines[0], "outside active run"},
		{"session mismatch", strings.Replace(fixture, `"session-1"`, `"other"`, 1), "multiple sessions"},
		{"unsupported result", strings.Replace(fixture, `"content": "# Example\n"`, `"content": []`, 1), "observation must be text"},
		{"unknown block", strings.Replace(fixture, `"type": "tool_use"`, `"type": "future_tool"`, 1), "unsupported content block"},
		{"refusal", strings.Replace(fixture, `"end_turn"`, `"refusal"`, 1), "unsupported stop reason"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.input))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
	r, err = Parse(strings.NewReader(strings.Join(lines[:2], "\n")))
	if err != nil || r.ExecutionState != run.Incomplete || len(r.MissingObservations()) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = Parse(strings.NewReader(strings.Replace(fixture, `"stop_reason": "end_turn"`, `"stop_reason": null`, 1)))
	if err != nil || r.ExecutionState != run.Incomplete {
		t.Fatalf("%+v %v", r, err)
	}
}
