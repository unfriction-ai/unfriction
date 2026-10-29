//go:build darwin || linux

package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestInspectWithoutPath(t *testing.T) {
	binary := buildBinary(t)
	repo, _ := fixtureRepository(t)
	codexHome, claudeHome := t.TempDir(), t.TempDir()
	paths := map[string]string{"codex": installSession(t, codexHome, "codex", repo, "current"), "claude-code": installSession(t, claudeHome, "claude-code", repo, "current")}
	for family, path := range paths {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		paths[family] = resolved
	}
	for _, family := range []string{"codex", "claude-code"} {
		t.Run(family, func(t *testing.T) {
			p := startTerminal(t, binary, repo, codexHome, claudeHome, "inspect", "--format", family)
			p.waitFor(t, "Discovery complete")
			p.waitFor(t, "Enter inspect")
			p.send(t, "validation")
			p.waitFor(t, "Search: validation")
			p.send(t, "\x1b[B")
			p.waitFor(t, "User: Go")
			p.send(t, "\r")
			report := p.finish(t, 0)
			expected, err := exec.Command(binary, "inspect", "--format", family, "--turn", family+":second", "--", paths[family]).CombinedOutput()
			if err != nil {
				t.Fatalf("direct inspect: %v\n%s", err, expected)
			}
			if report != string(expected) {
				t.Fatalf("picker inspection differs from direct inspection\ngot: %s\nwant: %s", report, expected)
			}
			if strings.Count(report, "Execution state:") != 1 || !strings.Contains(report, "Task:\nGo\n") {
				t.Fatal("selection did not inspect exactly once")
			}
			other := "codex"
			if family == "codex" {
				other = "claude-code"
			}
			if strings.Contains(p.screen.text(), paths[other]) {
				t.Fatal("format filter leaked another family")
			}
		})
	}
	t.Run("automatic family", func(t *testing.T) {
		p := startTerminal(t, binary, repo, codexHome, claudeHome, "inspect")
		p.waitFor(t, "Discovery complete")
		p.send(t, "codex")
		p.waitFor(t, "Search: codex")
		p.send(t, "\x1b[B")
		p.waitFor(t, "User: Go")
		p.send(t, "\r")
		out := p.finish(t, 0)
		if !strings.Contains(out, "Session: session-codex") || !strings.Contains(out, "Task:\nGo\n") {
			t.Fatal(out)
		}
	})
}

func TestInspectPickerOutcomes(t *testing.T) {
	binary := buildBinary(t)
	repo, _ := fixtureRepository(t)
	codexHome, claudeHome := t.TempDir(), t.TempDir()
	t.Run("empty", func(t *testing.T) {
		p := startTerminal(t, binary, repo, codexHome, claudeHome, "inspect")
		p.waitFor(t, "no supported recordings")
		if out := p.finish(t, 1); out != "" {
			t.Fatal(out)
		}
	})
	installSession(t, codexHome, "codex", repo, "current")
	for _, keys := range []string{"\x1b", "\x03"} {
		t.Run("cancel="+keys, func(t *testing.T) {
			p := startTerminal(t, binary, repo, codexHome, claudeHome, "inspect")
			p.waitFor(t, "Discovery complete")
			p.send(t, keys)
			if out := p.finish(t, 0); out != "" {
				t.Fatalf("cancel emitted report: %s", out)
			}
		})
	}
	for _, tc := range []struct {
		name    string
		args    []string
		code    int
		message string
	}{
		{"no terminal", []string{"inspect"}, 2, "trace path"},
		{"selector requires path", []string{"inspect", "--turn", "codex:second"}, 1, "--turn requires a trace path"},
		{"invalid format", []string{"inspect", "--format", "unknown"}, 1, "unsupported format"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binary, tc.args...)
			cmd.Dir = repo
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			out, err := cmd.CombinedOutput()
			e, ok := err.(*exec.ExitError)
			if !ok || e.ExitCode() != tc.code || !strings.Contains(string(out), tc.message) {
				t.Fatalf("%v: %s", err, out)
			}
		})
	}
	t.Run("explicit path never discovers", func(t *testing.T) {
		fixture, err := filepath.Abs(filepath.Join("..", "fixtures", "first-run.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "inspect", fixture)
		cmd.Dir = t.TempDir()
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		cmd.Env = append(os.Environ(), "CODEX_HOME=/nonexistent", "CLAUDE_CONFIG_DIR=/nonexistent")
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "Execution state: COMPLETED") || strings.Contains(string(out), "Select recorded work") {
			t.Fatalf("%v: %s", err, out)
		}
	})
}
