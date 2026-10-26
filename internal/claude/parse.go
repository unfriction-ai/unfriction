// Package claude imports a reduced, single-turn Claude Code session JSONL trace.
package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/diagnostic"
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
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	IsError   json.RawMessage `json:"is_error"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	Text      string          `json:"text"`
}

func Parse(input io.Reader) (result run.Run, parseErr error) {
	currentLine := 0
	var currentRecord diagnostic.Record
	reportFailure := true
	defer func() {
		if parseErr != nil && reportFailure {
			parseErr = diagnostic.Adapter(parseErr, currentLine, currentRecord)
		}
	}()
	r := run.Run{ExecutionState: run.Incomplete}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	session := ""
	var pointers []navigationPointer
	var nodes []navigationNode
	var usage usageCollector
	for line := 1; scanner.Scan(); line++ {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		currentLine = line
		currentRecord = diagnostic.RecordFromJSON(scanner.Bytes())
		var e event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return run.Run{}, fmt.Errorf("line %d: invalid event: %w", line, err)
		}
		if e.Type == "" {
			return run.Run{}, fmt.Errorf("line %d: missing event type", line)
		}
		fail := func(message string) (run.Run, error) { return run.Run{}, fmt.Errorf("line %d: %s", line, message) }
		unsupported := func(message string) (run.Run, error) {
			return run.Run{}, diagnostic.Unsupported(fmt.Errorf("line %d: %s", line, message))
		}
		switch e.Type {
		case "user", "assistant", "last-prompt", "attachment":
		case "file-history-snapshot", "progress", "queue-operation":
			continue
		default:
			return unsupported("unsupported event " + e.Type + "; supply a reduced single turn")
		}
		if e.Sidechain {
			return unsupported("sidechain runs are unsupported")
		}
		if e.SessionID == "" {
			return fail("missing session ID")
		}
		if session == "" {
			session = e.SessionID
		} else if session != e.SessionID {
			return unsupported("multiple sessions are unsupported")
		}
		if e.Type == "last-prompt" {
			leaf, err := validateLastPrompt(scanner.Bytes())
			if err != nil {
				return run.Run{}, fmt.Errorf("line %d: %w", line, err)
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
				return run.Run{}, fmt.Errorf("line %d: %w", line, err)
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
		if e.Type == "assistant" {
			usage.consume(scanner.Bytes(), line)
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
				if e.Type != "assistant" || (b.Name != "Read" && b.Name != "Edit") {
					return unsupported("unsupported acquisition or action; expected assistant Read or supported Edit")
				}
				hasCall = true
				if b.Name == "Read" {
					var input struct {
						Path string `json:"file_path"`
					}
					if json.Unmarshal(b.Input, &input) != nil {
						return fail("invalid Read input")
					}
					err = r.Acquire(b.ID, input.Path, line)
				} else {
					target, parseErr := editTarget(b.Input)
					if parseErr != nil {
						return unsupported(parseErr.Error())
					}
					err = r.Act(b.ID, target, line)
				}
			case "tool_result":
				if e.Type != "user" {
					return fail("tool result must be a user event")
				}
				var isError bool
				if len(b.IsError) > 0 && (string(b.IsError) == "null" || json.Unmarshal(b.IsError, &isError) != nil) {
					return fail("tool result is_error must be Boolean when present")
				}
				var text string
				if json.Unmarshal(b.Content, &text) == nil && string(b.Content) != "null" {
					err = r.RecordResult(b.ToolUseID, text, isError, line)
				} else {
					image, parseErr := imageObservation(b.Content)
					if parseErr != nil {
						return fail(parseErr.Error())
					}
					err = r.ObserveImage(b.ToolUseID, image, isError, line)
				}
			case "text":
				if e.Type != "assistant" {
					return unsupported("unsupported user text block; supply one task string")
				}
				texts = append(texts, b.Text)
			case "thinking":
				if e.Type != "assistant" {
					return unsupported("unsupported user thinking block")
				}
			default:
				return unsupported("unsupported content block " + b.Type)
			}
			if err != nil {
				return run.Run{}, fmt.Errorf("line %d: %w", line, err)
			}
		}
		switch e.Message.StopReason {
		case "": // A text message without explicit termination does not prove completion.
		case "tool_use":
			if !hasCall {
				return fail("tool_use stop reason without acquisition or action")
			}
		case "end_turn":
			final := strings.Join(texts, "\n")
			if e.Type != "assistant" || hasCall || strings.TrimSpace(final) == "" {
				return fail("end_turn requires a final text response")
			}
			r.FinalResponse = final
			r.ExecutionState = run.Completed
		default:
			return unsupported("unsupported stop reason " + e.Message.StopReason)
		}
	}
	if err := scanner.Err(); err != nil {
		reportFailure = false
		return run.Run{}, fmt.Errorf("read trace: %w", err)
	}
	if r.Task == "" {
		return run.Run{}, fmt.Errorf("trace does not contain a task")
	}
	if err := validateNavigation(pointers, nodes); err != nil {
		currentLine = 0
		currentRecord = diagnostic.Record{}
		return run.Run{}, err
	}
	r.ModelUsage = usage.entries
	return r, nil
}
