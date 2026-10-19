package codex

import (
	"encoding/json"
	"github.com/unfriction-ai/unfriction/internal/run"
	"os"
	"strings"
	"testing"
)

func TestContextRecords(t *testing.T) {
	for _, name := range []string{"world-state", "compaction"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../../fixtures/context-records/" + name + ".jsonl")
			if err != nil {
				t.Fatal(err)
			}
			r, err := Parse(strings.NewReader(string(data)))
			if err != nil || len(r.Coverage) != 1 || r.Coverage[0].SourceLine != 1 || len(r.Events) != 2 || r.ExecutionState != run.Completed {
				t.Fatalf("%+v %v", r, err)
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			r, err = Parse(strings.NewReader(strings.Join(lines[:len(lines)-1], "\n")))
			if err != nil || r.ExecutionState != run.Incomplete {
				t.Fatalf("context inferred completion: %+v %v", r, err)
			}
			for _, tc := range []struct {
				field string
				value any
			}{
				{"extra", true}, {"full", nil}, {"full", "private-sentinel"}, {"state", nil}, {"state", []any{}},
				{"encrypted_content", nil}, {"encrypted_content", 42}, {"id", ""}, {"internal_chat_message_metadata_passthrough", []any{}},
			} {
				if name == "world-state" && (tc.field == "encrypted_content" || tc.field == "id" || tc.field == "internal_chat_message_metadata_passthrough") {
					continue
				}
				if name == "compaction" && (tc.field == "full" || tc.field == "state") {
					continue
				}
				t.Run(tc.field, func(t *testing.T) {
					var record map[string]any
					if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
						t.Fatal(err)
					}
					record["payload"].(map[string]any)[tc.field] = tc.value
					changed, _ := json.Marshal(record)
					r, err := Parse(strings.NewReader(string(changed) + "\n" + strings.Join(lines[1:], "\n")))
					if err == nil || !strings.Contains(err.Error(), "line 1") || strings.Contains(err.Error(), "private-sentinel") || len(r.Events) != 0 {
						t.Fatalf("%+v %v", r, err)
					}
				})
			}
		})
	}
}

func TestContextCoverageOrder(t *testing.T) {
	world, err := os.ReadFile("../../fixtures/context-records/world-state.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	compact, err := os.ReadFile("../../fixtures/context-records/compaction.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	compactLine := strings.SplitN(string(compact), "\n", 2)[0]
	lines := strings.Split(strings.TrimSpace(string(world)), "\n")
	input := strings.Join(lines[:4], "\n") + "\n" + compactLine + "\n" + strings.Join(lines[4:], "\n")
	r, err := Parse(strings.NewReader(input))
	if err != nil || len(r.Coverage) != 2 || r.Coverage[1].SourceLine != 5 || len(r.Events) != 2 || r.Events[0].AcquisitionID != r.Events[1].AcquisitionID || r.Events[0].SourceLine != 4 || r.Events[1].SourceLine != 6 {
		t.Fatalf("%+v %v", r, err)
	}
}
