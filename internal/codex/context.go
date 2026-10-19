package codex

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/run"
)

// Context records are acknowledged without retaining their potentially sensitive
// values. Their contents never become acquisitions, observations, or completion.
func contextRecord(data []byte, line int) (run.Coverage, error) {
	var envelope struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return run.Coverage{}, fmt.Errorf("invalid context record")
	}
	var p map[string]json.RawMessage
	if json.Unmarshal(envelope.Payload, &p) != nil || p == nil {
		return run.Coverage{}, fmt.Errorf("context payload must be an object")
	}
	notice := run.Coverage{SourceLine: line}
	if envelope.Type == "world_state" {
		var full *bool
		if len(p) != 2 || json.Unmarshal(p["full"], &full) != nil || full == nil || !contextObject(p["state"]) {
			return notice, fmt.Errorf("unsupported world_state shape; expected boolean full and object state")
		}
		notice.Summary = "Environment state recorded; supplied context is not reconstructed."
	} else {
		if len(p) != 4 || !contextString(p["id"]) || !contextString(p["encrypted_content"]) || !contextObject(p["internal_chat_message_metadata_passthrough"]) {
			return notice, fmt.Errorf("unsupported compaction shape; expected ID, opaque text, and metadata object")
		}
		notice.Summary = "Opaque context compaction recorded; retained context is unknown."
	}
	return notice, nil
}

func contextObject(raw json.RawMessage) bool {
	var value map[string]json.RawMessage
	return json.Unmarshal(raw, &value) == nil && value != nil
}
func contextString(raw json.RawMessage) bool {
	var value string
	return json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != ""
}
