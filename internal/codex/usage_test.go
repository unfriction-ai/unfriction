package codex

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func usageTrace(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../fixtures/usage-evidence/codex-usage.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRecordedSessionUsageSnapshot(t *testing.T) {
	trace := usageTrace(t)
	r, err := Parse(strings.NewReader(trace))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ModelUsage) != 1 {
		t.Fatalf("entries %+v", r.ModelUsage)
	}
	entry := r.ModelUsage[0]
	if entry.Scope != "session cumulative snapshot" || entry.SourceLine != 9 || entry.Unavailable != "" {
		t.Fatalf("entry %+v", entry)
	}
	if got := entry.Counters[0].Tokens; got == nil || *got != 1467546 {
		t.Fatalf("latest input %+v", got)
	}
	lines := strings.Split(strings.TrimSpace(trace), "\n")
	snapshot := lines[len(lines)-2]
	duplicated := strings.Join(lines[:len(lines)-1], "\n") + "\n" + snapshot + "\n" + lines[len(lines)-1] + "\n"
	repeated, err := Parse(strings.NewReader(duplicated))
	if err != nil {
		t.Fatal(err)
	}
	if got := repeated.ModelUsage[0].Counters[0].Tokens; got == nil || *got != 1467546 {
		t.Fatal("duplicate snapshot added as usage")
	}
}

func TestUsageOptionalInvalidAndVersionBoundaries(t *testing.T) {
	base := usageTrace(t)
	for _, tc := range []struct {
		name, old, replacement string
		unavailable            bool
		unknownCounter         int
	}{
		{"unknown version", "0.142.0-alpha.1", "0.163.0", true, -1},
		{"malformed version", `"cli_version": "0.142.0-alpha.1"`, `"cli_version": 42`, true, -1},
		{"decrease", "1467546", "1", true, -1},
		{"inconsistent total", "1488274", "1488275", true, -1},
		{"cache outside input", "1223680", "9999999", true, -1},
		{"reasoning outside output", "8453", "9999999", true, -1},
		{"null counter", `"input_tokens": 1467546`, `"input_tokens": null`, false, 0},
		{"negative counter", `"input_tokens": 1467546`, `"input_tokens": -1`, false, 0},
		{"fractional counter", `"input_tokens": 1467546`, `"input_tokens": 1.5`, false, 0},
		{"overflow counter", `"input_tokens": 1467546`, `"input_tokens": 9223372036854775808`, false, 0},
		{"string counter", `"input_tokens": 1467546`, `"input_tokens": "1"`, false, 0},
		{"missing counter", `"input_tokens": 1467546, `, ``, false, 0},
		{"unknown category", `"input_tokens": 1467546`, `"new_counter": 1, "input_tokens": 1467546`, true, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trace := strings.Replace(base, tc.old, tc.replacement, 1)
			if trace == base {
				t.Fatal("mutation did not apply")
			}
			r, err := Parse(strings.NewReader(trace))
			if err != nil {
				t.Fatal(err)
			}
			entry := r.ModelUsage[0]
			if (entry.Unavailable != "") != tc.unavailable {
				t.Fatalf("entry %+v", entry)
			}
			if tc.unknownCounter >= 0 && entry.Counters[tc.unknownCounter].Tokens != nil {
				t.Fatal("untrusted count became a number")
			}
		})
	}
}

func TestUsageCannotEstablishLifecycleOrCrossItsBoundary(t *testing.T) {
	trace := usageTrace(t)
	lines := strings.Split(strings.TrimSpace(trace), "\n")
	snapshot := lines[len(lines)-2]
	// Usage before start and after completion is ignored, including malformed metadata.
	outside := lines[0] + "\n" + snapshot + "\n" + strings.Join(lines[1:], "\n") + "\n" + strings.Replace(snapshot, "1467546", "1", 1)
	r, err := Parse(strings.NewReader(outside))
	if err != nil {
		t.Fatal(err)
	}
	if r.ModelUsage[0].Unavailable != "" {
		t.Fatal("outside snapshot influenced active usage")
	}
	noCompletion := strings.Join(lines[:len(lines)-1], "\n")
	r, err = Parse(strings.NewReader(noCompletion))
	if err != nil {
		t.Fatal(err)
	}
	if string(r.ExecutionState) != "INCOMPLETE" {
		t.Fatal("usage inferred completion")
	}
	// A source saturation-shaped total is unavailable, not observed zero usage.
	var u usageCollector
	u.consume([]byte(`{"type":"session_meta","payload":{"cli_version":"0.142.0-alpha.1"}}`), false, 1)
	u.consume([]byte(fmt.Sprintf(`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0,"total_tokens":%d}}}}`, 258400)), true, 2)
	if u.entries()[0].Unavailable == "" {
		t.Fatal("saturation accepted")
	}
}
