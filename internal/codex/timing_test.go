package codex

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestOptionalStructuredDuration(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`{"secs":1,"nanos":42}`, "1.000000042"},
		{`{"secs":0,"nanos":0}`, "0.000000000"},
		{`{"secs":18446744073709551615,"nanos":999999999}`, "18446744073709551615.999999999"},
		{`null`, ""}, {``, ""}, {`{}`, ""}, {`{"secs":0}`, ""},
		{`{"secs":-1,"nanos":0}`, ""}, {`{"secs":1.2,"nanos":0}`, ""},
		{`{"secs":0,"nanos":1000000000}`, ""}, {`{"secs":0,"nanos":-1}`, ""},
		{`{"secs":"1","nanos":0}`, ""}, {`{"secs":18446744073709551616,"nanos":0}`, ""},
		{`{"secs":0,"nanos":4294967296}`, ""}, {`"bad"`, ""},
	} {
		got := structuredDuration(json.RawMessage(tc.raw))
		if tc.want == "" {
			if got != nil {
				t.Errorf("%s became %+v", tc.raw, got)
			}
		} else if got == nil || got.Seconds != tc.want {
			t.Errorf("%s got %+v want %s", tc.raw, got, tc.want)
		}
	}
}

func TestOptionalEnvelopeDuration(t *testing.T) {
	for _, tc := range []struct{ header, want string }{
		{"Wall time: 0.0000 seconds", "0.0000"}, {"Wall time: 1.25 seconds", "1.25"},
		{"", ""}, {"Wall time: -1 seconds", ""}, {"Wall time: NaN seconds", ""},
		{"Wall time: 1e9 seconds", ""}, {"Wall time: 1 ms", ""},
		{"Wall time: 1 seconds\nWall time: 2 seconds", ""},
		{"Wall time: " + strings.Repeat("9", 400) + " seconds", ""},
	} {
		output := "Chunk ID: example\n" + tc.header + "\nProcess exited with code 0\nOutput:\nWall time: 9 seconds\n"
		got := envelopeDuration(output)
		if tc.want == "" {
			if got != nil {
				t.Errorf("%q became %+v", tc.header, got)
			}
		} else if got == nil || got.Seconds != tc.want {
			t.Errorf("%q got %+v", tc.header, got)
		}
	}
}

func TestMalformedTimingPreservesSupportedImport(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/timing-evidence/codex-0.163.0.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []string{`null`, `"malformed"`, `{"secs":-1,"nanos":0}`, `{"secs":0,"nanos":1000000000}`} {
		input := strings.Replace(string(b), `{"secs": 0, "nanos": 71249250}`, replacement, 1)
		r, err := Parse(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		timings := r.AcquisitionTimings()
		if len(timings) != 2 || timings[0].Duration != nil || timings[1].Duration == nil || timings[1].Duration.Seconds != "0.000007292" {
			t.Fatalf("bad optional timing: %+v", timings)
		}
	}
}
