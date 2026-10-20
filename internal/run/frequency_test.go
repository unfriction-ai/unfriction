package run

import "testing"

func TestAcquisitionFrequency(t *testing.T) {
	for _, tc := range []struct {
		name   string
		events []Event
		want   AcquisitionFrequency
	}{
		{"no acquisitions", nil, AcquisitionFrequency{}},
		{"action only", []Event{{Kind: Action, ActionID: "act1"}, {Kind: Result, ActionID: "act1"}}, AcquisitionFrequency{}},
		{"pending", []Event{{Kind: Acquisition, AcquisitionID: "a1"}}, AcquisitionFrequency{1, 0, 1}},
		{"empty", []Event{{Kind: Acquisition, AcquisitionID: "a1"}, {Kind: Observation, AcquisitionID: "a1"}}, AcquisitionFrequency{1, 1, 0}},
		{"image", []Event{{Kind: Acquisition, AcquisitionID: "a1"}, {Kind: Observation, AcquisitionID: "a1", Image: &ImageObservation{MediaType: "image/png", ByteLength: 73}}}, AcquisitionFrequency{1, 1, 0}},
		{"error", []Event{{Kind: Acquisition, AcquisitionID: "a1"}, {Kind: Observation, AcquisitionID: "a1", IsError: true}}, AcquisitionFrequency{1, 1, 0}},
		{"interleaved repeated target and out of order observations", []Event{
			{Kind: Acquisition, AcquisitionID: "a1", Target: "README.md"},
			{Kind: Action, ActionID: "act1"},
			{Kind: Acquisition, AcquisitionID: "a2", Target: "README.md"},
			{Kind: Observation, AcquisitionID: "a2", Text: "second"},
			{Kind: Result, ActionID: "act1"},
			{Kind: Acquisition, AcquisitionID: "a3", Target: "README.md"},
			{Kind: Observation, AcquisitionID: "a1", Text: "first"},
		}, AcquisitionFrequency{3, 2, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Run{Events: tc.events}
			got := r.AcquisitionFrequency()
			if got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
			if got.Attempts != got.MatchedObservations+got.MissingObservations {
				t.Fatal("count identity violated")
			}
			if got.MissingObservations != len(r.MissingObservations()) {
				t.Fatal("missing count differs from diagnostic")
			}
		})
	}
}

func TestAcquisitionAssociationsPreserveRequestsAndMatchByLocalID(t *testing.T) {
	r := Run{Events: []Event{
		{Kind: Acquisition, AcquisitionID: "a1", Target: "one", SourceID: "vendor-1", SourceLine: 2},
		{Kind: Acquisition, AcquisitionID: "a2", Target: "two", SourceID: "vendor-2", SourceLine: 3},
		{Kind: Observation, AcquisitionID: "a2", Text: "second", SourceLine: 4},
		{Kind: Observation, AcquisitionID: "a1", Text: "first", SourceLine: 5},
		{Kind: Acquisition, AcquisitionID: "a3", Target: "three", SourceLine: 6},
	}}
	pairs := r.AcquisitionAssociations()
	if len(pairs) != 3 {
		t.Fatalf("associations: %v", pairs)
	}
	if pairs[0].Request != r.Events[0] || pairs[1].Request != r.Events[1] || pairs[2].Request != r.Events[4] {
		t.Fatal("request order/provenance changed")
	}
	if pairs[0].Observation == nil || *pairs[0].Observation != r.Events[3] || pairs[1].Observation == nil || *pairs[1].Observation != r.Events[2] || pairs[2].Observation != nil {
		t.Fatal("incorrect observation association")
	}
}
