// Package codex is the input boundary for Codex rollout JSONL events.
package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/run"
)

type event struct {
	Type    string `json:"type"`
	Payload struct {
		Type             string `json:"type"`
		TurnID           string `json:"turn_id"`
		Message          string `json:"message"`
		Phase            string `json:"phase"`
		LastAgentMessage string `json:"last_agent_message"`
	} `json:"payload"`
}

// Parse reconstructs a single turn. A final answer does not prove execution
// completion: only a matching task_complete event can set Completed.
func Parse(input io.Reader) (run.Run, error) {
	r := run.Run{ExecutionState: run.Incomplete}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var turnID string
	var hasTask, hasFinal bool
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
		p := e.Payload
		if e.Type == "response_item" && (strings.HasSuffix(p.Type, "_call") || strings.HasSuffix(p.Type, "_call_output")) {
			return run.Run{}, fmt.Errorf("line %d: tool calls are not supported in M1", line)
		}
		if e.Type != "event_msg" {
			continue // Session metadata, context, and duplicate response items.
		}
		switch p.Type {
		case "task_started":
			if turnID != "" {
				return run.Run{}, fmt.Errorf("line %d: multiple turns are not supported; supply one turn", line)
			}
			if p.TurnID == "" {
				return run.Run{}, fmt.Errorf("line %d: missing turn ID", line)
			}
			turnID = p.TurnID
		case "user_message", "agent_message", "task_complete":
			if turnID == "" || r.ExecutionState == run.Completed {
				return run.Run{}, fmt.Errorf("line %d: %s outside an active turn", line, p.Type)
			}
			switch p.Type {
			case "user_message":
				if hasTask || hasFinal || strings.TrimSpace(p.Message) == "" {
					return run.Run{}, fmt.Errorf("line %d: expected one nonempty task before the final response", line)
				}
				r.Task, hasTask = p.Message, true
			case "agent_message":
				if p.Phase == "final_answer" {
					if !hasTask || hasFinal || strings.TrimSpace(p.Message) == "" {
						return run.Run{}, fmt.Errorf("line %d: expected one nonempty final response after the task", line)
					}
					r.FinalResponse, hasFinal = p.Message, true
				}
			case "task_complete":
				if p.TurnID != turnID || !hasFinal || p.LastAgentMessage != r.FinalResponse {
					return run.Run{}, fmt.Errorf("line %d: completion must match the active turn and final response", line)
				}
				r.ExecutionState = run.Completed
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return run.Run{}, fmt.Errorf("read trace: %w", err)
	}
	if !hasTask {
		return run.Run{}, fmt.Errorf("trace does not contain a task in a started turn")
	}
	return r, nil
}
