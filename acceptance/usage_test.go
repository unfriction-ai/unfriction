package acceptance_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestInspectRecordedModelUsage(t *testing.T) {
	binary := buildBinary(t)
	for _, tc := range []struct {
		file string
		want []string
	}{
		{"codex-usage.jsonl", []string{"Recorded model usage:", "session cumulative snapshot", "input_tokens: 1467546 tokens", "cached_input_tokens: 1223680 tokens"}},
		{"claude-usage.jsonl", []string{"Recorded model usage:", "model-message snapshot", "input_tokens: 2 tokens", "cache_read_input_tokens: 0 tokens", "cache_creation_input_tokens: 672 tokens"}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			output, err := exec.Command(binary, "inspect", "../fixtures/usage-evidence/"+tc.file).CombinedOutput()
			if err != nil {
				t.Fatalf("%v: %s", err, output)
			}
			got := string(output)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q:\n%s", want, got)
				}
			}
			if !strings.Contains(got, "not acquisition-only") {
				t.Error("usage attribution limit missing")
			}
		})
	}
	t.Run("absent usage preserves no-tool output", func(t *testing.T) {
		want, err := os.ReadFile("../fixtures/first-run.txt")
		if err != nil {
			t.Fatal(err)
		}
		got, err := exec.Command(binary, "inspect", "../fixtures/first-run.jsonl").CombinedOutput()
		if err != nil || string(got) != string(want) {
			t.Fatalf("%v: %s", err, got)
		}
	})
}

// Synthetic mutations test snapshot handling without adding an import variant.
func TestInspectUsageSnapshotsAndUnavailable(t *testing.T) {
	binary := buildBinary(t)
	b, err := os.ReadFile("../fixtures/usage-evidence/codex-usage.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	snapshot := lines[len(lines)-2]
	duplicated := append(append([]string{}, lines[:len(lines)-1]...), snapshot, lines[len(lines)-1])
	got := inspectRepeatedInput(t, binary, "codex", duplicated)
	if strings.Count(got, "- input_tokens: 1467546 tokens") != 1 {
		t.Fatalf("cumulative snapshot was summed or repeated: %s", got)
	}
	unknown := strings.Split(strings.ReplaceAll(string(b), "0.142.0-alpha.1", "future"), "\n")
	got = inspectRepeatedInput(t, binary, "codex", unknown)
	if !strings.Contains(got, "unavailable") || strings.Contains(got, "1467546 tokens") {
		t.Fatalf("unknown version trusted: %s", got)
	}
	partial := strings.Split(strings.Replace(string(b), `"input_tokens": 1467546`, `"input_tokens": null`, 1), "\n")
	got = inspectRepeatedInput(t, binary, "codex", partial)
	if !strings.Contains(got, "- input_tokens: unavailable") || !strings.Contains(got, "- total_tokens: unavailable") {
		t.Fatalf("missing evidence became numeric: %s", got)
	}
	if strings.Contains(got, "model-message-1") {
		t.Fatal("raw source identity rendered")
	}
}
