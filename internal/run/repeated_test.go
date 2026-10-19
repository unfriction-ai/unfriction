package run

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRepeatedRequestsKeepIdentityWhenFeedbackArrivesLater(t *testing.T) {
	r := Run{Task: "revise one file", ExecutionState: Incomplete}
	// Three reads and three actions to one path, without requiring alternation
	// or feedback before the next request. Results arrive in a different order.
	for i := 1; i <= 3; i++ {
		if err := r.Acquire(fmt.Sprintf("read-%d", i), "README.md", i); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 3; i++ {
		if err := r.Act(fmt.Sprintf("edit-%d", i), "README.md", i+3); err != nil {
			t.Fatal(err)
		}
	}
	for i, id := range []string{"edit-2", "read-3", "edit-1", "read-2"} {
		if err := r.RecordResult(id, id, id == "edit-2", i+7); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(r.MissingObservations(), []string{"a1"}) || !reflect.DeepEqual(r.MissingResults(), []string{"act3"}) {
		t.Fatalf("missing observations=%v results=%v", r.MissingObservations(), r.MissingResults())
	}
	for i, want := range []struct {
		source, local string
		kind          EventKind
		isError       bool
	}{
		{"edit-2", "act2", Result, true}, {"read-3", "a3", Observation, false}, {"edit-1", "act1", Result, false}, {"read-2", "a2", Observation, false},
	} {
		got := r.Events[i+6]
		local := got.ActionID
		if got.Kind == Observation {
			local = got.AcquisitionID
		}
		if got.SourceID != want.source || local != want.local || got.Kind != want.kind || got.IsError != want.isError || got.SourceLine != i+7 || got.Text != want.source {
			t.Fatalf("event %d: %+v", i, got)
		}
	}
	if r.ExecutionState != Incomplete {
		t.Fatal("tool feedback changed termination")
	}
	for _, tc := range []struct {
		name  string
		apply func() error
	}{
		{"duplicate acquisition", func() error { return r.Acquire("read-2", "OTHER.md", 11) }},
		{"duplicate action", func() error { return r.Act("edit-3", "OTHER.md", 11) }},
		{"acquisition reuses action ID", func() error { return r.Acquire("edit-1", "OTHER.md", 11) }},
		{"action reuses acquisition ID", func() error { return r.Act("read-3", "OTHER.md", 11) }},
		{"duplicate observation", func() error { return r.RecordResult("read-2", "again", false, 11) }},
		{"duplicate result", func() error { return r.RecordResult("edit-2", "again", false, 11) }},
		{"orphan result", func() error { return r.RecordResult("unknown", "unmatched", false, 11) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := append([]Event(nil), r.Events...)
			err := tc.apply()
			if err == nil || !reflect.DeepEqual(before, r.Events) {
				t.Fatalf("err=%v events=%+v", err, r.Events)
			}
			if !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "unmatched") {
				t.Fatal(err)
			}
		})
	}
	if err := r.RecordResult("read-1", "late context", false, 11); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordResult("edit-3", "late result", false, 12); err != nil {
		t.Fatal(err)
	}
	if len(r.MissingObservations()) != 0 || len(r.MissingResults()) != 0 || len(r.Events) != 12 {
		t.Fatalf("%+v", r)
	}
}
