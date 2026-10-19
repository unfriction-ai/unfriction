package claude

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestEditContract(t *testing.T) {
	good := `{"file_path":"README.md","old_string":"old","new_string":"new","replace_all":false}`
	for _, tc := range []struct{ name, input string }{
		{"replace all", strings.Replace(good, "false", "true", 1)},
		{"missing replace all", strings.Replace(good, `,"replace_all":false`, "", 1)},
		{"null old", strings.Replace(good, `"old"`, `null`, 1)},
		{"empty old", strings.Replace(good, `"old"`, `""`, 1)},
		{"null new", strings.Replace(good, `"new"`, `null`, 1)},
		{"same text", strings.Replace(good, `"new"`, `"old"`, 1)},
		{"unknown field", strings.Replace(good, `}`, `,"future":true}`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := editTarget(json.RawMessage(tc.input)); err == nil {
				t.Fatal("accepted unsupported edit")
			}
		})
	}
	if target, err := editTarget(json.RawMessage(good)); err != nil || target != "README.md" {
		t.Fatalf("%s %v", target, err)
	}
}

func TestRejectNullActionErrorFlag(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/claude-action.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	input := strings.Replace(string(b), `"tool_use_id": "edit-1"`, `"tool_use_id": "edit-1", "is_error": null`, 1)
	if _, err := Parse(strings.NewReader(input)); err == nil {
		t.Fatal("accepted null error flag")
	}
}

func TestRecordedOffsetEditAttempt(t *testing.T) {
	good := `{"file_path":"README.md","offset":"1884","new_text":"replacement"}`
	if target, err := editTarget(json.RawMessage(good)); err != nil || target != "README.md" {
		t.Fatalf("%s %v", target, err)
	}
	for _, input := range []string{
		strings.Replace(good, `"1884"`, `1884`, 1), strings.Replace(good, `"1884"`, `"-1"`, 1),
		strings.Replace(good, `"1884"`, `"1.5"`, 1), strings.Replace(good, `"1884"`, `null`, 1),
		strings.Replace(good, `"replacement"`, `null`, 1), strings.Replace(good, `"replacement"`, `""`, 1),
		strings.Replace(good, `}`, `,"old_string":"old"}`, 1),
		strings.Replace(good, `}`, `,"replace_all":false}`, 1),
	} {
		if _, err := editTarget(json.RawMessage(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
