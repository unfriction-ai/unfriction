package codex

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// patchTarget recognizes one recorded file creation or update. It never opens
// a target or applies changes. Each update hunk must contain an actual change.
func patchTarget(input string) (target string, create bool, err error) {
	lines := strings.Split(strings.TrimSuffix(input, "\n"), "\n")
	fail := func() (string, bool, error) {
		return "", false, fmt.Errorf("unsupported patch; expected one Add File or Update File with changed hunks")
	}
	if len(lines) < 4 || lines[0] != "*** Begin Patch" || lines[len(lines)-1] != "*** End Patch" {
		return fail()
	}
	switch {
	case strings.HasPrefix(lines[1], "*** Add File: "):
		create = true
		target = strings.TrimPrefix(lines[1], "*** Add File: ")
	case strings.HasPrefix(lines[1], "*** Update File: "):
		target = strings.TrimPrefix(lines[1], "*** Update File: ")
	default:
		return fail()
	}
	if !literalPath.MatchString(target) {
		return fail()
	}
	body := lines[2 : len(lines)-1]
	if create {
		for _, line := range body {
			if !strings.HasPrefix(line, "+") {
				return fail()
			}
		}
		return target, true, nil
	}
	inHunk, changed := false, false
	for _, line := range body {
		if line == "@@" {
			if inHunk && !changed {
				return fail()
			}
			inHunk, changed = true, false
			continue
		}
		if !inHunk || len(line) == 0 {
			return fail()
		}
		switch line[0] {
		case ' ':
		case '-', '+':
			changed = true
		default:
			return fail()
		}
	}
	if !inHunk || !changed {
		return fail()
	}
	return target, false, nil
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
	// This exact standalone refusal was observed without an execution envelope.
	if output == "patch rejected by user" {
		return output, false, nil
	}
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
