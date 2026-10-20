package codex

import (
	"encoding/json"
	"math"

	"github.com/unfriction-ai/unfriction/internal/run"
)

// Numeric support is deliberately pinned to inspected recording/source evidence.
// Other versions still reconstruct normally, but their optional usage is unknown.
type usageCollector struct {
	version         string
	versionSeen     bool
	versionConflict bool
	latest          *run.ModelUsage
	invalid         bool
	previous        map[string]int64
}

var usageNames = []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens"}

func (u *usageCollector) consume(data []byte, active bool, line int) {
	var record struct {
		Type    string
		Payload struct {
			Type       string
			CLIVersion json.RawMessage `json:"cli_version"`
			Info       json.RawMessage
		}
	}
	if json.Unmarshal(data, &record) != nil {
		return
	}
	if record.Type == "session_meta" {
		var version string
		_ = json.Unmarshal(record.Payload.CLIVersion, &version)
		if u.versionSeen && u.version != version {
			u.versionConflict = true
			u.invalid = true
		}
		u.version, u.versionSeen = version, true
		return
	}
	if record.Type != "event_msg" || record.Payload.Type != "token_count" || !active {
		return
	}
	entry := run.ModelUsage{Scope: "session cumulative snapshot", SourceLine: line}
	u.latest = &entry
	if u.version != "0.142.0-alpha.1" || u.versionConflict {
		entry.Unavailable = "unverified recording version"
		return
	}
	var info map[string]json.RawMessage
	if json.Unmarshal(record.Payload.Info, &info) != nil || info == nil {
		entry.Unavailable = "missing or malformed usage snapshot"
		u.invalid = true
		return
	}
	var total map[string]json.RawMessage
	if json.Unmarshal(info["total_token_usage"], &total) != nil || total == nil {
		entry.Unavailable = "missing or malformed cumulative counters"
		u.invalid = true
		return
	}
	for name := range total {
		known := false
		for _, n := range usageNames {
			known = known || name == n
		}
		if !known {
			entry.Unavailable = "unverified counter categories"
			u.invalid = true
			return
		}
	}
	current := map[string]int64{}
	for _, name := range usageNames {
		counter := run.UsageCounter{Name: name}
		var value int64
		raw := total[name]
		if len(raw) > 0 && string(raw) != "null" && json.Unmarshal(raw, &value) == nil && value >= 0 {
			counter.Tokens = &value
			current[name] = value
			if previous, ok := u.previous[name]; ok && value < previous {
				u.invalid = true
			}
		}
		entry.Counters = append(entry.Counters, counter)
	}
	input, hasInput := current["input_tokens"]
	output, hasOutput := current["output_tokens"]
	sum, hasTotal := current["total_tokens"]
	// Without both components the aggregate cannot be distinguished from the
	// vendor synthetic context-window saturation path. Keep it unavailable.
	if !hasInput || !hasOutput {
		entry.Counters[4].Tokens = nil
	}
	if hasInput && hasOutput && hasTotal && (input > math.MaxInt64-output || input+output != sum) {
		u.invalid = true
	}
	if cache, ok := current["cached_input_tokens"]; ok && hasInput && cache > input {
		u.invalid = true
	}
	if reasoning, ok := current["reasoning_output_tokens"]; ok && hasOutput && reasoning > output {
		u.invalid = true
	}
	if u.previous == nil {
		u.previous = map[string]int64{}
	}
	for name, value := range current {
		u.previous[name] = value
	}
}

func (u *usageCollector) entries() []run.ModelUsage {
	if u.latest == nil {
		return nil
	}
	entry := *u.latest
	if u.invalid {
		entry.Counters = nil
		entry.Unavailable = "unverified or inconsistent cumulative snapshots"
	}
	return []run.ModelUsage{entry}
}
