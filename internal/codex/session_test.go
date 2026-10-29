package codex

import (
	"os"
	"strings"
	"testing"
)

func TestSelectedTurnPreservesPhysicalEvents(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/sessions/codex.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	r, d, i, err := ParseTurn(strings.NewReader(string(b)), "second")
	if err != nil {
		t.Fatal(err)
	}
	if i != 1 || len(d.Turns) != 2 || r.Task != "Go" || len(r.Events) != 2 || r.Events[0].SourceLine != 10 || r.Events[1].SourceLine != 11 || r.Events[0].SourceID != "selected-read" {
		t.Fatalf("incorrect isolation: %+v, %+v", r, d)
	}
}

func TestSelectedTurnRejectsInvalidBoundaries(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/sessions/codex.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	base := string(b)
	lines := strings.Split(strings.TrimSpace(base), "\n")
	lateResult := strings.Join(append(append(append([]string{}, lines[:10]...), lines[11:]...), lines[10]), "\n")
	foreignResult := strings.Join(append(append(append([]string{}, lines[:6]...), lines[10]), append(append([]string{}, lines[6:10]...), lines[11:]...)...), "\n")
	for _, tc := range []struct{ name, input, id string }{
		{"missing selector", base, "absent"},
		{"result after completion", lateResult, "second"},
		{"result moved into another active turn", foreignResult, "second"},
		{"uncorrelated assistant representation", strings.Replace(base, lines[12], `{"type":"response_item","payload":{"type":"message","id":"unrelated","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":"Conflicting final"}]}}`+"\n"+lines[12], 1), "second"},
		{"missing session identity", strings.Replace(base, `"id":"session-codex"`, `"id":""`, 1), "second"},
		{"duplicate turn", strings.ReplaceAll(base, `"second"`, `"first"`), "first"},
		{"cross-turn result", strings.Replace(base, `"call_id":"selected-read","output"`, `"call_id":"old-read","output"`, 1), "second"},
		{"conflicting completion", strings.Replace(base, `"last_agent_message":"Validation checked."`, `"last_agent_message":"different"`, 1), "second"},
		{"unknown record", base + "\n" + `{"type":"unknown"}`, "second"},
		{"partial JSON", base + "\n{", "second"},
		{"missing boundary evidence after compaction", base + "\n" + `{"type":"compacted","payload":{}}`, "second"},
		{"ambiguous operation identity", strings.Replace(base, `{"type":"event_msg","payload":{"type":"agent_message","phase":"final_answer","message":"Validation checked."}}`, `{"type":"event_msg","payload":{"type":"item_completed","thread_id":"session-codex","turn_id":"second","item":{"type":"CommandExecution","id":"selected-read","command":["/bin/zsh","-lc","cat selected.md"],"source":"unified_exec_startup","status":"completed","exit_code":0,"aggregated_output":"SELECTED_PAYLOAD"}}}`+"\n"+`{"type":"event_msg","payload":{"type":"agent_message","phase":"final_answer","message":"Validation checked."}}`, 1), "second"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, _, err := ParseTurn(strings.NewReader(tc.input), tc.id)
			if err == nil || r.Task != "" {
				t.Fatalf("expected no report, got %+v %v", r, err)
			}
		})
	}
}

func TestSessionRejectsConflictingRawAgentMirror(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/sessions/codex-current.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	lines[42] = strings.ReplaceAll(lines[42], `"redacted-1283"`, `"conflicting-text"`)
	_, _, _, err = ParseTurn(strings.NewReader(strings.Join(lines, "\n")), "redacted-1272")
	if err == nil {
		t.Fatal("conflicting mirror accepted")
	}
}

