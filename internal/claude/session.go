package claude

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

type sessionNode struct {
	event
	Subtype    string          `json:"subtype"`
	Version    json.RawMessage `json:"version"`
	CWD        string          `json:"cwd"`
	Branch     string          `json:"gitBranch"`
	Timestamp  string          `json:"timestamp"`
	BlockIndex *int            `json:"apiBlockIndex"`
	RequestID  string          `json:"requestId"`
	MessageID  string          `json:"-"`
}

type sessionGraph struct {
	doc       session.Document
	nodes     map[string]sessionNode
	owner     map[string]int
	positions map[string]int
}

func Summarize(input io.Reader) (session.Document, error) {
	g, err := readSession(input)
	return g.doc, err
}

func readSession(input io.Reader) (sessionGraph, error) {
	g := sessionGraph{doc: session.Document{Family: "claude-code"}, nodes: map[string]sessionNode{}, owner: map[string]int{}, positions: map[string]int{}}
	records, err := source.ReadAll(input)
	g.doc.Records = records
	if err != nil {
		return g, err
	}
	var pointers []navigationPointer
	type messageGroup struct {
		request, stop string
		last          int
	}
	groups := map[string]messageGroup{}
	for i, rec := range records {
		if strings.TrimSpace(string(rec.Data)) == "" {
			continue
		}
		var e sessionNode
		if json.Unmarshal(rec.Data, &e) != nil {
			return g, sessionError(rec, "invalid event")
		}
		if !knownSessionRecord(e) {
			return g, sessionError(rec, "unsupported session record or subtype")
		}
		if e.SessionID != "" {
			if g.doc.ID != "" && e.SessionID != g.doc.ID {
				return g, sessionError(rec, "conflicting session identity")
			}
			g.doc.ID = e.SessionID
		}
		if g.doc.Version == "" {
			_ = json.Unmarshal(e.Version, &g.doc.Version)
		}
		if g.doc.CWD == "" && e.CWD != "" {
			g.doc.CWD = e.CWD
			g.doc.Branch = e.Branch
		}
		if g.doc.Timestamp == "" && e.Timestamp != "" {
			g.doc.Timestamp = e.Timestamp
		}
		if e.Type == "last-prompt" {
			var fields map[string]json.RawMessage
			_ = json.Unmarshal(rec.Data, &fields)
			var id string
			if json.Unmarshal(fields["leafUuid"], &id) != nil || id == "" || e.SessionID == "" {
				return g, sessionError(rec, "invalid navigation leaf")
			}
			for key, value := range fields {
				if key == "lastPrompt" {
					var prompt string
					if json.Unmarshal(value, &prompt) != nil {
						return g, sessionError(rec, "invalid navigation prompt")
					}
				} else if key != "type" && key != "leafUuid" && key != "sessionId" {
					return g, sessionError(rec, "unsupported navigation field")
				}
			}
			pointers = append(pointers, navigationPointer{id: id, line: rec.Line})
			continue
		}
		if e.Type != "user" && e.Type != "assistant" && e.Type != "attachment" && e.Type != "system" {
			continue
		}
		if e.UUID == "" || e.SessionID == "" {
			return g, sessionError(rec, "ancestry node requires identity and session")
		}
		if _, ok := g.nodes[e.UUID]; ok {
			return g, sessionError(rec, "duplicate node identity")
		}
		var parent *string
		if len(e.Parent) == 0 || json.Unmarshal(e.Parent, &parent) != nil {
			return g, sessionError(rec, "missing explicit parent")
		}
		g.nodes[e.UUID] = e
		g.positions[e.UUID] = i
		if e.Type == "assistant" && e.BlockIndex != nil {
			var identity struct{ Message struct{ ID string } }
			if json.Unmarshal(rec.Data, &identity) != nil {
				return g, sessionError(rec, "invalid message identity")
			}
			e.MessageID = identity.Message.ID
			if e.MessageID == "" || e.RequestID == "" || *e.BlockIndex < 0 {
				return g, sessionError(rec, "message block requires identity, request and index")
			}
			if prev, ok := groups[e.MessageID]; ok {
				if prev.request != e.RequestID || prev.stop != e.Message.StopReason || *e.BlockIndex != prev.last+1 {
					return g, sessionError(rec, "conflicting message block identity, stop reason or index")
				}
			} else if *e.BlockIndex != 0 {
				return g, sessionError(rec, "message starts with missing blocks")
			}
			groups[e.MessageID] = messageGroup{request: e.RequestID, stop: e.Message.StopReason, last: *e.BlockIndex}
		}
		g.nodes[e.UUID] = e
		var prompt string
		if e.Type == "user" && json.Unmarshal(e.Message.Content, &prompt) == nil && string(e.Message.Content) != "null" {
			if e.Message.Role != "user" || strings.TrimSpace(prompt) == "" {
				return g, sessionError(rec, "invalid user prompt")
			}
			g.owner[e.UUID] = len(g.doc.Turns)
			g.doc.Turns = append(g.doc.Turns, session.Turn{ID: e.UUID, Prompt: prompt, Start: i, End: len(records), Timestamp: e.Timestamp})
		}
	}
	// Validate all recorded parent edges independently of prompt boundaries.
	colors := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if colors[id] == 1 {
			return fmt.Errorf("cycle in recorded ancestry")
		}
		if colors[id] == 2 {
			return nil
		}
		n, ok := g.nodes[id]
		if !ok {
			return fmt.Errorf("missing required ancestor")
		}
		colors[id] = 1
		var parent *string
		_ = json.Unmarshal(n.Parent, &parent)
		if parent != nil {
			if err := visit(*parent); err != nil {
				return err
			}
		}
		colors[id] = 2
		return nil
	}
	for id := range g.nodes {
		if err := visit(id); err != nil {
			return g, sessionError(records[g.positions[id]], err.Error())
		}
	}
	for _, p := range pointers {
		if _, ok := g.nodes[p.id]; !ok {
			return g, fmt.Errorf("line %d: unresolved navigation leaf", p.line)
		}
	}
	var owner func(string) int
	owner = func(id string) int {
		if n, ok := g.owner[id]; ok {
			return n
		}
		e := g.nodes[id]
		var p *string
		_ = json.Unmarshal(e.Parent, &p)
		n := -1
		if p != nil {
			n = owner(*p)
		}
		g.owner[id] = n
		return n
	}
	for id := range g.nodes {
		owner(id)
	}
	// Check associations over the complete graph before selecting any records.
	// Filtering first could hide a result reparented into another user turn.
	calls := map[string]string{}
	assistants := map[int]sessionNode{}
	messageOwners := map[string]int{}
	for _, rec := range records {
		var identity struct{ UUID string }
		_ = json.Unmarshal(rec.Data, &identity)
		e, ok := g.nodes[identity.UUID]
		if !ok {
			continue
		}
		var bs []block
		_ = json.Unmarshal(e.Message.Content, &bs)
		if e.Type == "assistant" {
			turn := g.owner[e.UUID]
			if e.MessageID != "" {
				if previous, ok := messageOwners[e.MessageID]; ok && previous != turn {
					return g, sessionError(rec, "message blocks cross user-turn ownership")
				}
				messageOwners[e.MessageID] = turn
			}
			if previous, ok := assistants[turn]; ok && (e.MessageID == "" || e.MessageID != previous.MessageID || e.RequestID != previous.RequestID) {
				continues := false
				for node := e; ; {
					var parent *string
					_ = json.Unmarshal(node.Parent, &parent)
					if parent == nil {
						break
					}
					if *parent == previous.UUID {
						continues = true
						break
					}
					node = g.nodes[*parent]
				}
				if !continues {
					return g, sessionError(rec, "ambiguous assistant continuations")
				}
			}
			assistants[turn] = e
			for _, b := range bs {
				if b.Type == "tool_use" {
					if b.ID == "" || calls[b.ID] != "" {
						return g, sessionError(rec, "missing or duplicate tool identity")
					}
					calls[b.ID] = e.UUID
				}
			}
		}
		if e.Type == "user" {
			for _, b := range bs {
				if b.Type == "tool_result" {
					request, ok := calls[b.ToolUseID]
					if !ok || g.owner[request] != g.owner[e.UUID] {
						return g, sessionError(rec, "tool result crosses turn ownership or lacks a recorded request")
					}
				}
			}
		}
	}
	for i := range g.doc.Turns {
		t := &g.doc.Turns[i]
		if i+1 < len(g.doc.Turns) {
			t.End = g.doc.Turns[i+1].Start
		}
		for _, rec := range records {
			var e sessionNode
			_ = json.Unmarshal(rec.Data, &e)
			if e.Type != "assistant" || g.owner[e.UUID] != i {
				continue
			}
			var bs []block
			_ = json.Unmarshal(e.Message.Content, &bs)
			if e.Message.StopReason == "end_turn" {
				for _, b := range bs {
					if b.Type == "text" {
						if t.Final != "" {
							t.Final += "\n"
						}
						t.Final += b.Text
						t.Completed = true
					}
				}
			}
		}
		var parent *string
		_ = json.Unmarshal(g.nodes[t.ID].Parent, &parent)
		if parent != nil {
			previous := g.owner[*parent]
			if previous >= 0 && previous < i {
				t.PreviousTurnID = g.doc.Turns[previous].ID
				t.PreviousAgent = g.doc.Turns[previous].Final
				t.EarlierUser = g.doc.Turns[previous].Prompt
			}
		}
	}
	if len(g.doc.Turns) == 0 {
		return g, fmt.Errorf("session contains no user turns")
	}
	return g, nil
}

