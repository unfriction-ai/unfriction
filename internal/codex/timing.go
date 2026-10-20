package codex

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/run"
)

var decimalSeconds = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

// Optional timing is conservative: malformed metadata does not invalidate an
// otherwise supported trace and never becomes a measured zero.
func envelopeDuration(output string) *run.RecordedDuration {
	header, _, ok := strings.Cut(output, "\nOutput:\n")
	if !ok {
		return nil
	}
	var value string
	count := 0
	for _, line := range strings.Split(header, "\n") {
		if !strings.HasPrefix(line, "Wall time:") {
			continue
		}
		count++
		text, ok := strings.CutPrefix(line, "Wall time: ")
		if !ok {
			return nil
		}
		text, ok = strings.CutSuffix(text, " seconds")
		if !ok || !decimalSeconds.MatchString(text) {
			return nil
		}
		value = text
	}
	if count != 1 {
		return nil
	}
	// Validate finite representability, but preserve the recorded decimal string.
	if _, err := strconv.ParseFloat(value, 64); err != nil {
		return nil
	}
	return &run.RecordedDuration{Seconds: value}
}

func structuredDuration(raw json.RawMessage) *run.RecordedDuration {
	var value struct {
		Seconds *uint64 `json:"secs"`
		Nanos   *uint32 `json:"nanos"`
	}
	if json.Unmarshal(raw, &value) != nil || value.Seconds == nil || value.Nanos == nil || *value.Nanos >= 1000000000 {
		return nil
	}
	return &run.RecordedDuration{Seconds: fmt.Sprintf("%d.%09d", *value.Seconds, *value.Nanos)}
}