func TestSessionRejectsConflictingRepresentations(t *testing.T) {
	b, _ := os.ReadFile("../../fixtures/sessions/codex-current.jsonl")
	for _, tc := range []struct {
		name             string
		line             int
		old, replacement string
	}{
		{"user prompt", 39, `"redacted-1276"`, `"conflicting"`},
		{"foreign mirror", 42, `"role":"assistant"`, `"role":"assistant","turn_id":"foreign"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := strings.Split(string(b), "\n")
			lines[tc.line] = strings.Replace(lines[tc.line], tc.old, tc.replacement, 1)
			if _, _, _, err := ParseTurn(strings.NewReader(strings.Join(lines, "\n")), "redacted-1272"); err == nil {
				t.Fatal("conflict accepted")
			}
		})
	}
}

func TestSelectedTurnIgnoresMalformedOptionalVersion(t *testing.T) {
	b, _ := os.ReadFile("../../fixtures/sessions/codex.jsonl")
	input := strings.Replace(string(b), `"cli_version":"0.163.0"`, `"cli_version":123`, 1)
	if _, _, _, err := ParseTurn(strings.NewReader(input), "first"); err != nil {
		t.Fatal(err)
	}
}

func TestMixedSessionKeepsDistinctOperations(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/sessions/codex.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	extra := `{"type":"event_msg","payload":{"type":"item_completed","thread_id":"session-codex","turn_id":"second","item":{"type":"CommandExecution","id":"separate-read","command":["/bin/zsh","-lc","cat separate.md"],"source":"unified_exec_startup","status":"completed","exit_code":0,"aggregated_output":"SEPARATE"}}}`
	lines = append(lines[:11], append([]string{extra}, lines[11:]...)...)
	r, _, _, err := ParseTurn(strings.NewReader(strings.Join(lines, "\n")), "second")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 4 || r.AcquisitionFrequency().Attempts != 2 || r.Events[2].SourceID != "separate-read" {
		t.Fatalf("lost or duplicated operations: %+v", r)
	}
	if _, err := Parse(strings.NewReader(strings.Join(lines[7:], "\n"))); err == nil {
		t.Fatal("prepared parser accepted mixed turn")
	}
}

func TestSessionAbortDoesNotComplete(t *testing.T) {
	b, _ := os.ReadFile("../../fixtures/sessions/codex.jsonl")
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	lines = append(lines[:11], `{"type":"event_msg","payload":{"type":"turn_aborted","turn_id":"second"}}`)
	r, _, _, err := ParseTurn(strings.NewReader(strings.Join(lines, "\n")), "second")
	if err != nil || r.ExecutionState != "INCOMPLETE" {
		t.Fatalf("%+v %v", r, err)
	}
	if len(r.Coverage) == 0 || !strings.Contains(r.Coverage[len(r.Coverage)-1].Summary, "abort") {
		t.Fatal("abort was not disclosed")
	}
}

func TestSelectedUsageKeepsSessionScopeAndExcludesEarlierSnapshots(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/sessions/codex.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	base := strings.Replace(string(b), "0.163.0", "0.142.0-alpha.1", 1)
	usage := strings.Split(strings.TrimSpace(usageTrace(t)), "\n")[8]
	lines := strings.Split(strings.TrimSpace(base), "\n")
	// A synthetic larger earlier snapshot would poison selected usage if it
	// were carried into this turn's collector.
	earlier := strings.ReplaceAll(usage, "1467546", "9999999")
	input := strings.Join(append(append([]string{}, lines[:6]...), append([]string{earlier}, lines[6:]...)...), "\n")
	r, _, _, err := ParseTurn(strings.NewReader(input), "second")
	if err != nil || len(r.ModelUsage) != 0 {
		t.Fatalf("earlier usage leaked: %+v %v", r.ModelUsage, err)
	}
	input = strings.Replace(input, lines[12], usage+"\n"+lines[12], 1)
	r, _, _, err = ParseTurn(strings.NewReader(input), "second")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ModelUsage) != 1 || r.ModelUsage[0].Scope != "session cumulative snapshot" || r.ModelUsage[0].Unavailable != "" || *r.ModelUsage[0].Counters[0].Tokens != 1467546 {
		t.Fatalf("selected cumulative scope changed: %+v", r.ModelUsage)
	}
}
