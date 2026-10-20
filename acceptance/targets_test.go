package acceptance_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// These are synthetic sequencing/path variants of the supported real sanitized
// action fixtures. No new vendor telemetry or tool grammar is asserted.
func TestInspectLiteralAcquisitionTargets(t *testing.T) {
	binary := buildBinary(t)
	for _, vendor := range []struct {
		name, format, target string
		prefix, suffix       int
	}{
		{"codex", "codex", "GUIDE.md", 2, 2},
		{"claude", "claude-code", "README.md", 1, 1},
	} {
		b, err := os.ReadFile("../fixtures/" + vendor.name + "-action.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		read, observation := lines[vendor.prefix], lines[vendor.prefix+1]
		action, result := lines[vendor.prefix+2], lines[vendor.prefix+3]
		if vendor.name == "codex" {
			action = strings.ReplaceAll(action, "README.md", vendor.target)
			result = strings.ReplaceAll(result, "README.md", vendor.target)
		}
		for _, scenario := range []string{"revisit after action", "literal spelling", "pending revisit", "three actions", "error revisit"} {
			t.Run(vendor.name+"/"+scenario, func(t *testing.T) {
				input := append([]string{}, lines[:vendor.prefix]...)
				input = append(input, read, observation)
				distinct, repeated, attempts := 1, 0, 1
				switch scenario {
				case "revisit after action":
					input = append(input, action, result, strings.ReplaceAll(read, "read-1", "read-2"), strings.ReplaceAll(observation, "read-1", "read-2"), strings.ReplaceAll(strings.ReplaceAll(read, "read-1", "read-3"), vendor.target, "OTHER.md"), strings.ReplaceAll(observation, "read-1", "read-3"))
					distinct, repeated, attempts = 2, 1, 3
				case "literal spelling":
					input = append(input, strings.ReplaceAll(strings.ReplaceAll(read, "read-1", "read-2"), vendor.target, "./"+vendor.target), strings.ReplaceAll(observation, "read-1", "read-2"))
					distinct, attempts = 2, 2
				case "pending revisit":
					input = append(input, strings.ReplaceAll(read, "read-1", "read-2"))
					repeated, attempts = 1, 2
				case "three actions":
					for n := 1; n <= 3; n++ {
						id := fmt.Sprintf("edit-%d", n)
						input = append(input, strings.ReplaceAll(action, "edit-1", id), strings.ReplaceAll(result, "edit-1", id))
					}
				case "error revisit":
					failure := strings.ReplaceAll(observation, "read-1", "read-2")
					if vendor.name == "claude" {
						failure = strings.ReplaceAll(failure, `"is_error": false`, `"is_error": true`)
					} else {
						failure = strings.ReplaceAll(failure, "Process exited with code 0", "Process exited with code 1")
					}
					input = append(input, strings.ReplaceAll(read, "read-1", "read-2"), failure)
					repeated, attempts = 1, 2
				}
				input = append(input, lines[len(lines)-vendor.suffix:]...)
				got := inspectRepeatedInput(t, binary, vendor.format, input)
				for _, want := range []string{fmt.Sprintf("- Attempts: %d\n", attempts), fmt.Sprintf("- Distinct targets: %d\n- Repeated requests: %d\n", distinct, repeated)} {
					if !strings.Contains(got, want) {
						t.Errorf("missing %q:\n%s", want, got)
					}
				}
				if scenario == "pending revisit" && !strings.Contains(got, "Diagnostic: Observation missing for a2") {
					t.Error("missing observation diagnostic lost")
				}
				for n := 1; n <= attempts; n++ {
					if !strings.Contains(got, fmt.Sprintf("Acquisition a%d:", n)) {
						t.Errorf("acquisition a%d was lost", n)
					}
				}
			})
		}
	}
}
