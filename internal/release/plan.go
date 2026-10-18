// Package release contains the release policy used by CI, not application code.
package release

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var headerPattern = regexp.MustCompile(`^([a-z]+)(\([^()\r\n]+\))?(!)?: .+`)
var breakingPattern = regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE: .+`)

type Plan struct {
	Version string `json:"version"`
	Tag     string `json:"tag"`
	Notes   string `json:"notes"`
}

// Next starts at 0.1.0 only when a release-driving commit exists. Subsequent
// breaking changes bump the major, including 0.x -> 1.0.0. No manual bootstrap
// tag is needed. Public notes deliberately omit private commit text and links.
func Next(previous string, messages []string) (Plan, error) {
	var parts [3]int
	if previous != "" {
		match := versionPattern.FindStringSubmatch(previous)
		if match == nil {
			return Plan{}, fmt.Errorf("invalid release version %q", previous)
		}
		for i := range parts {
			v, err := strconv.Atoi(match[i+1])
			if err != nil {
				return Plan{}, err
			}
			parts[i] = v
		}
	}
	level, fixes, features, breaking := 0, 0, 0, 0
	for _, message := range messages {
		header := strings.SplitN(message, "\n", 2)[0]
		match := headerPattern.FindStringSubmatch(header)
		if match == nil {
			continue
		}
		switch {
		case match[3] == "!" || breakingPattern.MatchString(message):
			level = 3
			breaking++
		case match[1] == "feat":
			level = max(level, 2)
			features++
		case match[1] == "fix":
			level = max(level, 1)
			fixes++
		}
	}
	if level == 0 {
		return Plan{}, nil
	}
	if previous == "" {
		parts = [3]int{0, 1, 0}
	} else {
		switch level {
		case 3:
			parts = [3]int{parts[0] + 1, 0, 0}
		case 2:
			parts = [3]int{parts[0], parts[1] + 1, 0}
		case 1:
			parts[2]++
		}
	}
	v := fmt.Sprintf("%d.%d.%d", parts[0], parts[1], parts[2])
	return Plan{Version: v, Tag: "v" + v, Notes: fmt.Sprintf("unfriction %s\n\nChanges: %d feature(s), %d fix(es), %d breaking change(s).\n\nInstall: `brew install unfriction-ai/tap/unfriction`\n\nExecution completion does not imply verified task success.\n", v, features, fixes, breaking)}, nil
}
