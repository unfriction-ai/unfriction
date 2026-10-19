package traceformat

import (
	"errors"
	"io"
	"strings"
	"testing"
)

const codexRecord = `{"type":"event_msg","payload":{}}`
const claudeRecord = `{"type":"assistant","message":{},"sessionId":"s"}`

func TestStructuralDetection(t *testing.T) {
	for _, tc := range []struct{ name, input, want, diagnostic string }{
		{"codex", codexRecord, "codex", ""},
		{"response item", `{"type":"response_item","payload":{}}`, "codex", ""},
		{"claude", claudeRecord, "claude-code", ""},
		{"user", `{"type":"user","message":{},"sessionId":"s"}`, "claude-code", ""},
		{"skip metadata", "\n{\"type\":\"session_meta\"}\n" + codexRecord, "codex", ""},
		{"blank CRLF", " \r\n" + claudeRecord + "\r\n", "claude-code", ""},
		{"empty", "", "", "unknown trace format"},
		{"keywords", `{"type":"other","text":"Codex event_msg Claude Code assistant"}`, "", "unknown trace format"},
		{"type alone", `{"type":"event_msg"}`, "", "unknown trace format"},
		{"null payload", `{"type":"event_msg","payload":null}`, "", "unknown trace format"},
		{"scalar payload", `{"type":"event_msg","payload":"text"}`, "", "unknown trace format"},
		{"no session", `{"type":"assistant","message":{}}`, "", "unknown trace format"},
		{"empty session", `{"type":"assistant","message":{},"sessionId":""}`, "", "unknown trace format"},
		{"array message", `{"type":"assistant","message":[],"sessionId":"s"}`, "", "unknown trace format"},
		{"conflicting", codexRecord + "\n" + claudeRecord, "", "ambiguous trace format"},
		{"conflicting reversed", claudeRecord + "\n" + codexRecord, "", "ambiguous trace format"},
		{"malformed only", "{", "", "unknown trace format"},
		{"malformed after evidence", codexRecord + "\n{", "codex", ""},
		{"malformed before evidence", "{\n" + claudeRecord, "claude-code", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			format, replay, err := Detect(strings.NewReader(tc.input))
			if tc.diagnostic != "" {
				if err == nil || !strings.Contains(err.Error(), tc.diagnostic) || !strings.Contains(err.Error(), "--format") {
					t.Fatalf("%s %v", format, err)
				}
				return
			}
			if err != nil || format != tc.want {
				t.Fatalf("want %s, got %s %v", tc.want, format, err)
			}
			data, err := io.ReadAll(replay)
			if err != nil || string(data) != tc.input {
				t.Fatalf("input changed: %v %q", err, data)
			}
		})
	}
}

type countingReader struct {
	io.Reader
	bytes int
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.bytes += n
	return n, err
}

func TestPrefixBudgetsAndReplay(t *testing.T) {
	metadata := "{\"type\":\"session_meta\"}\n"
	for _, tc := range []struct{ name, input, want string }{
		{"last allowed record", strings.Repeat(metadata, MaxPrefixRecords-1) + codexRecord, "codex"},
		{"beyond record limit", strings.Repeat(metadata, MaxPrefixRecords) + codexRecord, ""},
		{"byte limit", strings.Repeat(" ", MaxPrefixBytes) + codexRecord, ""},
		{"partial record", strings.Repeat(" ", MaxPrefixBytes-10) + codexRecord, ""},
		{"last complete record", strings.Repeat(" ", MaxPrefixBytes-len(codexRecord)-1) + codexRecord + "\n", "codex"},
		{"trailing bytes replay", codexRecord + "\n" + strings.Repeat("x", MaxPrefixBytes*2), "codex"},
		{"conflict at record limit", codexRecord + "\n" + strings.Repeat(metadata, MaxPrefixRecords-2) + claudeRecord, "ambiguous"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := &countingReader{Reader: strings.NewReader(tc.input)}
			format, replay, err := Detect(input)
			if input.bytes > MaxPrefixBytes {
				t.Fatalf("read %d bytes", input.bytes)
			}
			if tc.want == "" {
				if err == nil || !strings.Contains(err.Error(), "prefix limit") {
					t.Fatalf("%s %v", format, err)
				}
				return
			}
			if tc.want == "ambiguous" {
				if err == nil || !strings.Contains(err.Error(), "ambiguous") {
					t.Fatal(err)
				}
				return
			}
			if err != nil || format != tc.want {
				t.Fatalf("%s %v", format, err)
			}
			data, err := io.ReadAll(replay)
			if err != nil || string(data) != tc.input {
				t.Fatalf("replay changed: %v", err)
			}
		})
	}
}

type brokenReader struct{}

var readFailure = errors.New("read failure")

func (brokenReader) Read([]byte) (int, error) { return 0, readFailure }
func TestReadFailureDoesNotSelectAnAdapter(t *testing.T) {
	input := io.MultiReader(strings.NewReader(codexRecord+"\n"), brokenReader{})
	format, _, err := Detect(input)
	if format != "" || !errors.Is(err, readFailure) {
		t.Fatalf("%s %v", format, err)
	}
}
