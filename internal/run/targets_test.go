package run

import "testing"

func TestLiteralAcquisitionTargets(t *testing.T) {
	for _, tc := range []struct {
		name    string
		targets []string
		want    AcquisitionTargets
	}{
		{"empty", nil, AcquisitionTargets{}},
		{"one", []string{"src/file.go"}, AcquisitionTargets{1, 0}},
		{"repeated", []string{"A", "A", "B", "A"}, AcquisitionTargets{2, 2}},
		{"no path cleaning", []string{"src/file.go", "./src/file.go", "src/../src/file.go"}, AcquisitionTargets{3, 0}},
		{"no case folding", []string{"README.md", "readme.md"}, AcquisitionTargets{2, 0}},
		{"no whitespace trimming", []string{"file", " file", "file "}, AcquisitionTargets{3, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Run{Task: "inspect"}
			for i, target := range tc.targets {
				id := string(rune('a' + i))
				if err := r.Acquire(id, target, i+1); err != nil {
					t.Fatal(err)
				}
				// Alternate pending reads and tool-reported errors around unrelated actions.
				if i%2 == 0 {
					if err := r.RecordResult(id, "", true, i+2); err != nil {
						t.Fatal(err)
					}
				}
				if err := r.Act("action-"+id, "action-only", i+3); err != nil {
					t.Fatal(err)
				}
			}
			before := append([]Event(nil), r.Events...)
			got := r.AcquisitionTargets()
			if got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
			if got.DistinctTargets < 0 || got.RepeatedRequests < 0 || r.AcquisitionFrequency().Attempts != got.DistinctTargets+got.RepeatedRequests {
				t.Fatal("target count identity violated")
			}
			if len(before) != len(r.Events) {
				t.Fatal("measurement changed event count")
			}
			for i, e := range before {
				if e != r.Events[i] {
					t.Fatal("measurement changed recorded events")
				}
			}
		})
	}
}

func TestActionsCannotCreateAcquisitionRepetition(t *testing.T) {
	r := Run{Task: "inspect"}
	if err := r.Acquire("read", "A", 1); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"edit-1", "edit-2", "edit-3"} {
		if err := r.Act(id, "A", 2); err != nil {
			t.Fatal(err)
		}
	}
	if got := r.AcquisitionTargets(); got != (AcquisitionTargets{1, 0}) {
		t.Fatalf("actions counted: %+v", got)
	}
	r.Events = r.Events[1:]
	if got := r.AcquisitionTargets(); got != (AcquisitionTargets{}) {
		t.Fatalf("action-only run counted: %+v", got)
	}
}
