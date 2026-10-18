package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const succeed = "\u2713"
const failed = "\u2717"

// Scenario: Import a completed Codex run without acquisitions.
// Build a fresh application binary: this test must exercise the public CLI,
// real input parsing, run reconstruction, and report generation together.
func TestImportCompletedCodexRunWithoutAcquisitions(t *testing.T) {
	binary := buildBinary(t)

	t.Log("Given a sanitized recorded Codex run containing a task.")
	t.Log("And the agent completed without using tools.")
	fixture := filepath.Join("..", "fixtures", "first-run.jsonl")
	want := "Task:\nReply with the word ready.\n\nFinal response:\nready\n\nExecution state: COMPLETED\nCompletion describes execution termination, not verified task success.\n"
	for i := range 2 {
		t.Logf("\tRun %d:\tWhen I import the run with 'unfriction inspect'.", i+1)
		output, err := exec.Command(binary, "inspect", fixture).CombinedOutput()
		if err != nil {
			t.Fatalf("\t%s\tThen it should execute without error, but got %v\n%s", failed, err, output)
		}
		t.Logf("\t%s\tThen it should execute without error.", succeed)
		if string(output) != want {
			t.Fatalf("\t%s\tThen it should show the original task, final response, and COMPLETED execution state.\ngot:  %q\nwant: %q", failed, output, want)
		}
		t.Logf("\t%s\tThen it should show the original task, final response, and COMPLETED execution state.", succeed)
	}
	t.Logf("\t%s\tThen repeated imports should produce identical reports.", succeed)

	trace, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(trace)), "\n")
	t.Run("final response alone is incomplete", func(t *testing.T) {
		t.Log("Given a recorded run with a final response but no completion event.")
		path := filepath.Join(t.TempDir(), "incomplete.jsonl")
		if err := os.WriteFile(path, []byte(strings.Join(lines[:len(lines)-1], "\n")), 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("\tWhen I import the run with 'unfriction inspect'.")
		output, err := exec.Command(binary, "inspect", path).CombinedOutput()
		if err != nil || string(output) != strings.Replace(want, "COMPLETED", "INCOMPLETE", 1) {
			t.Fatalf("\t%s\tThen it should report INCOMPLETE without error, but got %v\n%s", failed, err, output)
		}
		t.Logf("\t%s\tThen it should show the task and final response with execution state INCOMPLETE.", succeed)
	})

	for _, tc := range []struct{ name, input, diagnostic string }{
		{"malformed JSON", "{", "invalid event"},
		{"empty trace", "", "does not contain a task"},
		{"multiple turns", string(trace) + string(trace), "multiple turns"},
		{"wrong completion ID", strings.Join(lines[:3], "\n") + "\n" + strings.ReplaceAll(lines[3], "00000000-0000-4000-8000-000000000001", "wrong-turn"), "completion must match"},
		{"conflicting final response", strings.Join(lines[:3], "\n") + "\n" + strings.ReplaceAll(lines[3], "ready", "different"), "completion must match"},
		{"tool call", strings.Join(lines[:2], "\n") + "\n" + `{"type":"response_item","payload":{"type":"function_call"}}`, "tool calls are not supported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Given an invalid or unsupported trace: %s.", tc.name)
			path := filepath.Join(t.TempDir(), "invalid.jsonl")
			if err := os.WriteFile(path, []byte(tc.input), 0600); err != nil {
				t.Fatal(err)
			}
			t.Log("\tWhen I import the trace with 'unfriction inspect'.")
			output, err := exec.Command(binary, "inspect", path).CombinedOutput()
			if err == nil || !strings.Contains(string(output), tc.diagnostic) || strings.Contains(string(output), "Execution state:") {
				t.Fatalf("\t%s\tThen it should fail with %q and no report, but got %v\n%s", failed, tc.diagnostic, err, output)
			}
			t.Logf("\t%s\tThen it should fail with %q and no report.", succeed, tc.diagnostic)
		})
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "unfriction")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", binary, "./cmd/unfriction")
	build.Dir = ".."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build application: %v\n%s", err, output)
	}
	return binary
}
