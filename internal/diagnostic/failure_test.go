package diagnostic

import (
	"errors"
	"testing"
)

func TestSafeRecordFields(t *testing.T) {
	for _, tc := range []struct{ input, typ, subtype string }{
		{`{"type":"world_state","payload":{"state":{"token":"private"}}}`, "world_state", "absent"},
		{`{"type":"response_item","payload":{"type":""}}`, "response_item", "empty"},
		{`{"type":"response_item","payload":{}}`, "response_item", "absent"},
		{`{"type":"attachment","attachment":{"type":"hook_success"}}`, "attachment", "hook_success"},
		{`{"type":"response_item","payload":{"type":"private-token"}}`, "response_item", "redacted"},
		{`{"type":"private/path"}`, "redacted", "absent"},
		{`{"type":"absent"}`, "redacted", "absent"},
		{`{"type":null}`, "redacted", "absent"},
		{`{}`, "absent", "absent"},
		{`{`, "", ""},
	} {
		record := RecordFromJSON([]byte(tc.input))
		if record.Type != tc.typ || record.Subtype != tc.subtype {
			t.Fatalf("%+v", record)
		}
	}
}
func TestStructuredCategoryPreservesCause(t *testing.T) {
	cause := errors.New("local text")
	for _, tc := range []struct {
		err  error
		want Category
	}{{cause, MalformedInput}, {Unsupported(cause), UnsupportedVariant}} {
		err := Adapter(tc.err, 4, Record{Type: "world_state"})
		var failure *Failure
		if !errors.As(err, &failure) || failure.Category != tc.want || failure.Line != 4 || !errors.Is(err, cause) || err.Error() != cause.Error() {
			t.Fatal(err)
		}
	}
}
