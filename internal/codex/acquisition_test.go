package codex

import (
	"os"
	"strings"
	"testing"
)

func TestAcquisition(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/codex-acquisition.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	fixture := string(b)
	r, err := Parse(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 2 || r.Events[0].Target != "README.md" || r.Events[1].AcquisitionID != r.Events[0].AcquisitionID || r.Events[1].Text != "# Example\n" {
		t.Fatalf("events: %+v", r.Events)
	}
	lines := strings.Split(strings.TrimSpace(fixture), "\n")
	for _, tc := range []struct{ name, input, want string }{
		{"orphan", strings.Join(append(append([]string{}, lines[:2]...), lines[3:]...), "\n"), "unmatched observation"},
		{"duplicate request", strings.Join(append(append([]string{}, lines[:3]...), lines[2:]...), "\n"), "duplicate acquisition"},
		{"duplicate result", strings.Join(append(append([]string{}, lines[:4]...), lines[3:]...), "\n"), "duplicate observation"},
		{"unsupported tool", strings.Replace(fixture, "exec_command", "unknown_tool", 1), "unsupported acquisition"},
		{"unsupported command", strings.Replace(fixture, "cat README.md", "cat README.md; pwd", 1), "unsupported acquisition"},
		{"missing ID", strings.ReplaceAll(fixture, `"call_id": "call-1"`, `"call_id": ""`), "missing acquisition ID"},
		{"missing output", strings.Replace(fixture, `"output":`, `"omitted":`, 1), "missing observation output"},
		{"unknown output envelope", strings.Replace(fixture, "Chunk ID:", "Unknown:", 1), "unsupported observation envelope"},
		{"null output", strings.Join(lines[:3], "\n") + "\n" + `{"type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":null}}`, "missing observation output"},
		{"after completion", fixture + lines[2] + "\n", "outside acquisition phase"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.input))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
	for _, completed := range []bool{false, true} {
		input := strings.Join(lines[:3], "\n")
		if completed {
			input += "\n" + strings.Join(lines[4:], "\n")
		}
		r, err := Parse(strings.NewReader(input))
		if err != nil || len(r.MissingObservations()) != 1 {
			t.Fatalf("missing observation: %+v %v", r, err)
		}
	}
}

func TestUnsupportedEventsAreNotSilentlyIgnored(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/codex-acquisition.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	for _, event := range []string{
		`{"type":"response_item","payload":{"type":"future_tool"}}`,
		`{"type":"event_msg","payload":{"type":"turn_aborted"}}`,
		`{"type":"unknown","payload":{}}`,
	} {
		input := strings.Join(lines[:2], "\n") + "\n" + event + "\n" + strings.Join(lines[2:], "\n")
		if _, err := Parse(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %s", event)
		}
	}
}
