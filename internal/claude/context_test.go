package claude

import (
	"encoding/json"
	"github.com/unfriction-ai/unfriction/internal/run"
	"os"
	"strings"
	"testing"
)

func TestSessionHookShapes(t *testing.T) {
	data, err := os.ReadFile("../../fixtures/context-records/session-hook.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitN(string(data), "\n", 2)
	r, err := Parse(strings.NewReader(string(data)))
	if err != nil || len(r.Coverage) != 1 || len(r.Events) != 2 || r.Events[1].Text != "# Example\n" || r.ExecutionState != run.Completed {
		t.Fatalf("%+v %v", r, err)
	}
	for _, tc := range []struct {
		field string
		value any
	}{
		{"type", "private-sentinel"}, {"hookEvent", "Stop"}, {"exitCode", 1}, {"exitCode", 0.5}, {"durationMs", -1}, {"durationMs", nil},
		{"stdout", nil}, {"content", []any{}}, {"command", false}, {"hookName", ""}, {"toolUseID", ""}, {"futureField", true},
	} {
		t.Run(tc.field, func(t *testing.T) {
			var e map[string]any
			if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
				t.Fatal(err)
			}
			e["attachment"].(map[string]any)[tc.field] = tc.value
			b, _ := json.Marshal(e)
			r, err := Parse(strings.NewReader(string(b) + "\n" + lines[1]))
			if err == nil || !strings.Contains(err.Error(), "line 1") || strings.Contains(err.Error(), "private-sentinel") || len(r.Events) != 0 {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
}

func TestLastPromptRequiresUnambiguousLinearTurn(t *testing.T) {
	data, err := os.ReadFile("../../fixtures/context-records/last-prompt.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parse(strings.NewReader(string(data)))
	if err != nil || len(r.Coverage) != 0 || len(r.Events) != 2 || r.ExecutionState != run.Completed {
		t.Fatalf("%+v %v", r, err)
	}
	for _, tc := range []struct {
		name  string
		index int
		field string
		value any
	}{
		{"missing leaf", 0, "leafUuid", ""}, {"unresolved leaf", 0, "leafUuid", "private-sentinel"}, {"different session", 0, "sessionId", "private-sentinel"},
		{"extra bookkeeping field", 0, "branch", true}, {"sidechain", 1, "isSidechain", true},
		{"branch", 3, "parentUuid", "event-1"}, {"missing ancestor", 1, "parentUuid", "outside"}, {"duplicate node", 2, "uuid", "event-1"},
		{"null subsequent parent", 2, "parentUuid", nil}, {"missing node ID", 2, "uuid", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			var e map[string]any
			if err := json.Unmarshal([]byte(lines[tc.index]), &e); err != nil {
				t.Fatal(err)
			}
			e[tc.field] = tc.value
			b, _ := json.Marshal(e)
			lines[tc.index] = string(b)
			r, err := Parse(strings.NewReader(strings.Join(lines, "\n")))
			if err == nil || strings.Contains(err.Error(), "private-sentinel") || len(r.Events) != 0 {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
	// A pointer may reference a retained ancestor rather than the final response.
	r, err = Parse(strings.NewReader(strings.Replace(string(data), `"leafUuid": "event-1"`, `"leafUuid": "event-4"`, 1)))
	if err != nil || r.ExecutionState != run.Completed {
		t.Fatalf("%+v %v", r, err)
	}
}
