package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func measurements(attempts, matched, missing int) string {
	return fmt.Sprintf("Acquisition measurements:\n- Attempts: %d\n- Matched observations: %d\n- Missing observations: %d\n\n", attempts, matched, missing)
}

// The second read is a synthetic mutation of the sanitized M2 recording: a
// request without a recorded result, not additional vendor evidence.
func TestMeasureCodexObservedAndPendingReads(t *testing.T) {
	binary := buildBinary(t)
	b, err := os.ReadFile("../fixtures/codex-acquisition.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	pending := strings.NewReplacer("item-1", "item-2", "call-1", "call-2", "cat README.md", "cat GUIDE.md").Replace(lines[2])
	input := append(append(append([]string{}, lines[:4]...), pending), lines[4:]...)
	for _, mode := range []string{"auto", "codex"} {
		t.Run(mode, func(t *testing.T) {
			got := inspectRepeatedInput(t, binary, mode, input)
			want := "Task:\nRead README.md and report the project name.\n\n" +
				"Context events:\n1. Acquisition a1: README.md\n2. Observation for a1:\n# Example\n3. Acquisition a2: GUIDE.md\n" +
				"Diagnostic: Observation missing for a2\n\n" +
				measurements(2, 1, 1) +
				"Final response:\nThe project is Example.\n\nExecution state: COMPLETED\nCompletion describes execution termination, not verified task success.\n"
			if got != want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// Synthetic combination of the recorded Claude Code image-read and tool-error
// result shapes; both are observations, neither is a verified outcome.
func TestMeasureClaudeImageAndErrorObservations(t *testing.T) {
	binary := buildBinary(t)
	input := []string{
		`{"type": "user", "sessionId": "session-1", "version": "2.1.302", "message": {"role": "user", "content": "Read picture.png and MISSING.md."}}`,
		`{"type": "assistant", "sessionId": "session-1", "version": "2.1.302", "message": {"role": "assistant", "stop_reason": "tool_use", "content": [{"type": "tool_use", "id": "call-1", "name": "Read", "input": {"file_path": "picture.png"}}]}}`,
		`{"type": "user", "sessionId": "session-1", "version": "2.1.302", "message": {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "call-1", "content": [{"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAAEElEQVR4nGL6z8AACAAA//8DCQECWLbVUAAAAABJRU5ErkJggg=="}}]}]}}`,
		`{"type": "assistant", "sessionId": "session-1", "version": "2.1.302", "message": {"role": "assistant", "stop_reason": "tool_use", "content": [{"type": "tool_use", "id": "call-2", "name": "Read", "input": {"file_path": "MISSING.md"}}]}}`,
		`{"type": "user", "sessionId": "session-1", "version": "2.1.302", "message": {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "call-2", "is_error": true, "content": "File does not exist."}]}}`,
		`{"type": "assistant", "sessionId": "session-1", "version": "2.1.302", "message": {"role": "assistant", "stop_reason": "end_turn", "content": [{"type": "text", "text": "Read the image; MISSING.md does not exist."}]}}`,
	}
	for _, mode := range []string{"auto", "claude-code"} {
		t.Run(mode, func(t *testing.T) {
			got := inspectRepeatedInput(t, binary, mode, input)
			for _, want := range []string{
				"2. Observation for a1:\nImage: image/png, 73 bytes (content not interpreted)\n",
				"4. Observation for a2:\nFile does not exist.\n",
				"\n\n" + measurements(2, 2, 0) + "Final response:\n",
				"Execution state: COMPLETED\nCompletion describes execution termination, not verified task success.\n",
			} {
				if !strings.Contains(got, want) {
					t.Fatalf("missing %q:\n%s", want, got)
				}
			}
			if strings.Contains(got, "Diagnostic:") {
				t.Fatal(got)
			}
		})
	}
}

// Reuses the synthetic three-repetition construction from repeated_test.go:
// three reads of one target interleaved with three actions on it.
func TestMeasureInterleavedRepeatedReadsAndActions(t *testing.T) {
	binary := buildBinary(t)
	for _, vendor := range []string{"codex", "claude"} {
		b, err := os.ReadFile("../fixtures/" + vendor + "-action.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		start, format := 2, "codex"
		if vendor == "claude" {
			start, format = 1, "claude-code"
		}
		var events []string
		for i := 1; i <= 3; i++ {
			for _, line := range lines[start : start+4] {
				events = append(events, strings.NewReplacer("read-1", fmt.Sprintf("read-%d", i), "edit-1", fmt.Sprintf("edit-%d", i), "GUIDE.md", "README.md").Replace(line))
			}
		}
		t.Run(vendor+"/all feedback", func(t *testing.T) {
			input := append(append(append([]string{}, lines[:start]...), events...), lines[start+4:]...)
			got := inspectRepeatedInput(t, binary, format, input)
			assertThreeSequences(t, got)
			if !strings.Contains(got, "\n\n"+measurements(3, 3, 0)+"Final response:\n") {
				t.Fatal(got)
			}
		})
		t.Run(vendor+"/missing feedback", func(t *testing.T) {
			// Drop the second observation and second action result: only the
			// acquisition side may change the counts.
			input := append([]string{}, lines[:start]...)
			for i, line := range events {
				if i != 5 && i != 7 {
					input = append(input, line)
				}
			}
			input = append(input, lines[start+4:]...)
			got := inspectRepeatedInput(t, binary, format, input)
			if !strings.Contains(got, "Diagnostic: Result missing for act2\nDiagnostic: Observation missing for a2\n\n"+measurements(3, 2, 1)+"Final response:\n") {
				t.Fatal(got)
			}
		})
	}
}

func TestNoToolReportHasNoMeasurements(t *testing.T) {
	binary := buildBinary(t)
	want, err := os.ReadFile("../fixtures/first-run.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, err := exec.Command(binary, "inspect", filepath.Join("..", "fixtures", "first-run.jsonl")).CombinedOutput()
	if err != nil || string(got) != string(want) || strings.Contains(string(got), "measurements") {
		t.Fatalf("%v\ngot:\n%s\nwant:\n%s", err, got, want)
	}
}
