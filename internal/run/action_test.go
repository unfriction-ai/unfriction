package run

import (
	"reflect"
	"testing"
)

func TestActionAssociation(t *testing.T) {
	r := Run{Task: "update", ExecutionState: Incomplete}
	if err := r.Act("edit", "README.md", 2); err != nil {
		t.Fatal(err)
	}
	if err := r.Acquire("edit", "other", 3); err == nil {
		t.Fatal("accepted cross-kind duplicate")
	}
	if err := r.RecordResult("edit", "failed", true, 4); err != nil {
		t.Fatal(err)
	}
	if r.Events[1].Kind != Result || r.Events[1].ActionID != "act1" || !r.Events[1].IsError || r.ExecutionState != Incomplete {
		t.Fatalf("%+v", r)
	}
	if err := r.RecordResult("edit", "again", false, 5); err == nil {
		t.Fatal("accepted duplicate")
	}
	if err := r.RecordResult("unknown", "", false, 6); err == nil {
		t.Fatal("accepted orphan")
	}
}

func TestMixedRequestsPreserveResultArrivalOrder(t *testing.T) {
	r := Run{Task: "task", ExecutionState: Incomplete}
	for _, err := range []error{r.Acquire("read", "GUIDE.md", 2), r.Act("edit", "README.md", 3), r.RecordResult("edit", "done", false, 4), r.RecordResult("read", "context", false, 5)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	kinds := []EventKind{}
	for _, e := range r.Events {
		kinds = append(kinds, e.Kind)
	}
	if !reflect.DeepEqual(kinds, []EventKind{Acquisition, Action, Result, Observation}) || r.Events[2].SourceLine != 4 || r.Events[3].AcquisitionID != "a1" || r.Events[2].ActionID != "act1" {
		t.Fatalf("%+v", r.Events)
	}
	if len(r.MissingResults()) != 0 || len(r.MissingObservations()) != 0 {
		t.Fatal("unexpected missing result")
	}
}

func TestInvalidActionsDoNotMutateRun(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*Run)
		apply   func(*Run) error
	}{
		{"before task", func(r *Run) { r.Task = "" }, func(r *Run) error { return r.Act("edit", "README.md", 2) }},
		{"empty ID", func(r *Run) {}, func(r *Run) error { return r.Act("", "README.md", 2) }},
		{"empty target", func(r *Run) {}, func(r *Run) error { return r.Act("edit", " ", 2) }},
		{"after final", func(r *Run) { r.FinalResponse = "done" }, func(r *Run) error { return r.Act("edit", "README.md", 2) }},
		{"after completion", func(r *Run) { r.ExecutionState = Completed }, func(r *Run) error { return r.RecordResult("edit", "done", false, 2) }},
		{"duplicate", func(r *Run) { _ = r.Act("edit", "README.md", 2) }, func(r *Run) error { return r.Act("edit", "README.md", 3) }},
		{"read collision", func(r *Run) { _ = r.Acquire("edit", "README.md", 2) }, func(r *Run) error { return r.Act("edit", "README.md", 3) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Run{Task: "task", ExecutionState: Incomplete}
			tc.prepare(&r)
			before := append([]Event(nil), r.Events...)
			if err := tc.apply(&r); err == nil || !reflect.DeepEqual(before, r.Events) {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
}

func TestCreationAndImageEvidencePreserveAssociations(t *testing.T) {
	r := Run{Task: "read image and create file", ExecutionState: Incomplete}
	if err := r.Acquire("read", "picture.png", 2); err != nil {
		t.Fatal(err)
	}
	if err := r.ObserveImage("read", ImageObservation{MediaType: "image/png", ByteLength: 70}, false, 3); err != nil {
		t.Fatal(err)
	}
	if len(r.MissingObservations()) != 0 || r.Events[1].Text != "" || r.Events[1].Image == nil || r.Events[1].AcquisitionID != "a1" || r.Events[1].SourceLine != 3 {
		t.Fatalf("image not associated: %+v", r)
	}
	if err := r.ObserveImage("read", ImageObservation{MediaType: "image/png", ByteLength: 70}, false, 4); err == nil || len(r.Events) != 2 {
		t.Fatal("accepted duplicate image result")
	}
	if err := r.Create("create", "CREATED.md", 4); err != nil {
		t.Fatal(err)
	}
	if r.Events[2].Operation != CreateFile || len(r.MissingResults()) != 1 || r.ExecutionState != Incomplete {
		t.Fatalf("creation inferred an outcome: %+v", r)
	}
	if err := r.ObserveImage("create", ImageObservation{MediaType: "image/png", ByteLength: 70}, false, 5); err == nil || len(r.Events) != 3 {
		t.Fatal("accepted image as action result")
	}
	if err := r.RecordResult("create", "created", false, 5); err != nil {
		t.Fatal(err)
	}
	if r.Events[3].ActionID != r.Events[2].ActionID || r.ExecutionState != Incomplete {
		t.Fatalf("creation result inferred task outcome: %+v", r)
	}
}
