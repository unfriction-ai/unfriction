package claude

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/run"
)

type usageCollector struct {
	entries   []run.ModelUsage
	ids       map[string]int
	snapshots map[string]map[string]json.RawMessage
}

var usageNames = []string{"input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens"}

func (u *usageCollector) consume(data []byte, line int) {
	var record struct {
		Version json.RawMessage
		Message struct {
			ID    json.RawMessage
			Usage json.RawMessage
		}
	}
	if json.Unmarshal(data, &record) != nil || len(record.Message.Usage) == 0 {
		return
	}
	entry := run.ModelUsage{Scope: "model-message snapshot", SourceLine: line}
	var version, id string
	_ = json.Unmarshal(record.Version, &version)
	_ = json.Unmarshal(record.Message.ID, &id)
	var fields map[string]json.RawMessage
	if json.Unmarshal(record.Message.Usage, &fields) != nil || fields == nil {
		entry.Unavailable = "missing or malformed message usage"
	}
	if version != "2.1.302" {
		entry.Unavailable = "unverified recording version"
	}
	if strings.TrimSpace(id) == "" {
		entry.Unavailable = "missing model-message identity"
	}
	selected := map[string]json.RawMessage{}
	for _, name := range usageNames {
		raw := fields[name]
		selected[name] = raw
		counter := run.UsageCounter{Name: name}
		var value int64
		if len(raw) > 0 && string(raw) != "null" && json.Unmarshal(raw, &value) == nil && value >= 0 {
			counter.Tokens = &value
		}
		entry.Counters = append(entry.Counters, counter)
	}
	if u.ids == nil {
		u.ids = map[string]int{}
		u.snapshots = map[string]map[string]json.RawMessage{}
	}
	if strings.TrimSpace(id) != "" {
		if index, ok := u.ids[id]; ok {
			previous := &u.entries[index]
			// Only identical verified snapshots are supported. Changed values are not
			// silently interpreted as deltas or an authoritative later snapshot.
			if entry.Unavailable != "" || !reflect.DeepEqual(selected, u.snapshots[id]) {
				previous.Counters = nil
				previous.Unavailable = "conflicting or unverified model-message snapshots"
			}
			return
		}
		u.ids[id] = len(u.entries)
		u.snapshots[id] = selected
	}
	if entry.Unavailable != "" {
		entry.Counters = nil
	}
	u.entries = append(u.entries, entry)
}
