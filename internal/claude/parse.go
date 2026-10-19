// Package claude imports a reduced, single-turn Claude Code session JSONL trace.
package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/run"
)

type event struct {
	Type       string          `json:"type"`
	UUID       string          `json:"uuid"`
	Parent     json.RawMessage `json:"parentUuid"`
	Attachment json.RawMessage `json:"attachment"`
	SessionID  string          `json:"sessionId"`
	Sidechain  bool            `json:"isSidechain"`
	Message    struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		StopReason string          `json:"stop_reason"`
	} `json:"message"`
}

type block struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input struct {
		Path string `json:"file_path"`
	} `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	Text      string          `json:"text"`
}

func Parse(input io.Reader) (run.Run, error) {
	r := run.Run{ExecutionState: run.Incomplete}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	session := ""
	var pointers []navigationPointer
	var nodes []navigationNode
	for line := 1; scanner.Scan(); line++ {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var e event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return run.Run{}, fmt.Errorf("line %d: invalid event: %w", line, err)
		}
		if e.Type == "" {
			return run.Run{}, fmt.Errorf("line %d: missing event type", line)
		}
		fail := func(message string) (run.Run, error) { return run.Run{}, fmt.Errorf("line %d: %s", line, message) }
		switch e.Type {
		case "user", "assistant", "last-prompt", "attachment":
		case "file-history-snapshot", "progress", "queue-operation":
			continue
		default:
			return fail("unsupported event " + e.Type + "; supply a reduced single turn")
		}
		if e.Sidechain {
			return fail("sidechain runs are unsupported")
		}
		if e.SessionID == "" {
			return fail("missing session ID")
		}
		if session == "" {
			session = e.SessionID
		} else if session != e.SessionID {
			return fail("multiple sessions are unsupported")
		}
		if e.Type == "last-prompt" {
			leaf, err := validateLastPrompt(scanner.Bytes())
			if err != nil {
				return fail(err.Error())
			}
			pointers = append(pointers, navigationPointer{id: leaf, line: line})
			continue
		}
		nodes = append(nodes, navigationNode{id: e.UUID, parent: e.Parent, line: line})
		if e.Type == "attachment" {
			if strings.TrimSpace(e.UUID) == "" {
				return fail("SessionStart hook requires a nonempty node ID")
			}
			if err := validateSessionHook(e.Attachment); err != nil {
				return fail(err.Error())
			}
			r.Coverage = append(r.Coverage, run.Coverage{SourceLine: line, Summary: "Session-start hook recorded; output may supply context and is not interpreted."})
			continue
		}
		if r.ExecutionState == run.Completed {
			return fail("message outside active run")
		}
		if e.Message.Role != e.Type {
			return fail("message role must match event type")
		}
		var task string
		if e.Type == "user" && json.Unmarshal(e.Message.Content, &task) == nil && string(e.Message.Content) != "null" {
			if r.Task != "" || strings.TrimSpace(task) == "" {
				return fail("expected one nonempty task")
			}
			r.Task = task
			continue
		}
		if r.Task == "" {
			return fail("message before task")
		}
		var blocks []block
		if err := json.Unmarshal(e.Message.Content, &blocks); err != nil || len(blocks) == 0 {
			return fail("expected nonempty content blocks")
		}
		var texts []string
		hasCall := false
		for _, b := range blocks {
			var err error
			switch b.Type {
			case "tool_use":
				if e.Type != "assistant" || b.Name != "Read" {
					return fail("unsupported acquisition; expected assistant Read")
				}
				hasCall = true
				err = r.Acquire(b.ID, b.Input.Path, line)
			case "tool_result":
				if e.Type != "user" {
					return fail("tool result must be a user event")
				}
				var text string
				if json.Unmarshal(b.Content, &text) != nil || string(b.Content) == "null" || len(b.Content) == 0 {
					return fail("observation must be text")
				}
				err = r.Observe(b.ToolUseID, text, line)
			case "text":
				if e.Type != "assistant" {
					return fail("unsupported user text block; supply one task string")
				}
				texts = append(texts, b.Text)
			case "thinking":
				if e.Type != "assistant" {
					return fail("unsupported user thinking block")
				}
			default:
				return fail("unsupported content block " + b.Type)
			}
			if err != nil {
				return fail(err.Error())
			}
		}
		switch e.Message.StopReason {
		case "": // A text message without explicit termination does not prove completion.
		case "tool_use":
			if !hasCall {
				return fail("tool_use stop reason without acquisition")
			}
		case "end_turn":
			final := strings.Join(texts, "\n")
			if e.Type != "assistant" || hasCall || strings.TrimSpace(final) == "" {
				return fail("end_turn requires a final text response")
			}
			r.FinalResponse = final
			r.ExecutionState = run.Completed
		default:
			return fail("unsupported stop reason " + e.Message.StopReason)
		}
	}
	if err := scanner.Err(); err != nil {
		return run.Run{}, fmt.Errorf("read trace: %w", err)
	}
	if r.Task == "" {
		return run.Run{}, fmt.Errorf("trace does not contain a task")
	}
	if err := validateNavigation(pointers, nodes); err != nil {
		return run.Run{}, err
	}
	return r, nil
}
