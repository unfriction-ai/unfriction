package codex

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Only the recorded single-file update grammar is supported. This parses the
// request target; it never opens the target or attempts to apply the patch.
func patchTarget(input string) (string, error) {
	lines := strings.Split(strings.TrimSuffix(input, "\n"), "\n")
	fail := func() (string, error) {
		return "", fmt.Errorf("unsupported patch; expected one Update File with replacement hunks")
	}
	if len(lines) < 6 || lines[0] != "*** Begin Patch" || lines[len(lines)-1] != "*** End Patch" || !strings.HasPrefix(lines[1], "*** Update File: ") {
		return fail()
	}
	target := strings.TrimPrefix(lines[1], "*** Update File: ")
	if !literalPath.MatchString(target) {
		return fail()
	}
	inHunk, removed, added := false, false, false
	for _, line := range lines[2 : len(lines)-1] {
		if line == "@@" {
			if inHunk && (!removed || !added) {
				return fail()
			}
			inHunk, removed, added = true, false, false
			continue
		}
		if !inHunk || len(line) == 0 {
			return fail()
		}
		switch line[0] {
		case ' ':
		case '-':
			removed = true
		case '+':
			added = true
		default:
			return fail()
		}
	}
	if !inHunk || !removed || !added {
		return fail()
	}
	return target, nil
}

var literalPath = regexp.MustCompile(`^[a-zA-Z0-9_./][a-zA-Z0-9_./-]*$`)
var rangeRead = regexp.MustCompile(`^sed -n '1,220p' ([a-zA-Z0-9_./][a-zA-Z0-9_./-]*)$`)
var patchEnvelope = regexp.MustCompile(`\AExit code: (-?[0-9]+)\nWall time: [0-9]+(?:\.[0-9]+)? seconds\nOutput:\n`)

func acquisitionTarget(cmd string) string {
	if readCommand.MatchString(cmd) {
		return strings.TrimPrefix(cmd, "cat ")
	}
	if m := rangeRead.FindStringSubmatch(cmd); m != nil {
		return m[1]
	}
	return ""
}

func patchOutput(output string) (string, bool, error) {
	m := patchEnvelope.FindStringSubmatch(output)
	if m == nil {
		return "", false, fmt.Errorf("unsupported action result envelope")
	}
	code, err := strconv.Atoi(m[1])
	if err != nil {
		return "", false, fmt.Errorf("invalid action exit code")
	}
	body := output[len(m[0]):]
	// Keep nonzero exit evidence verbatim even when the tool's body is empty.
	if code != 0 {
		body = "Exit code: " + m[1] + "\n" + body
	}
	return body, code != 0, nil
}
