// Package traceformat selects an input adapter from bounded structural evidence.
// It does not validate vendor schemas or retry a failed adapter.
package traceformat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/diagnostic"
)

const (
	MaxPrefixBytes   = 64 * 1024
	MaxPrefixRecords = 64
)

const overrideHint = "specify --format codex or --format claude-code"

// Detect inspects at most 64 KiB and the first 64 nonblank complete JSONL records.
// Malformed records count toward the limit but are left for adapter validation.
// On success, replay yields the original prefix followed by the untouched input.
func Detect(input io.Reader) (format string, replay io.Reader, err error) {
	prefix, err := io.ReadAll(io.LimitReader(input, MaxPrefixBytes))
	if err != nil {
		return "", nil, fmt.Errorf("read trace prefix: %w", err)
	}
	lines := bytes.Split(prefix, []byte{'\n'})
	byteLimit := len(prefix) == MaxPrefixBytes
	if byteLimit && prefix[len(prefix)-1] != '\n' {
		// The final line may continue beyond the byte budget. Never classify a fragment.
		lines = lines[:len(lines)-1]
	}
	codex, claude := false, false
	records := 0
	for _, line := range lines {
		if records == MaxPrefixRecords {
			break
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		records++
		var record struct {
			Type      string          `json:"type"`
			Payload   json.RawMessage `json:"payload"`
			Message   json.RawMessage `json:"message"`
			SessionID json.RawMessage `json:"sessionId"`
		}
		if json.Unmarshal(line, &record) != nil {
			continue
		}
		switch record.Type {
		case "event_msg", "response_item":
			codex = codex || object(record.Payload)
		case "user", "assistant":
			var session string
			if json.Unmarshal(record.SessionID, &session) == nil && strings.TrimSpace(session) != "" {
				claude = claude || object(record.Message)
			}
		}
	}
	if codex && claude {
		return "", nil, diagnostic.Detection(diagnostic.AmbiguousFormat, fmt.Errorf("ambiguous trace format: prefix contains Codex and Claude Code evidence; %s", overrideHint))
	}
	switch {
	case codex:
		format = "codex"
	case claude:
		format = "claude-code"
	case byteLimit || records == MaxPrefixRecords:
		return "", nil, diagnostic.Detection(diagnostic.UndeterminedFormat, fmt.Errorf("trace format undetermined: prefix limit reached (%d bytes or %d nonblank records); %s", MaxPrefixBytes, MaxPrefixRecords, overrideHint))
	default:
		return "", nil, diagnostic.Detection(diagnostic.UnknownFormat, fmt.Errorf("unknown trace format: no distinctive structural evidence; %s", overrideHint))
	}
	return format, io.MultiReader(bytes.NewReader(prefix), input), nil
}

func object(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 0 && raw[0] == '{'
}
