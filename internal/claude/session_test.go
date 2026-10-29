package claude

import (
	"os"
	"strings"
	"testing"
)

func TestSelectedTurnPreservesPhysicalEvents(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/sessions/claude.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	r, d, i, err := ParseTurn(strings.NewReader(string(b)), "second")
	if err != nil {
		t.Fatal(err)
	}
	if i != 1 || len(d.Turns) != 2 || r.Task != "Go" || len(r.Events) != 2 || r.Events[0].SourceLine != 6 || r.Events[1].SourceLine != 7 || r.Events[0].SourceID != "selected-read" {
		t.Fatalf("incorrect isolation: %+v, %+v", r, d)
	}
}

func TestSelectedTurnRejectsUnsafeAncestry(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/sessions/claude.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	base := string(b)
	for _, tc := range []struct{ name, input string }{
		{"missing parent", strings.Replace(base, `"parentUuid":"old-final"`, `"parentUuid":"missing"`, 1)},
		{"cycle", strings.Replace(base, `"parentUuid":null`, `"parentUuid":"selected-final"`, 1)},
		{"duplicate node", strings.Replace(base, `"uuid":"selected-call"`, `"uuid":"second"`, 1)},
		{"cross-turn result", strings.Replace(base, `"tool_use_id":"selected-read"`, `"tool_use_id":"old-read"`, 1)},
		{"result reparented across turns", strings.Replace(base, `"parentUuid":"selected-call"`, `"parentUuid":"old-call"`, 1)},
		{"competing assistant", strings.Replace(base, `{"type":"user","uuid":"selected-result"`, `{"type":"assistant","uuid":"branch","parentUuid":"second","sessionId":"session-claude","message":{"role":"assistant","content":[{"type":"text","text":"alternative"}]}}`+"\n"+`{"type":"user","uuid":"selected-result"`, 1)},
		{"sidechain ancestor", strings.Replace(base, `"uuid":"first"`, `"uuid":"first","isSidechain":true`, 1)},
		{"foreign session", strings.Replace(base, `"sessionId":"session-claude"`, `"sessionId":"foreign"`, 1)},
		{"branch behind system node", strings.Replace(base, `{"type":"user","uuid":"selected-result"`, `{"type":"system","subtype":"turn_duration","uuid":"bridge","parentUuid":"second","sessionId":"session-claude"}`+"\n"+`{"type":"assistant","uuid":"branch","parentUuid":"bridge","sessionId":"session-claude","message":{"role":"assistant","content":[{"type":"text","text":"alternative"}]}}`+"\n"+`{"type":"user","uuid":"selected-result"`, 1)},
		{"unknown record", base + "\n" + `{"type":"unknown"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, _, err := ParseTurn(strings.NewReader(tc.input), "second")
			if err == nil || r.Task != "" {
				t.Fatalf("expected no report, got %+v %v", r, err)
			}
		})
	}
}

func TestSelectedTurnIgnoresMalformedOptionalVersion(t *testing.T) {
	b, _ := os.ReadFile("../../fixtures/sessions/claude.jsonl")
	input := strings.Replace(string(b), `"version":"2.1.302"`, `"version":123`, 1)
	if _, _, _, err := ParseTurn(strings.NewReader(input), "first"); err != nil {
		t.Fatal(err)
	}
}

func TestSessionRejectsMessageBlocksAcrossTurns(t *testing.T) {
	input := `{"type":"user","uuid":"u1","parentUuid":null,"sessionId":"s","message":{"role":"user","content":"First"}}
{"type":"assistant","uuid":"a1","parentUuid":"u1","sessionId":"s","requestId":"r","apiBlockIndex":0,"message":{"id":"m","role":"assistant","content":[{"type":"thinking","thinking":"reasoning"}],"stop_reason":"end_turn"}}
{"type":"user","uuid":"u2","parentUuid":"a1","sessionId":"s","message":{"role":"user","content":"Second"}}
{"type":"assistant","uuid":"a2","parentUuid":"u2","sessionId":"s","requestId":"r","apiBlockIndex":1,"message":{"id":"m","role":"assistant","content":[{"type":"text","text":"First answer"}],"stop_reason":"end_turn"}}`
	if _, _, _, err := ParseTurn(strings.NewReader(input), "u2"); err == nil {
		t.Fatal("message blocks crossed user turns")
	}
}

func TestSessionRetainsNonAdjacentToolResult(t *testing.T) {
	b, _ := os.ReadFile("../../fixtures/sessions/claude.jsonl")
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	request := `{"type":"assistant","uuid":"other-call","parentUuid":"selected-call","sessionId":"session-claude","message":{"role":"assistant","content":[{"type":"tool_use","id":"other-read","name":"Read","input":{"file_path":"other.md"}}],"stop_reason":"tool_use"}}`
	result := `{"type":"user","uuid":"other-result","parentUuid":"other-call","sessionId":"session-claude","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"other-read","content":"OTHER"}]}}`
	input := strings.Join(append(append(append([]string{}, lines[:6]...), request, lines[6], result), strings.Replace(lines[7], `"parentUuid":"selected-result"`, `"parentUuid":"other-result"`, 1)), "\n")
	r, _, _, err := ParseTurn(strings.NewReader(input), "second")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 4 || r.AcquisitionFrequency().Attempts != 2 || r.Events[2].SourceID != "selected-read" || r.Events[2].SourceLine != 8 {
		t.Fatalf("lost sibling result: %+v", r)
	}
}

func TestSessionRejectsConflictingMessageBlockIndex(t *testing.T) {
	b, _ := os.ReadFile("../../fixtures/sessions/claude-current.jsonl")
	lines := strings.Split(string(b), "\n")
	lines[26] = strings.Replace(lines[26], `"apiBlockIndex":1`, `"apiBlockIndex":0`, 1)
	_, _, _, err := ParseTurn(strings.NewReader(strings.Join(lines, "\n")), "redacted-14")
	if err == nil {
		t.Fatal("duplicate block index accepted")
	}
}

func TestSessionAccountsForBookkeeping(t *testing.T) {
	b, _ := os.ReadFile("../../fixtures/sessions/claude-current.jsonl")
	r, _, _, err := ParseTurn(strings.NewReader(string(b)), "redacted-14")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []int{8, 21, 25, 36, 37} {
		found := false
		for _, c := range r.Coverage {
			found = found || c.SourceLine == line
		}
		if !found {
			t.Errorf("missing coverage for source line %d", line)
		}
	}
}

func TestSelectedUsageExcludesEarlierMessages(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/sessions/claude.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	input := strings.Replace(string(b), `"uuid":"old-final"`, `"uuid":"old-final","version":"2.1.302"`, 1)
	input = strings.Replace(input, `"role":"assistant","content":[{"type":"text"`, `"role":"assistant","id":"earlier-message","usage":{"input_tokens":2,"output_tokens":225,"cache_creation_input_tokens":672,"cache_read_input_tokens":61605},"content":[{"type":"text"`, 1)
	first, _, _, err := ParseTurn(strings.NewReader(input), "first")
	if err != nil || len(first.ModelUsage) != 1 {
		t.Fatalf("missing baseline usage: %+v %v", first.ModelUsage, err)
	}
	second, _, _, err := ParseTurn(strings.NewReader(input), "second")
	if err != nil || len(second.ModelUsage) != 0 {
		t.Fatalf("earlier message usage leaked: %+v %v", second.ModelUsage, err)
	}
}
