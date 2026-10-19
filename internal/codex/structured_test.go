package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/unfriction-ai/unfriction/internal/run"
)

func structuredFixture(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("../../fixtures/repeated-evidence/codex-0.163.0.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestStructuredItemsRetainCombinedProvenance(t *testing.T) {
	lines := structuredFixture(t)
	r, err := Parse(strings.NewReader(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 8 || len(r.Coverage) != 4 || r.ExecutionState != run.Completed {
		t.Fatalf("%+v", r)
	}
	for i, want := range []struct {
		id, local, target string
		kind              run.EventKind
	}{
		{"read-1", "a1", "CONTEXT-1.md", run.Acquisition}, {"edit-1", "act1", "PROJECT-1.md", run.Action},
		{"read-2", "a2", "CONTEXT-2.md", run.Acquisition}, {"edit-2", "act2", "PROJECT-2.md", run.Action},
	} {
		request, feedback := r.Events[i*2], r.Events[i*2+1]
		if request.Kind != want.kind || request.Target != want.target || request.SourceID != want.id || feedback.SourceID != want.id || request.SourceLine != i+3 || feedback.SourceLine != request.SourceLine {
			t.Fatalf("%+v %+v", request, feedback)
		}
		if want.kind == run.Acquisition {
			if request.AcquisitionID != want.local || feedback.AcquisitionID != want.local || feedback.Kind != run.Observation {
				t.Fatalf("%+v %+v", request, feedback)
			}
		} else if request.ActionID != want.local || feedback.ActionID != want.local || feedback.Kind != run.Result {
			t.Fatalf("%+v %+v", request, feedback)
		}
	}
	// Three requests for the same path remain distinct, with no fixed cycle count.
	thirdRead := strings.Replace(lines[2], `"id": "read-1"`, `"id": "read-3"`, 1)
	thirdEdit := strings.ReplaceAll(lines[3], "edit-1", "edit-3")
	input := append(append(append([]string{}, lines[:6]...), thirdRead, thirdEdit), lines[6:]...)
	r, err = Parse(strings.NewReader(strings.Join(input, "\n")))
	if err != nil || len(r.Events) != 12 || r.Events[8].AcquisitionID != "a3" || r.Events[10].ActionID != "act3" || r.Events[8].Target != "CONTEXT-1.md" || r.Events[10].Target != "PROJECT-1.md" {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestStructuredItemsRejectAmbiguousOrUnsupportedInput(t *testing.T) {
	lines := structuredFixture(t)
	for _, tc := range []struct {
		name   string
		line   int
		change func(map[string]any)
		want   string
	}{
		{"missing item", 2, func(p map[string]any) { delete(p, "item") }, "item"},
		{"wrong turn", 3, func(p map[string]any) { p["turn_id"] = "other" }, "turn"},
		{"missing turn", 3, func(p map[string]any) { delete(p, "turn_id") }, "turn"},
		{"wrong thread", 3, func(p map[string]any) { p["thread_id"] = "other" }, "thread"},
		{"missing thread", 2, func(p map[string]any) { delete(p, "thread_id") }, "thread"},
		{"blank ID", 3, itemChange(func(i map[string]any) { i["id"] = " " }), "ID"},
		{"duplicate message ID", 3, itemChange(func(i map[string]any) { i["id"] = "task-1" }), "duplicate"},
		{"duplicate operation ID", 6, itemChange(func(i map[string]any) { i["id"] = "read-1" }), "duplicate"},
		{"unknown item", 3, itemChange(func(i map[string]any) { i["type"] = "Other" }), "unsupported"},
		{"compound command", 3, itemChange(func(i map[string]any) { i["command"] = []string{"/bin/zsh", "-lc", "cat CONTEXT-1.md; touch CREATED"} }), "command"},
		{"wrong shell", 3, itemChange(func(i map[string]any) { i["command"] = []string{"/bin/bash", "-lc", "cat CONTEXT-1.md"} }), "command"},
		{"wrong shell flag", 3, itemChange(func(i map[string]any) { i["command"] = []string{"/bin/zsh", "-c", "cat CONTEXT-1.md"} }), "command"},
		{"wrong source", 3, itemChange(func(i map[string]any) { i["source"] = "other" }), "source"},
		{"missing output", 3, itemChange(func(i map[string]any) { delete(i, "aggregated_output") }), "feedback"},
		{"null output", 3, itemChange(func(i map[string]any) { i["aggregated_output"] = nil }), "feedback"},
		{"missing exit", 3, itemChange(func(i map[string]any) { delete(i, "exit_code") }), "feedback"},
		{"null exit", 3, itemChange(func(i map[string]any) { i["exit_code"] = nil }), "feedback"},
		{"nonzero exit with completed", 3, itemChange(func(i map[string]any) { i["exit_code"] = 1 }), "status"},
		{"failed read", 3, itemChange(func(i map[string]any) { i["status"] = "failed"; i["exit_code"] = 1 }), "status"},
		{"conflicting stdout", 3, itemChange(func(i map[string]any) { i["stdout"] = "other" }), "feedback"},
		{"conflicting stderr", 3, itemChange(func(i map[string]any) { i["stderr"] = "other" }), "feedback"},
		{"conflicting formatted output", 3, itemChange(func(i map[string]any) { i["formatted_output"] = "other" }), "feedback"},
		{"missing action stdout", 4, itemChange(func(i map[string]any) { delete(i, "stdout") }), "feedback"},
		{"null action stderr", 4, itemChange(func(i map[string]any) { i["stderr"] = nil }), "feedback"},
		{"error text with completed action", 4, itemChange(func(i map[string]any) { i["stderr"] = "error" }), "status"},
		{"failed action", 4, itemChange(func(i map[string]any) { i["status"] = "failed" }), "status"},
		{"multiple targets", 4, itemChange(func(i map[string]any) { ch := i["changes"].(map[string]any); ch["OTHER.md"] = ch["PROJECT-1.md"] }), "one"},
		{"no targets", 4, itemChange(func(i map[string]any) { i["changes"] = map[string]any{} }), "one"},
		{"rename", 4, changeChange(func(c map[string]any) { c["move_path"] = "OTHER.md" }), "rename"},
		{"missing move marker", 4, changeChange(func(c map[string]any) { delete(c, "move_path") }), "rename"},
		{"delete", 4, changeChange(func(c map[string]any) { c["type"] = "delete" }), "update"},
		{"missing diff", 4, changeChange(func(c map[string]any) { delete(c, "unified_diff") }), "diff"},
		{"invalid diff", 4, changeChange(func(c map[string]any) { c["unified_diff"] = "not a diff" }), "diff"},
		{"task has attachment", 2, itemChange(func(i map[string]any) { i["content"] = append(i["content"].([]any), map[string]any{"type": "skill"}) }), "text"},
		{"empty task", 2, itemChange(func(i map[string]any) { i["content"].([]any)[0].(map[string]any)["text"] = " " }), "text"},
		{"nontext final", 7, itemChange(func(i map[string]any) { i["content"].([]any)[0].(map[string]any)["type"] = "Image" }), "text"},
		{"nonfinal message", 7, itemChange(func(i map[string]any) { i["phase"] = "commentary" }), "phase"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := append([]string{}, lines...)
			var record map[string]any
			if err := json.Unmarshal([]byte(input[tc.line-1]), &record); err != nil {
				t.Fatal(err)
			}
			tc.change(record["payload"].(map[string]any))
			b, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			input[tc.line-1] = string(b)
			r, err := Parse(strings.NewReader(strings.Join(input, "\n")))
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), fmt.Sprintf("line %d", tc.line)) || !reflect.DeepEqual(r, run.Run{}) {
				t.Fatalf("run=%+v err=%v", r, err)
			}
		})
	}
}

func itemChange(change func(map[string]any)) func(map[string]any) {
	return func(p map[string]any) { change(p["item"].(map[string]any)) }
}
func changeChange(change func(map[string]any)) func(map[string]any) {
	return itemChange(func(i map[string]any) { change(i["changes"].(map[string]any)["PROJECT-1.md"].(map[string]any)) })
}

func TestStructuredLifecycleAndMixedRepresentations(t *testing.T) {
	lines := structuredFixture(t)
	legacy, err := os.ReadFile("../../fixtures/codex-action.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	old := strings.Split(strings.TrimSpace(string(legacy)), "\n")
	join := func(parts ...[]string) string {
		var all []string
		for _, p := range parts {
			all = append(all, p...)
		}
		return strings.Join(all, "\n")
	}
	for _, tc := range []struct{ name, input, want string }{
		{"before start", join(lines[1:]), "active turn"},
		{"tool before task", join(lines[:1], lines[2:]), "phase"},
		{"duplicate task", join(lines[:2], []string{strings.ReplaceAll(lines[1], "task-1", "task-2")}, lines[2:]), "task"},
		{"after final", join(lines[:7], []string{strings.Replace(lines[2], `"id": "read-1"`, `"id": "read-3"`, 1)}, lines[7:]), "phase"},
		{"after completion", join(lines, []string{strings.Replace(lines[2], `"id": "read-1"`, `"id": "read-3"`, 1)}), "active turn"},
		{"legacy then structured", join(old[:2], lines[2:]), "mixed"},
		{"structured then legacy", join(lines[:6], old[2:4], lines[6:]), "mixed"},
		{"legacy final after structured", join(lines[:6], old[6:]), "mixed"},
		{"wrong completion", strings.Replace(join(lines), `"last_agent_message": "Revised the project files."`, `"last_agent_message": "different"`, 1), "completion"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Parse(strings.NewReader(tc.input))
			if err == nil || !strings.Contains(err.Error(), tc.want) || !reflect.DeepEqual(r, run.Run{}) {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
	for _, end := range []int{2, 6, 7} {
		r, err := Parse(strings.NewReader(join(lines[:end])))
		if err != nil || r.ExecutionState != run.Incomplete {
			t.Fatalf("end=%d %+v %v", end, r, err)
		}
	}
}

func TestStructuredFeedbackPreservesEmptyAndLiteralText(t *testing.T) {
	for _, text := range []string{"", "tool text\nwith trailing newline\n"} {
		lines := structuredFixture(t)
		for _, index := range []int{2, 3} {
			var record map[string]any
			if err := json.Unmarshal([]byte(lines[index]), &record); err != nil {
				t.Fatal(err)
			}
			item := record["payload"].(map[string]any)["item"].(map[string]any)
			field := "aggregated_output"
			if index == 3 {
				field = "stdout"
			}
			item[field] = text
			b, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			lines[index] = string(b)
		}
		r, err := Parse(strings.NewReader(strings.Join(lines[:6], "\n")))
		if err != nil || r.ExecutionState != run.Incomplete || len(r.Events) != 8 || r.Events[1].Text != text || r.Events[3].Text != text || r.Events[1].IsError || r.Events[3].IsError {
			t.Fatalf("%+v %v", r, err)
		}
	}
}

func TestStructuredTextElementsUseJSONStructure(t *testing.T) {
	lines := structuredFixture(t)
	lines[1] = strings.Replace(lines[1], `"text_elements": []`, `"text_elements": [ ]`, 1)
	if _, err := Parse(strings.NewReader(strings.Join(lines, "\n"))); err != nil {
		t.Fatal(err)
	}
}
