package compatreport

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/unfriction-ai/unfriction/internal/claude"
	"github.com/unfriction-ai/unfriction/internal/codex"
	"github.com/unfriction-ai/unfriction/internal/diagnostic"
	"github.com/unfriction-ai/unfriction/internal/run"
	"github.com/unfriction-ai/unfriction/internal/traceformat"
)

func TestAdaptersSupplyStructuredFailures(t *testing.T) {
	hook, err := os.ReadFile("../../fixtures/context-records/session-hook.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, input, typ, subtype string
		parse                     func(io.Reader) (run.Run, error)
		category                  diagnostic.Category
	}{
		{"unsupported codex", `{"type":"response_item","payload":{"type":"private-token"}}`, "response_item", "redacted", codex.Parse, diagnostic.UnsupportedVariant},
		{"malformed codex", `{"type":"world_state","payload":{}}`, "world_state", "absent", codex.Parse, diagnostic.MalformedInput},
		{"unsupported hook", strings.Replace(string(hook), `"hookEvent": "SessionStart"`, `"hookEvent": "Stop"`, 1), "attachment", "hook_success", claude.Parse, diagnostic.UnsupportedVariant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.parse(strings.NewReader(tc.input))
			var failure *diagnostic.Failure
			if !errors.As(err, &failure) || failure.Category != tc.category || failure.Line != 1 || failure.Record.Type != tc.typ || failure.Record.Subtype != tc.subtype {
				t.Fatalf("%+v %v", failure, err)
			}
		})
	}
	for _, tc := range []struct {
		input    string
		category diagnostic.Category
	}{
		{"{}", diagnostic.UnknownFormat},
		{`{"type":"event_msg","payload":{}}` + "\n" + `{"type":"user","message":{},"sessionId":"s"}`, diagnostic.AmbiguousFormat},
		{strings.Repeat("{}\n", 64), diagnostic.UndeterminedFormat},
	} {
		_, _, err := traceformat.Detect(strings.NewReader(tc.input))
		var failure *diagnostic.Failure
		if !errors.As(err, &failure) || failure.Category != tc.category {
			t.Fatalf("%+v %v", failure, err)
		}
	}
}

type failedReader struct{}

var readError = errors.New("local read failure")

func (failedReader) Read([]byte) (int, error) { return 0, readError }
func TestReadFailuresAreNotVendorIncompatibilities(t *testing.T) {
	for _, parse := range []func(io.Reader) (run.Run, error){codex.Parse, claude.Parse} {
		_, err := parse(failedReader{})
		var failure *diagnostic.Failure
		if errors.As(err, &failure) || !errors.Is(err, readError) || Append(err, "dev", "codex", "unknown", true) != err {
			t.Fatal(err)
		}
	}
	_, _, err := traceformat.Detect(failedReader{})
	if Append(err, "dev", "", "unknown", false) != err {
		t.Fatal("detection read error changed")
	}
}