func knownSessionRecord(e sessionNode) bool {
	switch e.Type {
	case "user", "assistant", "last-prompt", "file-history-snapshot", "file-history-delta", "progress", "queue-operation", "mode", "permission-mode", "atis-latch", "ai-title", "api-request-shape", "api-request-blob", "api-request", "cost-state":
		return true
	case "system":
		switch e.Subtype {
		case "stop_hook_summary", "turn_duration", "local_command", "away_summary":
			return true
		}
	case "attachment":
		var a struct{ Type string }
		if json.Unmarshal(e.Attachment, &a) != nil {
			return false
		}
		switch a.Type {
		case "hook_success", "hook_system_message", "environment", "model", "deferred_tools_delta", "agent_listing_delta", "mcp_instructions_delta", "skill_listing", "total_tokens_reminder", "instructions", "session_context", "date", "credential_org", "remote_session_change", "prompt_snapshot", "deferred_tools_record", "auto_mode":
			return true
		}
	}
	return false
}

func sessionError(rec source.Record, message string) error {
	return diagnostic.Adapter(diagnostic.Unsupported(fmt.Errorf("line %d: %s", rec.Line, message)), rec.Line, diagnostic.RecordFromJSON(rec.Data))
}

func ParseTurn(input io.Reader, id string) (run.Run, session.Document, int, error) {
	g, err := readSession(input)
	d := g.doc
	if err != nil {
		return run.Run{}, d, 0, err
	}
	turn, index, err := d.Find(id)
	if err != nil {
		return run.Run{}, d, 0, err
	}
	// Context ancestry must be mainline too, even when its activity is not part
	// of the selected turn. The graph has already been checked for cycles.
	for ancestor := id; ancestor != ""; {
		n := g.nodes[ancestor]
		if n.Sidechain {
			return run.Run{}, d, index, sessionError(d.Records[g.positions[ancestor]], "sidechain ancestor unsupported")
		}
		var parent *string
		_ = json.Unmarshal(n.Parent, &parent)
		ancestor = ""
		if parent != nil {
			ancestor = *parent
		}
	}
	var selected []source.Record
	var coverage []run.Coverage
	for position, rec := range d.Records {
		var e sessionNode
		_ = json.Unmarshal(rec.Data, &e)
		owned, ok := g.owner[e.UUID]
		if !ok && e.Type != "" && position >= turn.Start && position < turn.End {
			coverage = append(coverage, run.Coverage{SourceLine: rec.Line, Summary: "Session bookkeeping recorded near this turn; not attributed as task activity or completion."})
		}
		if !ok || owned != index {
			continue
		}
		if e.Sidechain {
			return run.Run{}, d, index, sessionError(rec, "sidechain selection unsupported")
		}
		if e.Type == "user" || e.Type == "assistant" {
			selected = append(selected, rec)
		} else {
			coverage = append(coverage, run.Coverage{SourceLine: rec.Line, Summary: "Session ancestry/context record retained; contents are not measured acquisitions."})
		}
	}
	r, err := parse(source.Records(selected), true)
	if err == nil {
		r.Coverage = append(r.Coverage, coverage...)
	}
	return r, d, index, err
}
