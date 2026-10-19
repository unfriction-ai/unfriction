// Package codex is the input boundary for Codex rollout JSONL events.
package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/run"
)

type event struct {
	Type    string `json:"type"`
	Payload struct {
		Type             string  `json:"type"`
		Name             string  `json:"name"`
		CallID           string  `json:"call_id"`
		Arguments        string  `json:"arguments"`
		Output           *string `json:"output"`
		TurnID           string  `json:"turn_id"`
		Message          string  `json:"message"`
		Phase            string  `json:"phase"`
		LastAgentMessage string  `json:"last_agent_message"`
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
		if e.Type == "world_state" || (e.Type == "response_item" && p.Type == "compaction") {
			notice, err := contextRecord(scanner.Bytes(), line)
			if err != nil {
				return run.Run{}, fmt.Errorf("line %d: %w", line, err)
			}
			r.Coverage = append(r.Coverage, notice)
			continue
		}
		if e.Type == "response_item" && (strings.HasSuffix(p.Type, "_call") || strings.HasSuffix(p.Type, "_call_output")) {
			var err error
			switch p.Type {
			case "function_call":
				var args struct {
					Cmd string `json:"cmd"`
				}
				if p.CallID == "" && p.Name == "" {
					return run.Run{}, fmt.Errorf("line %d: tool calls are not supported without an acquisition name and ID", line)
				}
				if json.Unmarshal([]byte(p.Arguments), &args) != nil || p.Name != "exec_command" || !readCommand.MatchString(args.Cmd) {
					return run.Run{}, fmt.Errorf("line %d: unsupported acquisition; expected exec_command with cat <path>", line)
				}
				err = r.Acquire(p.CallID, strings.TrimPrefix(args.Cmd, "cat "), line)
			case "function_call_output":
				if p.Output == nil {
					return run.Run{}, fmt.Errorf("line %d: missing observation output", line)
				}
				// The observed exec_command variant wraps stdout with execution metadata.
				// Keep the observation body, never infer run completion from the exit code.
				_, body, ok := strings.Cut(*p.Output, "\nOutput:\n")
				if !ok || !strings.HasPrefix(*p.Output, "Chunk ID:") || !strings.Contains(*p.Output, "\nProcess exited with code ") {
					return run.Run{}, fmt.Errorf("line %d: unsupported observation envelope", line)
				}
				err = r.Observe(p.CallID, body, line)
			default:
				return run.Run{}, fmt.Errorf("line %d: unsupported tool event %q", line, p.Type)
			}
			if err != nil {
				return run.Run{}, fmt.Errorf("line %d: %w", line, err)
			}
			continue
		}
		if e.Type != "event_msg" {
			switch e.Type {
			case "session_meta", "turn_context":
				continue
			case "response_item":
				if p.Type == "message" || p.Type == "reasoning" {
					continue
				}
			}
			return run.Run{}, fmt.Errorf("line %d: unsupported event %q/%q", line, e.Type, p.Type)
		}
		switch p.Type {
		case "token_count", "agent_reasoning": // Known non-lifecycle records.
			continue
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
		default:
			return run.Run{}, fmt.Errorf("line %d: unsupported lifecycle event %q", line, p.Type)
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

// Deliberately accepts one literal relative or absolute path, without shell syntax.
var readCommand = regexp.MustCompile(`^cat [a-zA-Z0-9_./][a-zA-Z0-9_./-]*$`)
