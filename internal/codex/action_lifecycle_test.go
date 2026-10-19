package codex

import (
	"encoding/json"
	"github.com/unfriction-ai/unfriction/internal/run"
	"os"
	"strings"
	"testing"
)

func TestActionLifecycleAndInvalidResults(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/codex-action.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	fixture := string(b)
	lines := strings.Split(strings.TrimSpace(fixture), "\n")
	join := func(parts ...[]string) string {
		var all []string
		for _, p := range parts {
			all = append(all, p...)
		}
		return strings.Join(all, "\n")
	}
	for _, tc := range []struct {
		name, input     string
		state           run.ExecutionState
		events, missing int
	}{
		{"action only", join(lines[:2], lines[4:]), run.Completed, 2, 0},
		{"missing result completed", join(lines[:5], lines[5+1:]), run.Completed, 3, 1},
		{"missing result incomplete", join(lines[:5]), run.Incomplete, 3, 1},
		{"result without termination", join(lines[:5+1]), run.Incomplete, 4, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Parse(strings.NewReader(tc.input))
			if err != nil || r.ExecutionState != tc.state || len(r.Events) != tc.events || len(r.MissingResults()) != tc.missing {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
	for _, tc := range []struct{ name, input, want string }{
		{"orphan", join(lines[:4], lines[5:]), "unmatched"},
		{"duplicate request", join(lines[:5], lines[4:]), "duplicate"},
		{"duplicate result", join(lines[:5+1], lines[5:]), "duplicate"},
		{"cross kind duplicate", strings.Replace(fixture, `"call_id": "edit-1"`, `"call_id": "read-1"`, 1), "duplicate"},
		{"after completion", fixture + lines[4], "outside"},
		{"unknown tool", strings.Replace(fixture, `apply_patch`, `unknown_tool`, 1), "unsupported"},
		{"null result", join(lines[:5], []string{nullResult(lines[5])}, lines[5+1:]), "missing action result"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.input))
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "line ") {
				t.Fatalf("want %s, got %v", tc.want, err)
			}
		})
	}
	errorFixture := strings.Replace(fixture, `Exit code: 0`, `Exit code: 1`, 1)
	for _, completed := range []bool{false, true} {
		input := errorFixture
		if !completed {
			parts := strings.Split(strings.TrimSpace(input), "\n")
			input = join(parts[:5+1])
		}
		r, err := Parse(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		want := run.Incomplete
		if completed {
			want = run.Completed
		}
		if r.ExecutionState != want || !r.Events[3].IsError {
			t.Fatalf("%+v", r)
		}
	}
}

func nullResult(line string) string {
	var e map[string]interface{}
	_ = json.Unmarshal([]byte(line), &e)
	e["payload"].(map[string]interface{})["output"] = nil
	b, _ := json.Marshal(e)
	return string(b)
}
