package claude

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func usageTrace(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../fixtures/usage-evidence/claude-usage.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRecordedMessageUsage(t *testing.T) {
	r, err := Parse(strings.NewReader(usageTrace(t)))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ModelUsage) != 2 {
		t.Fatalf("entries %+v", r.ModelUsage)
	}
	first := r.ModelUsage[0]
	if first.Scope != "model-message snapshot" || first.SourceLine != 2 || first.Unavailable != "" {
		t.Fatalf("entry %+v", first)
	}
	for i, want := range []int64{2, 81, 61605, 0} {
		if first.Counters[i].Tokens == nil || *first.Counters[i].Tokens != want {
			t.Fatalf("counter %d", i)
		}
	}
}

func TestUsageMessageDuplicateAndConflict(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(usageTrace(t)), "\n")
	// Synthetic supported intermediate text carrying the same verified snapshot;
	// actual duplicate evidence is documented, but its unsupported lifecycle is not imported.
	var duplicate map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &duplicate); err != nil {
		t.Fatal(err)
	}
	message := duplicate["message"].(map[string]any)
	message["content"] = []any{map[string]any{"type": "text", "text": "Recorded commentary."}}
	delete(message, "stop_reason")
	for _, conflict := range []bool{false, true} {
		if conflict {
			message["usage"].(map[string]any)["input_tokens"] = float64(3)
		}
		b, err := json.Marshal(duplicate)
		if err != nil {
			t.Fatal(err)
		}
		trace := strings.Join(append(append([]string{}, lines[:3]...), string(b), lines[3]), "\n")
		r, err := Parse(strings.NewReader(trace))
		if err != nil {
			t.Fatal(err)
		}
		if len(r.ModelUsage) != 2 {
			t.Fatalf("duplicate added a charge: %+v", r.ModelUsage)
		}
		if (r.ModelUsage[0].Unavailable != "") != conflict {
			t.Fatalf("conflict=%v entry=%+v", conflict, r.ModelUsage[0])
		}
	}
}

func TestUsageOptionalFieldsCannotBreakImport(t *testing.T) {
	base := usageTrace(t)
	for _, tc := range []struct {
		name, old, replacement string
		unavailable            bool
		nilCounter             bool
	}{
		{"version", `"version": "2.1.302"`, `"version": "future"`, true, false},
		{"numeric version", `"version": "2.1.302"`, `"version": 42`, true, false},
		{"missing id", `"id": "model-message-1", `, "", true, false},
		{"malformed id", `"id": "model-message-1"`, `"id": []`, true, false},
		{"missing input", `"input_tokens": 2, `, "", false, true},
		{"null input", `"input_tokens": 2`, `"input_tokens": null`, false, true},
		{"negative input", `"input_tokens": 2`, `"input_tokens": -1`, false, true},
		{"fractional input", `"input_tokens": 2`, `"input_tokens": 1.2`, false, true},
		{"overflow input", `"input_tokens": 2`, `"input_tokens": 9223372036854775808`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Version occurs on user and assistant; change all so the selected message is affected.
			trace := strings.ReplaceAll(base, tc.old, tc.replacement)
			if trace == base {
				t.Fatal("mutation did not apply")
			}
			r, err := Parse(strings.NewReader(trace))
			if err != nil {
				t.Fatal(err)
			}
			if len(r.ModelUsage) != 2 {
				t.Fatal("usage entries lost")
			}
			first := r.ModelUsage[0]
			if (first.Unavailable != "") != tc.unavailable {
				t.Fatalf("entry %+v", first)
			}
			if tc.nilCounter && first.Counters[0].Tokens != nil {
				t.Fatal("invalid value became count")
			}
		})
	}
}
