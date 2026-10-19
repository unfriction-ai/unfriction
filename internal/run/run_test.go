package run

import (
	"reflect"
	"testing"
)

func TestAssociationPreservesArrivalOrder(t *testing.T) {
	r := Run{Task: "Read two files", ExecutionState: Incomplete}
	for _, call := range []struct{ id, target string }{{"second", "a.md"}, {"first", "b.md"}} {
		if err := r.Acquire(call.id, call.target, len(r.Events)+1); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Observe("first", "B", 3); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.MissingObservations(), []string{"a1"}) {
		t.Fatal(r.MissingObservations())
	}
	if err := r.Observe("second", "A", 4); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range r.Events {
		ids = append(ids, e.AcquisitionID)
	}
	if !reflect.DeepEqual(ids, []string{"a1", "a2", "a2", "a1"}) || len(r.MissingObservations()) != 0 {
		t.Fatalf("%+v", r.Events)
	}
	if r.Events[2].SourceID != "first" || r.Events[2].SourceLine != 3 {
		t.Fatal("lost provenance")
	}
}

func TestRejectInvalidEventsWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*Run)
		apply   func(*Run) error
	}{
		{"before task", func(r *Run) { r.Task = "" }, func(r *Run) error { return r.Acquire("id", "a.md", 1) }},
		{"empty target", func(r *Run) {}, func(r *Run) error { return r.Acquire("id", " ", 1) }},
		{"empty ID", func(r *Run) {}, func(r *Run) error { return r.Acquire("", "a.md", 1) }},
		{"orphan", func(r *Run) {}, func(r *Run) error { return r.Observe("unknown", "text", 1) }},
		{"after final", func(r *Run) { r.FinalResponse = "done" }, func(r *Run) error { return r.Acquire("id", "a.md", 1) }},
		{"after completion", func(r *Run) { r.ExecutionState = Completed }, func(r *Run) error { return r.Observe("id", "text", 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Run{Task: "task", ExecutionState: Incomplete}
			tc.prepare(&r)
			if err := tc.apply(&r); err == nil || len(r.Events) != 0 {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
}
