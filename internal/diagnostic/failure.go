// Package diagnostic carries structured compatibility failures independently of
// local error text. Public reporting must never copy Cause.Error() into a URL.
package diagnostic

import (
	"encoding/json"
	"errors"
)

type Category string

const (
	UnknownFormat      Category = "unknown format"
	AmbiguousFormat    Category = "ambiguous format"
	UndeterminedFormat Category = "undetermined format"
	UnsupportedVariant Category = "unsupported record or variant"
	MalformedInput     Category = "malformed adapter input"
)

type Record struct{ Type, Subtype string }
type Failure struct {
	Category Category
	Line     int
	Record   Record
	Cause    error
}

func (f *Failure) Error() string { return f.Cause.Error() }
func (f *Failure) Unwrap() error { return f.Cause }

func Detection(category Category, err error) error { return &Failure{Category: category, Cause: err} }

type unsupported struct{ error }

func (u *unsupported) Unwrap() error { return u.error }
func Unsupported(err error) error    { return &unsupported{err} }
func Adapter(err error, line int, record Record) error {
	var existing *Failure
	if errors.As(err, &existing) {
		return err
	}
	category := MalformedInput
	var marker *unsupported
	if errors.As(err, &marker) {
		category = UnsupportedVariant
	}
	return &Failure{Category: category, Line: line, Record: record, Cause: err}
}

// RecordFromJSON retains only allowlisted structural names, including an explicit
// distinction between an absent field, an empty string, and a redacted value.
func RecordFromJSON(data []byte) Record {
	var e map[string]json.RawMessage
	if json.Unmarshal(data, &e) != nil || e == nil {
		return Record{}
	}
	record := Record{Type: field(e["type"]), Subtype: "absent"}
	var nested map[string]json.RawMessage
	if record.Type == "response_item" || record.Type == "event_msg" {
		if json.Unmarshal(e["payload"], &nested) == nil {
			record.Subtype = field(nested["type"])
		}
	} else if record.Type == "attachment" {
		if json.Unmarshal(e["attachment"], &nested) == nil {
			record.Subtype = field(nested["type"])
		}
	}
	return record
}

func field(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "absent"
	}
	var value *string
	if json.Unmarshal(raw, &value) != nil || value == nil {
		return "redacted"
	}
	if *value == "" {
		return "empty"
	}
	switch *value {
	case "absent", "empty", "unavailable":
		return "redacted"
	}
	return SafeName(*value)
}

func SafeName(value string) string {
	switch value {
	case "absent", "empty", "redacted", "unavailable":
		return value
	case "session_meta", "turn_context", "world_state", "response_item", "event_msg",
		"message", "reasoning", "compaction", "function_call", "function_call_output", "custom_tool_call", "custom_tool_call_output",
		"task_started", "user_message", "agent_message", "task_complete", "token_count", "agent_reasoning", "turn_aborted", "thread_settings_applied", "item_completed",
		"user", "assistant", "attachment", "last-prompt", "file-history-snapshot", "progress", "queue-operation", "mode", "permission-mode", "atis-latch",
		"hook_success", "hook_error", "tool_use", "tool_result", "text", "thinking":
		return value
	case "":
		return "unavailable"
	default:
		return "redacted"
	}
}
