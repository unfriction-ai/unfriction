package run

import "testing"

func TestAcquisitionVolume(t *testing.T) {
	for _, tc := range []struct {
		name        string
		observation *Event
		want        AcquisitionVolume
	}{
		{"missing", nil, AcquisitionVolume{}},
		{"empty", &Event{}, AcquisitionVolume{}},
		{"ascii", &Event{Text: "abc"}, AcquisitionVolume{TextBytes: 3}},
		{"utf8 and newline", &Event{Text: "é🙂\n"}, AcquisitionVolume{TextBytes: 7}},
		{"error text", &Event{Text: "error\n", IsError: true}, AcquisitionVolume{TextBytes: 6}},
		{"validated image", &Event{Image: &ImageObservation{MediaType: "image/png", ByteLength: 73}}, AcquisitionVolume{ImageBytes: 73}},
		{"image metadata is not text", &Event{Text: "ignored", Image: &ImageObservation{MediaType: "image/jpeg", ByteLength: 599}}, AcquisitionVolume{ImageBytes: 599}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Run{Events: []Event{{Kind: Acquisition, AcquisitionID: "a1", Text: "request is not returned text"}}}
			if tc.observation != nil {
				e := *tc.observation
				e.Kind = Observation
				e.AcquisitionID = "a1"
				r.Events = append(r.Events, e)
			}
			if got := r.AcquisitionVolume(); got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

func TestAcquisitionVolumeSumsOnlyAssociatedObservationPayloads(t *testing.T) {
	r := Run{Events: []Event{
		{Kind: Acquisition, AcquisitionID: "a1"},
		{Kind: Action, ActionID: "act1", Text: "action text"},
		{Kind: Acquisition, AcquisitionID: "a2"},
		{Kind: Observation, AcquisitionID: "a2", Image: &ImageObservation{MediaType: "image/png", ByteLength: 73}},
		{Kind: Result, ActionID: "act1", Text: "result text"},
		{Kind: Observation, AcquisitionID: "a1", Text: "é🙂\n"},
		{Kind: Acquisition, AcquisitionID: "a3"},
		{Kind: Observation, AcquisitionID: "a3", Text: "error\n", IsError: true},
		{Kind: Acquisition, AcquisitionID: "a4"},
		{Kind: Observation, AcquisitionID: "a4", Image: &ImageObservation{MediaType: "image/jpeg", ByteLength: 599}},
		{Kind: Acquisition, AcquisitionID: "a5"},
	}}
	if got := r.AcquisitionVolume(); got != (AcquisitionVolume{TextBytes: 13, ImageBytes: 672}) {
		t.Fatalf("volume %+v", got)
	}
	if got := r.AcquisitionFrequency(); got != (AcquisitionFrequency{Attempts: 5, MatchedObservations: 4, MissingObservations: 1}) {
		t.Fatalf("frequency changed %+v", got)
	}
	if got := (Run{}).AcquisitionVolume(); got != (AcquisitionVolume{}) {
		t.Fatalf("empty run volume %+v", got)
	}
}
