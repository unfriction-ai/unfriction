package compatreport

import (
	"bytes"
	"encoding/json"
	"io"
)

const MaxMetadataBytes = 64 * 1024
const MaxMetadataRecords = 64

// Capture observes only bytes already read for detection/parsing. It never reads
// ahead or reopens the trace just to discover a version.
type Capture struct {
	input   io.Reader
	prefix  []byte
	limited bool
}

func NewCapture(input io.Reader) *Capture { return &Capture{input: input} }
func (c *Capture) Read(p []byte) (int, error) {
	n, err := c.input.Read(p)
	keep := min(n, MaxMetadataBytes-len(c.prefix))
	c.prefix = append(c.prefix, p[:keep]...)
	c.limited = c.limited || len(c.prefix) == MaxMetadataBytes
	return n, err
}

// AgentVersion considers complete metadata records at or before the failing line.
// Conflicting or unsafe versions become unknown. No inference from file names.
func (c *Capture) AgentVersion(family string, throughLine int) string {
	version := "unknown"
	lines := bytes.Split(c.prefix, []byte{'\n'})
	if c.limited && len(c.prefix) > 0 && c.prefix[len(c.prefix)-1] != '\n' {
		lines = lines[:len(lines)-1]
	}
	count := 0
	for i, line := range lines {
		if throughLine > 0 && i+1 > throughLine {
			break
		}
		if count == MaxMetadataRecords {
			break
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		count++
		var e struct {
			Type       string          `json:"type"`
			SessionID  string          `json:"sessionId"`
			Version    json.RawMessage `json:"version"`
			Payload    json.RawMessage `json:"payload"`
			Message    json.RawMessage `json:"message"`
			Attachment json.RawMessage `json:"attachment"`
		}
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		var raw json.RawMessage
		switch family {
		case "codex":
			if e.Type != "session_meta" {
				continue
			}
			var payload map[string]json.RawMessage
			if json.Unmarshal(e.Payload, &payload) != nil {
				continue
			}
			raw = payload["cli_version"]
		case "claude-code":
			if e.SessionID == "" {
				continue
			}
			if !((e.Type == "user" || e.Type == "assistant") && metadataObject(e.Message)) && !(e.Type == "attachment" && metadataObject(e.Attachment)) {
				continue
			}
			raw = e.Version
		default:
			return "unknown"
		}
		if len(raw) == 0 {
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) != nil || SafeVersion(value) == "unknown" {
			return "unknown"
		}
		if version != "unknown" && version != value {
			return "unknown"
		}
		version = value
	}
	return version
}
func metadataObject(raw json.RawMessage) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal(raw, &object) == nil && object != nil
}
