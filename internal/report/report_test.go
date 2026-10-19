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
