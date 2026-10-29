package codex

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/diagnostic"
	"github.com/unfriction-ai/unfriction/internal/run"
	"github.com/unfriction-ai/unfriction/internal/session"
	"github.com/unfriction-ai/unfriction/internal/source"
)

// Session summaries decode lifecycle fields only. Tool output can have a newer
// envelope shape in an unrelated turn and is validated only when selected.
type sessionEnvelope struct {
	Type    string
	Payload struct {
		Type             string
		TurnID           string `json:"turn_id"`
		ThreadID         string `json:"thread_id"`
		CallID           string `json:"call_id"`
		Message          string
		Phase            string
		LastAgentMessage string `json:"last_agent_message"`
		Item             json.RawMessage
	}
}

// Summarize validates session boundaries without normalizing tool payloads.
func Summarize(input io.Reader) (session.Document, error) {
	records, err := source.ReadAll(input)
	if err != nil {
		return session.Document{}, err
	}
	d := session.Document{Family: "codex", Records: records}
	seen := map[string]bool{}
	active := -1
	thread := ""
	calls := map[string]string{}
	for i, rec := range records {
		if strings.TrimSpace(string(rec.Data)) == "" {
			continue
		}
		var e sessionEnvelope
		if err := json.Unmarshal(rec.Data, &e); err != nil {
			return d, sessionError(rec, "invalid event")
		}
		var meta struct {
			Timestamp string
			Payload   struct {
				ID         string
				CWD        string
				CLIVersion json.RawMessage `json:"cli_version"`
				Git        struct{ Branch string }
				Timestamp  string
			}
		}
		if e.Type == "session_meta" {
			if json.Unmarshal(rec.Data, &meta) != nil {
				return d, sessionError(rec, "invalid session metadata")
			}
			if strings.TrimSpace(meta.Payload.ID) == "" {
				return d, sessionError(rec, "missing session identity")
			}
			if d.ID != "" && d.ID != meta.Payload.ID {
				return d, sessionError(rec, "conflicting session identity")
			}
			d.ID, d.CWD, d.Branch, d.Timestamp = meta.Payload.ID, meta.Payload.CWD, meta.Payload.Git.Branch, meta.Payload.Timestamp
			_ = json.Unmarshal(meta.Payload.CLIVersion, &d.Version)
			continue
		}
		p := e.Payload
		if p.TurnID != "" && !(e.Type == "event_msg" && p.Type == "task_started") && (active < 0 || p.TurnID != d.Turns[active].ID) {
			return d, sessionError(rec, "record belongs to another turn")
		}
		if p.ThreadID != "" && d.ID != "" && p.ThreadID != d.ID {
			return d, sessionError(rec, "record belongs to another session")
		}
		if e.Type == "event_msg" && p.Type == "task_started" {
			if d.ID == "" {
				return d, sessionError(rec, "turn requires recorded session identity")
			}
			if active >= 0 {
				return d, sessionError(rec, "overlapping turns without completion or abort")
			}
			if strings.TrimSpace(p.TurnID) == "" || seen[p.TurnID] {
				return d, sessionError(rec, "missing or duplicate turn identity")
			}
			seen[p.TurnID] = true
			_ = json.Unmarshal(rec.Data, &meta)
			t := session.Turn{ID: p.TurnID, Start: i, End: len(records), Timestamp: meta.Timestamp}
			if len(d.Turns) > 0 {
				prev := d.Turns[len(d.Turns)-1]
				t.PreviousTurnID = prev.ID
				t.PreviousAgent = prev.Final
				t.EarlierUser = prev.Prompt
			}
			d.Turns = append(d.Turns, t)
			active = len(d.Turns) - 1
			continue
		}
		if e.Type == "turn_context" {
			if active < 0 || p.TurnID != "" && p.TurnID != d.Turns[active].ID {
				return d, sessionError(rec, "turn context outside matching turn")
			}
			continue
		}
		if e.Type == "event_msg" {
			switch p.Type {
			case "thread_settings_applied", "token_count", "agent_reasoning":
				continue
			case "user_message", "agent_message", "item_completed", "task_complete", "turn_aborted":
				if active < 0 {
					return d, sessionError(rec, "activity outside an active turn")
				}
				t := &d.Turns[active]
				if p.TurnID != "" && p.TurnID != t.ID {
					return d, sessionError(rec, "record belongs to another turn")
				}
				switch p.Type {
				case "user_message":
					if t.Prompt != "" || strings.TrimSpace(p.Message) == "" {
						return d, sessionError(rec, "expected one nonempty prompt")
					}
					t.Prompt = p.Message
				case "agent_message":
					if p.Phase == "final_answer" {
						if t.Final != "" {
							return d, sessionError(rec, "duplicate final response")
						}
						t.Final = p.Message
					}
				case "item_completed":
					if p.TurnID == "" || p.ThreadID == "" || thread != "" && thread != p.ThreadID {
						return d, sessionError(rec, "structured item requires matching turn and thread")
					}
					thread = p.ThreadID
					var item completedItem
					if json.Unmarshal(p.Item, &item) != nil || item.ID == "" {
						return d, sessionError(rec, "invalid structured item")
					}
					if item.Type == "UserMessage" || item.Type == "AgentMessage" {
						if len(item.Content) != 1 || strings.TrimSpace(item.Content[0].Text) == "" {
							return d, sessionError(rec, "unsupported structured message")
						}
						if item.Type == "UserMessage" {
							if t.Prompt != "" {
								return d, sessionError(rec, "duplicate prompt representation")
							}
							t.Prompt = item.Content[0].Text
						}
						if item.Type == "AgentMessage" && item.Phase == "final_answer" {
							if t.Final != "" {
								return d, sessionError(rec, "duplicate final response")
							}
							t.Final = item.Content[0].Text
						}
					}
				case "task_complete":
					if p.TurnID != t.ID || t.Final == "" || p.LastAgentMessage != t.Final {
						return d, sessionError(rec, "completion must match the active turn and final response")
					}
					t.Completed = true
					t.End = i + 1
					active = -1
				case "turn_aborted":
					t.Aborted = true
					t.End = i + 1
					active = -1
				}
			default:
				return d, sessionError(rec, "unsupported lifecycle record")
			}
			continue
		}
		switch e.Type {
		case "response_item":
			if active < 0 && p.Type != "reasoning" && p.Type != "additional_tools" {
				return d, sessionError(rec, "response activity outside an active turn")
			}
			switch p.Type {
			case "function_call", "custom_tool_call":
				if p.CallID == "" || calls[p.CallID] != "" {
					return d, sessionError(rec, "missing or duplicate tool request identity")
				}
				calls[p.CallID] = d.Turns[active].ID
			case "function_call_output", "custom_tool_call_output":
				if p.CallID == "" || calls[p.CallID] != d.Turns[active].ID {
					return d, sessionError(rec, "tool result crosses turn ownership or lacks a recorded request")
				}
			}
		case "world_state", "token_usage_record":
		case "compacted":
			return d, sessionError(rec, "compaction may remove required boundary evidence; cannot isolate this session")
		default:
			return d, sessionError(rec, "unsupported session record")
		}
	}
	for _, t := range d.Turns {
		if t.Prompt == "" {
			return d, fmt.Errorf("turn has no recorded user prompt")
		}
	}
	if len(d.Turns) == 0 {
		return d, fmt.Errorf("session contains no user turns")
	}
	return d, nil
}

