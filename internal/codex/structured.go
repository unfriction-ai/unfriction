package codex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/diagnostic"
	"github.com/unfriction-ai/unfriction/internal/run"
)

// completedItems tracks one structured representation of a reduced turn. It
// intentionally does not reconcile duplicate legacy tool/wrapper records.
type completedItems struct {
	thread string
	ids    map[string]bool
}

type completedItem struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Phase   string `json:"phase"`
	Content []struct {
		Type         string          `json:"type"`
		Text         string          `json:"text"`
		TextElements json.RawMessage `json:"text_elements"`
	} `json:"content"`
	Command         []string        `json:"command"`
	Source          string          `json:"source"`
	Status          string          `json:"status"`
	Output          *string         `json:"aggregated_output"`
	ExitCode        *int            `json:"exit_code"`
	Stdout          *string         `json:"stdout"`
	Stderr          *string         `json:"stderr"`
	FormattedOutput json.RawMessage `json:"formatted_output"`
	Changes         map[string]struct {
		Type     string          `json:"type"`
		Diff     string          `json:"unified_diff"`
		MovePath json.RawMessage `json:"move_path"`
	} `json:"changes"`
}

func (s *completedItems) consume(r *run.Run, raw json.RawMessage, thread, turn, activeTurn string, line int) error {
	unsupported := func(message string) error { return diagnostic.Unsupported(fmt.Errorf("%s", message)) }
	if activeTurn == "" || r.ExecutionState == run.Completed {
		return fmt.Errorf("item_completed outside an active turn")
	}
	if turn == "" || turn != activeTurn {
		return fmt.Errorf("structured item must match the active turn")
	}
	if strings.TrimSpace(thread) == "" || (s.thread != "" && s.thread != thread) {
		return fmt.Errorf("structured item requires one matching thread ID")
	}
	var item completedItem
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("missing completed item")
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		return fmt.Errorf("invalid completed item: %w", err)
	}
	if strings.TrimSpace(item.ID) == "" {
		return fmt.Errorf("missing structured item ID")
	}
	if s.ids[item.ID] {
		return fmt.Errorf("duplicate structured item ID %q", item.ID)
	}
	s.thread = thread
	if s.ids == nil {
		s.ids = make(map[string]bool)
	}
	s.ids[item.ID] = true
	switch item.Type {
	case "UserMessage", "AgentMessage":
		textType := "text"
		if item.Type == "AgentMessage" {
			textType = "Text"
		}
		if len(item.Content) != 1 || item.Content[0].Type != textType || strings.TrimSpace(item.Content[0].Text) == "" {
			return unsupported("structured message requires one nonempty text block")
		}
		elements := bytes.TrimSpace(item.Content[0].TextElements)
		if len(elements) > 0 {
			var entries []json.RawMessage
			if json.Unmarshal(elements, &entries) != nil || entries == nil || len(entries) != 0 {
				return unsupported("unsupported structured text elements")
			}
		}
		if item.Type == "UserMessage" {
			if r.Task != "" || r.FinalResponse != "" {
				return fmt.Errorf("expected one task before the final response")
			}
			r.Task = item.Content[0].Text
		} else {
			if item.Phase != "final_answer" {
				return unsupported("unsupported structured agent phase; expected final_answer")
			}
			if r.Task == "" || r.FinalResponse != "" {
				return fmt.Errorf("expected one final response after the task")
			}
			r.FinalResponse = item.Content[0].Text
		}
	case "CommandExecution":
		if len(item.Command) != 3 || item.Command[0] != "/bin/zsh" || item.Command[1] != "-lc" || acquisitionTarget(item.Command[2]) == "" {
			return unsupported("unsupported structured command; expected /bin/zsh -lc with a supported literal file read")
		}
		if item.Source != "unified_exec_startup" {
			return unsupported("unsupported structured command source")
		}
		if item.Output == nil || item.ExitCode == nil || item.Stdout != nil || item.Stderr != nil || !absentOrNull(item.FormattedOutput) {
			return fmt.Errorf("missing or ambiguous structured command feedback")
		}
		if item.Status != "completed" || *item.ExitCode != 0 {
			return unsupported("unsupported structured command status/exit combination")
		}
		if err := r.Acquire(item.ID, acquisitionTarget(item.Command[2]), line); err != nil {
			return err
		}
		if err := r.Observe(item.ID, *item.Output, line); err != nil {
			return err
		}
		id := r.Events[len(r.Events)-1].AcquisitionID
		r.Coverage = append(r.Coverage, run.Coverage{SourceLine: line, Summary: fmt.Sprintf("Combined record supplies acquisition %s and its observation; record order does not establish operation start order.", id)})
	case "FileChange":
		if len(item.Changes) != 1 {
			return unsupported("structured file change requires one update target")
		}
		if item.Stdout == nil || item.Stderr == nil {
			return fmt.Errorf("missing structured file change feedback")
		}
		if item.Status != "completed" || *item.Stderr != "" {
			return unsupported("unsupported structured file change status/feedback combination")
		}
		if item.Output != nil || item.ExitCode != nil || !absentOrNull(item.FormattedOutput) {
			return fmt.Errorf("ambiguous structured file change feedback")
		}
		for target, change := range item.Changes {
			if change.Type != "update" || !literalPath.MatchString(target) {
				return unsupported("unsupported structured update target or change kind")
			}
			if !bytes.Equal(bytes.TrimSpace(change.MovePath), []byte("null")) {
				return unsupported("unsupported rename; structured update requires move_path=null")
			}
			if !validUnifiedDiff(change.Diff) {
				return unsupported("unsupported structured unified diff")
			}
			if err := r.Act(item.ID, target, line); err != nil {
				return err
			}
			if err := r.RecordResult(item.ID, *item.Stdout, false, line); err != nil {
				return err
			}
		}
		id := r.Events[len(r.Events)-1].ActionID
		r.Coverage = append(r.Coverage, run.Coverage{SourceLine: line, Summary: fmt.Sprintf("Combined record supplies action %s and its result; record order does not establish operation start order.", id)})
	default:
		return unsupported("unsupported completed item kind; expected UserMessage, AgentMessage, CommandExecution, or FileChange")
	}
	return nil
}

func absentOrNull(raw json.RawMessage) bool {
	return len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
