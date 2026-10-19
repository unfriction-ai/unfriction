package report

import (
	"bytes"
	"github.com/unfriction-ai/unfriction/internal/run"
	"strings"
	"testing"
)

func TestContextReport(t *testing.T) {
	r := run.Run{Task: "Read file", ExecutionState: run.Incomplete}
	if err := r.Acquire("vendor-id", "README.md", 2); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := Write(&b, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "1. Acquisition a1: README.md") || !strings.Contains(b.String(), "Observation missing for a1") || strings.Contains(b.String(), "vendor-id") {
		t.Fatal(b.String())
	}
	if err := r.Observe("vendor-id", "", 3); err != nil {
		t.Fatal(err)
	}
	b.Reset()
	if err := Write(&b, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "2. Observation for a1:\n(empty)") || strings.Contains(b.String(), "Observation missing") {
		t.Fatal(b.String())
	}
}

func TestActionReportDoesNotInventResultsOrTaskOutcome(t *testing.T) {
	for _, state := range []run.ExecutionState{run.Incomplete, run.Completed} {
		r := run.Run{Task: "update", ExecutionState: run.Incomplete}
		if err := r.Act("private-id", "README.md", 2); err != nil {
			t.Fatal(err)
		}
		r.ExecutionState = state
		var b bytes.Buffer
		if err := Write(&b, r); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "Diagnostic: Result missing for act1") || !strings.Contains(b.String(), "Execution state: "+string(state)) || strings.Contains(b.String(), "private-id") {
			t.Fatal(b.String())
		}
		r.ExecutionState = run.Incomplete
		if err := r.RecordResult("private-id", "", true, 3); err != nil {
			t.Fatal(err)
		}
		r.ExecutionState = state
		b.Reset()
		if err := Write(&b, r); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"Result for act1:\n(empty)\nTool reported an error.", "Execution state: " + string(state), "not verified task success"} {
			if !strings.Contains(b.String(), want) {
				t.Fatal(b.String())
			}
		}
		if strings.Contains(b.String(), "Result missing") {
			t.Fatal(b.String())
		}
	}
}
