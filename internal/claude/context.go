package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/diagnostic"
)

func validateLastPrompt(data []byte) (string, error) {
	var fields map[string]json.RawMessage
	var leaf string
	if json.Unmarshal(data, &fields) != nil || len(fields) != 3 || json.Unmarshal(fields["leafUuid"], &leaf) != nil || strings.TrimSpace(leaf) == "" {
		return "", fmt.Errorf("unsupported last-prompt shape; expected type, sessionId, and nonempty leafUuid")
	}
	return leaf, nil
}

func validateSessionHook(raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 10 {
		return fmt.Errorf("unsupported attachment shape; expected a SessionStart hook_success record")
	}
	for _, key := range []string{"type", "hookEvent", "hookName", "toolUseID", "command", "content", "stdout", "stderr"} {
		var text *string
		if json.Unmarshal(fields[key], &text) != nil || text == nil {
			return fmt.Errorf("invalid SessionStart hook field %s; expected text", key)
		}
		if (key == "type" && *text != "hook_success") || (key == "hookEvent" && *text != "SessionStart") {
			return diagnostic.Unsupported(fmt.Errorf("unsupported attachment variant; only SessionStart hook_success is supported"))
		}
		if (key == "hookName" || key == "toolUseID") && strings.TrimSpace(*text) == "" {
			return fmt.Errorf("missing SessionStart hook identifier")
		}
	}
	var exit *int
	var duration *float64
	if json.Unmarshal(fields["exitCode"], &exit) != nil || exit == nil || *exit != 0 || json.Unmarshal(fields["durationMs"], &duration) != nil || duration == nil || *duration < 0 {
		return fmt.Errorf("invalid SessionStart hook result; expected zero exitCode and nonnegative durationMs")
	}
	return nil
}

type navigationPointer struct {
	id   string
	line int
}
type navigationNode struct {
	id     string
	parent json.RawMessage
	line   int
}

// Only discard navigation bookkeeping after proving there is no branch choice:
// all retained message/hook nodes must form one explicit, ordered parent chain.
func validateNavigation(pointers []navigationPointer, nodes []navigationNode) error {
	if len(pointers) == 0 {
		return nil
	}
	ids := map[string]bool{}
	previous := ""
	for i, node := range nodes {
		var parent *string
		if strings.TrimSpace(node.id) == "" || ids[node.id] || len(node.parent) == 0 || json.Unmarshal(node.parent, &parent) != nil {
			return fmt.Errorf("line %d: cannot resolve last-prompt safely; supply unique node IDs and explicit parent links for one linear turn", node.line)
		}
		if (i == 0 && parent != nil) || (i > 0 && (parent == nil || *parent != previous)) {
			return fmt.Errorf("line %d: last-prompt requires one linear parent chain; branches or missing ancestors are unsupported", node.line)
		}
		ids[node.id] = true
		previous = node.id
	}
	for _, pointer := range pointers {
		if !ids[pointer.id] {
			return fmt.Errorf("line %d: last-prompt references a node outside the supplied turn; supply its complete linear turn", pointer.line)
		}
	}
	return nil
}
