package acceptance_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectStructuredCodexRepeatedFeedback(t *testing.T) {
	binary := buildBinary(t)
	for _, version := range []string{"0.162.1", "0.163.0"} {
		fixture, err := filepath.Abs("../fixtures/repeated-evidence/codex-" + version + ".jsonl")
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(strings.TrimSuffix(fixture, ".jsonl") + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		for _, format := range []string{"auto", "codex"} {
			for _, existing := range []bool{false, true} {
				t.Run(version+"/"+format+map[bool]string{false: "/absent", true: "/existing"}[existing], func(t *testing.T) {
					dir := t.TempDir()
					targets := []string{"CONTEXT-1.md", "PROJECT-1.md", "CONTEXT-2.md", "PROJECT-2.md"}
					if existing {
						for _, target := range targets {
							if err := os.WriteFile(filepath.Join(dir, target), []byte("untouched\n"), 0600); err != nil {
								t.Fatal(err)
							}
						}
					}
					cmd := exec.Command(binary, "inspect", "--format", format, fixture)
					cmd.Dir = dir
					var stdout, stderr bytes.Buffer
					cmd.Stdout = &stdout
					cmd.Stderr = &stderr
					if err := cmd.Run(); err != nil || stderr.Len() != 0 || stdout.String() != string(want) {
						t.Fatalf("%v stderr=%s\ngot:\n%s\nwant:\n%s", err, &stderr, &stdout, want)
					}
					for _, target := range targets {
						got, err := os.ReadFile(filepath.Join(dir, target))
						if existing {
							if err != nil || string(got) != "untouched\n" {
								t.Fatalf("inspection changed %s: %q %v", target, got, err)
							}
						} else if !os.IsNotExist(err) {
							t.Fatalf("inspection created %s: %v", target, err)
						}
					}
				})
			}
		}
	}
}

func TestInspectStructuredCodexTerminationAndInvalidFeedback(t *testing.T) {
	binary := buildBinary(t)
	b, err := os.ReadFile("../fixtures/repeated-evidence/codex-0.163.0.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	for _, tc := range []struct {
		name, input, want string
		invalid           bool
	}{
		{"final alone", strings.Join(lines[:7], "\n"), "Execution state: INCOMPLETE", false},
		{"tools alone", strings.Join(lines[:6], "\n"), "Execution state: INCOMPLETE", false},
		{"late missing feedback", strings.Replace(string(b), `"stdout": "Success. Updated the following files:\nM PROJECT-2.md\n"`, `"stdout": null`, 1), "line 6", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "trace.jsonl")
			if err := os.WriteFile(path, []byte(tc.input), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "inspect", path)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			if tc.invalid {
				if err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.want) {
					t.Fatalf("%v stdout=%s stderr=%s", err, &stdout, &stderr)
				}
				return
			}
			if err != nil || stderr.Len() != 0 || !strings.Contains(stdout.String(), tc.want) || !strings.Contains(stdout.String(), "not verified task success") {
				t.Fatalf("%v stdout=%s stderr=%s", err, &stdout, &stderr)
			}
		})
	}
}
