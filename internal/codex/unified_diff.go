package codex

import (
	"regexp"
	"strconv"
	"strings"
)

var unifiedHunk = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@$`)

// validUnifiedDiff validates the recorded single-file update grammar and hunk
// lengths. It does not read a file, apply the diff, or establish correctness.
func validUnifiedDiff(diff string) bool {
	lines := strings.Split(strings.TrimSuffix(diff, "\n"), "\n")
	oldLeft, newLeft := 0, 0
	inHunk, changed := false, false
	for _, line := range lines {
		if m := unifiedHunk.FindStringSubmatch(line); m != nil {
			if inHunk && (oldLeft != 0 || newLeft != 0 || !changed) {
				return false
			}
			oldStart, err := strconv.Atoi(m[1])
			if err != nil {
				return false
			}
			newStart, err := strconv.Atoi(m[3])
			if err != nil {
				return false
			}
			oldLeft, newLeft = 1, 1
			if m[2] != "" {
				oldLeft, err = strconv.Atoi(m[2])
				if err != nil {
					return false
				}
			}
			if m[4] != "" {
				newLeft, err = strconv.Atoi(m[4])
				if err != nil {
					return false
				}
			}
			if (oldStart == 0 && oldLeft != 0) || (newStart == 0 && newLeft != 0) {
				return false
			}
			inHunk, changed = true, false
			continue
		}
		if !inHunk || len(line) == 0 {
			return false
		}
		switch line[0] {
		case ' ':
			oldLeft--
			newLeft--
		case '-':
			oldLeft--
			changed = true
		case '+':
			newLeft--
			changed = true
		default:
			return false
		}
		if oldLeft < 0 || newLeft < 0 {
			return false
		}
	}
	return inHunk && changed && oldLeft == 0 && newLeft == 0
}