func sessionError(rec source.Record, message string) error {
	return diagnostic.Adapter(diagnostic.Unsupported(fmt.Errorf("line %d: %s", rec.Line, message)), rec.Line, diagnostic.RecordFromJSON(rec.Data))
}

func ParseTurn(input io.Reader, id string) (run.Run, session.Document, int, error) {
	d, err := Summarize(input)
	if err != nil {
		return run.Run{}, d, 0, err
	}
	t, index, err := d.Find(id)
	if err != nil {
		return run.Run{}, d, 0, err
	}
	var selected []source.Record
	var coverage []run.Coverage
	userMirror := false
	// Agent message mirrors have an evidenced shared item/message identity.
	// This rule does not extend to tool wrappers, whose identities differ.
	agents := map[string]completedItem{}
	for _, rec := range d.Records[t.Start:t.End] {
		var e sessionEnvelope
		_ = json.Unmarshal(rec.Data, &e)
		if e.Type == "event_msg" && e.Payload.Type == "item_completed" {
			var item completedItem
			_ = json.Unmarshal(e.Payload.Item, &item)
			if item.Type == "AgentMessage" {
				agents[item.ID] = item
			}
		}
	}
	// Session metadata supplies recording version, not earlier turn telemetry.
	for _, rec := range d.Records[:t.Start] {
		var e event
		_ = json.Unmarshal(rec.Data, &e)
		if e.Type == "session_meta" {
			selected = append(selected, rec)
		}
	}
	for _, rec := range d.Records[t.Start:t.End] {
		var e event
		_ = json.Unmarshal(rec.Data, &e)
		if e.Type == "response_item" && e.Payload.Type == "message" {
			var raw struct {
				Payload struct {
					ID, Role, Phase string
					Content         []struct{ Type, Text string }
				}
			}
			if json.Unmarshal(rec.Data, &raw) != nil {
				return run.Run{}, d, index, sessionError(rec, "invalid message mirror")
			}
			if raw.Payload.Role == "user" {
				if userMirror || len(raw.Payload.Content) != 1 || raw.Payload.Content[0].Type != "input_text" || raw.Payload.Content[0].Text != t.Prompt {
					return run.Run{}, d, index, sessionError(rec, "conflicting or ambiguous raw user prompt")
				}
				userMirror = true
				coverage = append(coverage, run.Coverage{SourceLine: rec.Line, Summary: "Raw user input agrees with the authoritative prompt for this turn; not a measured operation."})
			}
			if raw.Payload.Role == "assistant" {
				item, ok := agents[raw.Payload.ID]
				if !ok || len(item.Content) != 1 || len(raw.Payload.Content) != 1 || raw.Payload.Content[0].Type != "output_text" || raw.Payload.Content[0].Text != item.Content[0].Text || raw.Payload.Phase != item.Phase {
					return run.Run{}, d, index, sessionError(rec, "conflicting or uncorrelated agent message mirror")
				}
				coverage = append(coverage, run.Coverage{SourceLine: rec.Line, Summary: "Raw agent message mirrors the same recorded structured message identity; counted once."})
			}
		}
		ancillary := e.Type == "token_usage_record" || e.Type == "event_msg" && (e.Payload.Type == "thread_settings_applied" || e.Payload.Type == "turn_aborted") || e.Type == "response_item" && e.Payload.Type == "additional_tools"
		if e.Type == "event_msg" && e.Payload.Type == "item_completed" {
			var item completedItem
			_ = json.Unmarshal(e.Payload.Item, &item)
			ancillary = ancillary || item.Type == "Reasoning" || item.Type == "AgentMessage" && item.Phase == "commentary"
		}
		if ancillary {
			notice := "Session record retained as non-measured context; it does not establish task success."
			if e.Type == "event_msg" && e.Payload.Type == "turn_aborted" {
				notice = "Recorded turn abort; completion is not established."
			}
			coverage = append(coverage, run.Coverage{SourceLine: rec.Line, Summary: notice})
			continue
		}
		if e.Type == "response_item" && e.Payload.Type == "reasoning" || e.Type == "event_msg" && (e.Payload.Type == "token_count" || e.Payload.Type == "agent_reasoning" || e.Payload.Type == "agent_message" && e.Payload.Phase != "final_answer") {
			coverage = append(coverage, run.Coverage{SourceLine: rec.Line, Summary: "Recorded reasoning, commentary or usage context; not a measured acquisition or completion."})
		}
		selected = append(selected, rec)
	}
	r, err := parse(source.Records(selected), true)
	if err == nil {
		r.Coverage = append(r.Coverage, coverage...)
	}
	return r, d, index, err
}
