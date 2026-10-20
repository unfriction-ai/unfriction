package run

import "testing"

func TestTimingUsesAssociatedObservationAndNeverSums(t *testing.T) {
	zero := &RecordedDuration{Seconds: "0.0000"}
	duration := &RecordedDuration{Seconds: "0.071249250"}
	r := Run{Events: []Event{
		{Kind: Acquisition, AcquisitionID: "a1", SourceID: "first", SourceLine: 2},
		{Kind: Action, ActionID: "act1", Duration: duration},
		{Kind: Acquisition, AcquisitionID: "a2", SourceID: "second", SourceLine: 4},
		{Kind: Observation, AcquisitionID: "a2", SourceLine: 5, Duration: zero},
		{Kind: Result, ActionID: "act1", Duration: duration},
		{Kind: Observation, AcquisitionID: "a1", SourceLine: 7, Duration: duration},
		{Kind: Acquisition, AcquisitionID: "a3", SourceID: "pending", SourceLine: 8},
	}}
	got := r.AcquisitionTimings()
	if len(got) != 3 || got[0].Duration != duration || got[0].SourceLine != 7 || got[0].SourceID != "first" || got[1].Duration != zero || got[1].SourceLine != 5 || got[2].Duration != nil || got[2].SourceLine != 8 {
		t.Fatalf("incorrect associated timing: %+v", got)
	}
}
