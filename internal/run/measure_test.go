package run

import "testing"

func TestAcquisitionCounts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*Run) error
		want  AcquisitionCounts
	}{
		{"no acquisitions", func(r *Run) error { return nil }, AcquisitionCounts{}},
		{"missing", func(r *Run) error { return r.Acquire("read", "README.md", 2) }, AcquisitionCounts{1, 0, 1}},
		{"matched empty", func(r *Run) error {
			if err := r.Acquire("read", "README.md", 2); err != nil {
				return err
			}
			return r.Observe("read", "", 3)
		}, AcquisitionCounts{1, 1, 0}},
		{"image", func(r *Run) error {
			if err := r.Acquire("read", "picture.png", 2); err != nil {
				return err
			}
			return r.ObserveImage("read", ImageObservation{MediaType: "image/png", ByteLength: 70}, false, 3)
		}, AcquisitionCounts{1, 1, 0}},
		{"tool error", func(r *Run) error {
			if err := r.Acquire("read", "MISSING.md", 2); err != nil {
				return err
			}
			return r.RecordResult("read", "File does not exist.", true, 3)
		}, AcquisitionCounts{1, 1, 0}},
		{"interleaved actions and repeated target", func(r *Run) error {
			for _, step := range []func() error{
				func() error { return r.Acquire("read-1", "README.md", 2) },
				func() error { return r.Act("edit-1", "README.md", 3) },
				func() error { return r.Acquire("read-2", "README.md", 4) },
				func() error { return r.Create("create-1", "NEW.md", 5) },
				func() error { return r.RecordResult("edit-1", "done", false, 6) },
				func() error { return r.RecordResult("read-2", "# Example", false, 7) },
				func() error { return r.RecordResult("create-1", "created", false, 8) },
			} {
				if err := step(); err != nil {
					return err
				}
			}
			return nil
		}, AcquisitionCounts{2, 1, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Run{Task: "task", ExecutionState: Incomplete}
			if err := tc.build(&r); err != nil {
				t.Fatal(err)
			}
			got := r.AcquisitionCounts()
			if got != tc.want || got.Attempts != got.Matched+got.Missing || got.Missing != len(r.MissingObservations()) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
