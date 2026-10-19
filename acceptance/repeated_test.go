package acceptance_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectRecordedRepeatedFeedback(t *testing.T) {
	binary := buildBinary(t)
	// This fixture retains four real tool pairs from one Claude Code task.
	fixture, err := filepath.Abs("../fixtures/claude-repeated.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../fixtures/claude-repeated.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"auto", "claude-code"} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/existing=%t", mode, existing), func(t *testing.T) {
				dir := t.TempDir()
				targets := []string{"README.md", "GUIDE.md"}
				if existing {
					for _, name := range targets {
						if err := os.WriteFile(filepath.Join(dir, name), []byte("# Example\n"), 0600); err != nil {
							t.Fatal(err)
						}
					}
				}
				cmd := exec.Command(binary, "inspect", "--format", mode, fixture)
				cmd.Dir = dir
				got, err := cmd.CombinedOutput()
				if err != nil || string(got) != string(want) {
					t.Fatalf("%v\ngot:\n%s\nwant:\n%s", err, got, want)
				}
				for _, name := range targets {
					got, err := os.ReadFile(filepath.Join(dir, name))
					if existing {
						if err != nil || string(got) != "# Example\n" {
							t.Fatalf("inspection changed %s: %q %v", name, got, err)
						}
					} else if !os.IsNotExist(err) {
						t.Fatalf("inspection created %s: %q %v", name, got, err)
					}
				}
			})
		}
	}
}

// These synthetic sequences repeat the sanitized M3 pairs. They test repetition
// beyond two cycles, not additional claims about recorded vendor behavior.
func TestInspectThreeRepeatedSequences(t *testing.T) {
	binary := buildBinary(t)
	for _, vendor := range []string{"codex", "claude"} {
		b, err := os.ReadFile("../fixtures/" + vendor + "-action.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		start, format := 2, "codex"
		if vendor == "claude" {
			start, format = 1, "claude-code"
		}
		var events []string
		for i := 1; i <= 3; i++ {
			for _, line := range lines[start : start+4] {
				events = append(events, strings.NewReplacer("read-1", fmt.Sprintf("read-%d", i), "edit-1", fmt.Sprintf("edit-%d", i), "GUIDE.md", "README.md").Replace(line))
			}
		}
		for _, mode := range []string{"auto", format} {
			t.Run(vendor+"/"+mode, func(t *testing.T) {
				input := append(append(append([]string{}, lines[:start]...), events...), lines[start+4:]...)
				got := inspectRepeatedInput(t, binary, mode, input)
				assertThreeSequences(t, got)
			})
		}
		t.Run(vendor+"/late invalid feedback", func(t *testing.T) {
			input := append(append([]string{}, lines[:start]...), events...)
			input = append(input, events[3]) // Duplicate the first action result after three sequences.
			input = append(input, lines[start+4:]...)
			path := filepath.Join(t.TempDir(), "invalid.jsonl")
			if err := os.WriteFile(path, []byte(strings.Join(input, "\n")), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "inspect", path)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "duplicate") || !strings.Contains(stderr.String(), fmt.Sprintf("line %d", start+13)) {
				t.Fatalf("%v stdout=%s stderr=%s", err, &stdout, &stderr)
			}
		})
		for _, completed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/error/completed=%t", vendor, completed), func(t *testing.T) {
				changed := append([]string{}, events...)
				if vendor == "codex" {
					changed[3] = strings.Replace(changed[3], "Exit code: 0", "Exit code: 1", 1)
				} else {
					changed[3] = strings.Replace(changed[3], `"tool_use_id": "edit-1"`, `"tool_use_id": "edit-1", "is_error": true`, 1)
				}
				input := append(append([]string{}, lines[:start]...), changed...)
				if completed {
					input = append(input, lines[start+4:]...)
				}
				got := inspectRepeatedInput(t, binary, format, input)
				assertThreeSequences(t, got)
				if !strings.Contains(got, "Tool reported an error.\n5. Acquisition a2") {
					t.Fatal(got)
				}
				state := "INCOMPLETE"
				if completed {
					state = "COMPLETED"
				}
				if !strings.Contains(got, "Execution state: "+state) {
					t.Fatal(got)
				}
			})
			t.Run(fmt.Sprintf("%s/missing/completed=%t", vendor, completed), func(t *testing.T) {
				// Drop the second observation and second action result; later feedback
				// must not accidentally satisfy either missing request.
				input := append([]string{}, lines[:start]...)
				for i, line := range events {
					if i != 5 && i != 7 {
						input = append(input, line)
					}
				}
				if completed {
					input = append(input, lines[start+4:]...)
				}
				got := inspectRepeatedInput(t, binary, format, input)
				for _, want := range []string{"Diagnostic: Observation missing for a2", "Diagnostic: Result missing for act2", "Observation for a3:", "Result for act3:"} {
					if !strings.Contains(got, want) {
						t.Fatal(got)
					}
				}
				if strings.Count(got, "Diagnostic:") != 2 {
					t.Fatal(got)
				}
				state := "INCOMPLETE"
				if completed {
					state = "COMPLETED"
				}
				if !strings.Contains(got, "Execution state: "+state) {
					t.Fatal(got)
				}
			})
		}
	}
}

func inspectRepeatedInput(t *testing.T, binary, format string, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "README.md")
	// First inspect with no target, then with an existing target. Both reports
	// must agree: recorded evidence does not depend on the current filesystem.
	absentCmd := exec.Command(binary, "inspect", "--format", format, path)
	absentCmd.Dir = dir
	absentOutput, err := absentCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, absentOutput)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("inspection created target: %v", err)
	}
	if err := os.WriteFile(target, []byte("# Example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "inspect", "--format", format, path)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil || stderr.Len() != 0 {
		t.Fatalf("%v stdout=%s stderr=%s", err, &stdout, &stderr)
	}
	if stdout.String() != string(absentOutput) {
		t.Fatalf("report depends on target existence: absent=%s existing=%s", absentOutput, &stdout)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "# Example\n" {
		t.Fatalf("inspection changed target: %q %v", got, err)
	}
	if !strings.Contains(stdout.String(), "Completion describes execution termination, not verified task success.") {
		t.Fatal(stdout.String())
	}
	if strings.Contains(strings.ToLower(stdout.String()), "retry") {
		t.Fatal("report inferred retry: " + stdout.String())
	}
	return stdout.String()
}

func assertThreeSequences(t *testing.T, got string) {
	t.Helper()
	rest := got
	for i := 1; i <= 3; i++ {
		for _, want := range []string{
			fmt.Sprintf("%d. Acquisition a%d: README.md\n", 4*i-3, i),
			fmt.Sprintf("%d. Observation for a%d:\n", 4*i-2, i),
			fmt.Sprintf("%d. Action act%d: Modify README.md\n", 4*i-1, i),
			fmt.Sprintf("%d. Result for act%d:\n", 4*i, i),
		} {
			_, after, found := strings.Cut(rest, want)
			if !found {
				t.Fatalf("missing/out-of-order %q:\n%s", want, got)
			}
			rest = after
		}
	}
	if strings.Count(got, ". Acquisition ") != 3 || strings.Count(got, ". Action ") != 3 || strings.Count(got, "Task:\n") != 1 {
		t.Fatal(got)
	}
}
