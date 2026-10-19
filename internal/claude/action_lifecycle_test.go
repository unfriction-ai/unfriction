package claude

import (
	"encoding/json"
	"github.com/unfriction-ai/unfriction/internal/run"
	"os"
	"strings"
	"testing"
)

func TestActionLifecycleAndInvalidResults(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/claude-action.jsonl")
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
		{"action only", join(lines[:1], lines[3:]), run.Completed, 2, 0},
		{"missing result completed", join(lines[:4], lines[4+1:]), run.Completed, 3, 1},
		{"missing result incomplete", join(lines[:4]), run.Incomplete, 3, 1},
		{"result without termination", join(lines[:4+1]), run.Incomplete, 4, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Parse(strings.NewReader(tc.input))
			if err != nil || r.ExecutionState != tc.state || len(r.Events) != tc.events || len(r.MissingResults()) != tc.missing {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
	for _, tc := range []struct{ name, input, want string }{
		{"orphan", join(lines[:3], lines[4:]), "unmatched"},
		{"duplicate request", join(lines[:4], lines[3:]), "duplicate"},
		{"duplicate result", join(lines[:4+1], lines[4:]), "duplicate"},
		{"cross kind duplicate", strings.Replace(fixture, `"id": "edit-1"`, `"id": "read-1"`, 1), "duplicate"},
		{"after completion", fixture + lines[3], "outside"},
		{"unknown tool", strings.Replace(fixture, `Edit`, `unknown_tool`, 1), "unsupported"},
		{"null result", join(lines[:4], []string{nullResult(lines[4])}, lines[4+1:]), "observation must be text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.input))
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "line ") {
				t.Fatalf("want %s, got %v", tc.want, err)
			}
		})
	}
	errorFixture := strings.Replace(fixture, `"tool_use_id": "edit-1"`, `"tool_use_id": "edit-1", "is_error": true`, 1)
	for _, completed := range []bool{false, true} {
		input := errorFixture
		if !completed {
			parts := strings.Split(strings.TrimSpace(input), "\n")
			input = join(parts[:4+1])
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
	e["message"].(map[string]interface{})["content"].([]interface{})[0].(map[string]interface{})["content"] = nil
	b, _ := json.Marshal(e)
	return string(b)
}
