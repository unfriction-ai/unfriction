//go:build darwin || linux

package acceptance_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

type terminalCapture struct {
	sync.Mutex
	bytes.Buffer
}

func (b *terminalCapture) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.Write(p)
}
func (b *terminalCapture) text() string { b.Lock(); defer b.Unlock(); return b.Buffer.String() }

func fixtureRepository(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, other := filepath.Join(root, "repo"), filepath.Join(root, "other worktree")
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	git("init", "-b", "main", repo)
	git("-C", repo, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "fixture")
	git("-C", repo, "worktree", "add", "-b", "other", other)
	return repo, other
}

func installSession(t *testing.T, home, family, cwd, name string) string {
	t.Helper()
	fixture, relative := "codex.jsonl", filepath.Join("sessions", "2026", "10", "29", name+".jsonl")
	if family == "claude-code" {
		fixture = "claude.jsonl"
		relative = filepath.Join("projects", "encoded-name-is-not-identity", name+".jsonl")
	}
	b, err := os.ReadFile(filepath.Join("..", "fixtures", "sessions", fixture))
	if err != nil {
		t.Fatal(err)
	}
	// Temp paths have no JSON metacharacters; use a JSON-quoted path for portability.
	quoted := fmt.Sprintf("%q", cwd)
	data := strings.ReplaceAll(string(b), `"/workspace/example"`, quoted)
	path := filepath.Join(home, relative)
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

type pickerProcess struct {
	cmd             *exec.Cmd
	terminal, slave *os.File
	before, after   string
	stdout          bytes.Buffer
	screen          terminalCapture
	done, readDone  chan struct{}
	err             error
}

func startPicker(t *testing.T, binary, repo, codexHome, claudeHome string, args ...string) *pickerProcess {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	p := &pickerProcess{done: make(chan struct{}), readDone: make(chan struct{})}
	var err error
	p.terminal, p.slave, err = pty.Open()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err = pty.Setsize(p.terminal, &pty.Winsize{Rows: 35, Cols: 140}); err != nil {
		t.Fatal(err)
	}
	stateDir := t.TempDir()
	p.before = filepath.Join(stateDir, "before")
	p.after = filepath.Join(stateDir, "after")
	script := `before=$1; after=$2; shift 2; stty -g > "$before"; "$@"; result=$?; stty -g > "$after"; exit "$result"`
	commandArgs := []string{"-c", script, "picker-host", p.before, p.after, binary, "select"}
	p.cmd = exec.CommandContext(ctx, "sh", append(commandArgs, args...)...)
	p.cmd.Dir = repo
	p.cmd.Env = append(os.Environ(), "TERM=xterm-256color", "CODEX_HOME="+codexHome, "CLAUDE_CONFIG_DIR="+claudeHome)
	p.cmd.Stdin = p.slave
	p.cmd.Stderr = p.slave
	p.cmd.Stdout = &p.stdout
	p.cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err = p.cmd.Start(); err != nil {
		cancel()
		p.slave.Close()
		p.terminal.Close()
		t.Fatal(err)
	}
	go func() {
		defer close(p.readDone)
		buf := make([]byte, 4096)
		for {
			n, err := p.terminal.Read(buf)
			if n > 0 {
				_, _ = p.screen.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { p.err = p.cmd.Wait(); close(p.done) }()
	t.Cleanup(func() { cancel(); <-p.done; p.slave.Close(); p.terminal.Close(); <-p.readDone })
	return p
}
func (p *pickerProcess) waitFor(t *testing.T, want string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if strings.Contains(p.screen.text(), want) {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("missing %q:\n%s", want, p.screen.text())
		case <-ticker.C:
		}
	}
}
func (p *pickerProcess) send(t *testing.T, keys string) {
	t.Helper()
	if _, err := p.terminal.Write([]byte(keys)); err != nil {
		t.Fatal(err)
	}
}
func (p *pickerProcess) finish(t *testing.T, wantCode int) string {
	t.Helper()
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("picker did not exit")
	}
	code := 0
	if p.err != nil {
		if e, ok := p.err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			t.Fatal(p.err)
		}
	}
	if code != wantCode {
		t.Fatalf("exit=%d want=%d: %v\n%s", code, wantCode, p.err, p.screen.text())
	}
	before, err := os.ReadFile(p.before)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(p.after)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("picker did not restore terminal state: %q => %q", before, after)
	}
	return p.stdout.String()
}

func TestSelectRecordedWork(t *testing.T) {
	binary := buildBinary(t)
	repo, other := fixtureRepository(t)
	codexHome, claudeHome := t.TempDir(), t.TempDir()
	for _, family := range []string{"codex", "claude-code"} {
		home := codexHome
		if family == "claude-code" {
			home = claudeHome
		}
		installSession(t, home, family, repo, "current")
		installSession(t, home, family, other, "other")
	}
	for _, family := range []string{"codex", "claude-code"} {
		t.Run(family, func(t *testing.T) {
			p := startPicker(t, binary, repo, codexHome, claudeHome, "--format", family)
			p.waitFor(t, "Discovery complete")
			p.send(t, "validation")
			p.waitFor(t, "Search: validation")
			p.send(t, "\x1b[B")
			p.waitFor(t, "User: Go")
			p.send(t, "\r")
			command := p.finish(t, 0)
			if !strings.Contains(command, "--turn '"+family+":second'") || strings.Contains(command, "Execution state:") {
				t.Fatalf("not a selected command: %q", command)
			}
			if !strings.Contains(p.screen.text(), "Previous agent:") {
				t.Fatal("missing explanatory preview")
			}
			if !strings.Contains(command, "current.jsonl") {
				t.Fatal("current worktree was not prioritized")
			}
			run := exec.Command("sh", "-c", command)
			run.Dir = repo
			run.Env = append(os.Environ(), "PATH="+filepath.Dir(binary)+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := run.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "Task:\nGo\n") || !strings.Contains(string(out), "SELECTED_PAYLOAD") || strings.Contains(string(out), "EARLIER_PAYLOAD") {
				t.Fatalf("emitted command: %v\n%s", err, out)
			}
		})
	}
}

func TestSelectTerminalOutcomes(t *testing.T) {
	binary := buildBinary(t)
	repo, _ := fixtureRepository(t)
	codexHome, claudeHome := t.TempDir(), t.TempDir()
	t.Run("empty", func(t *testing.T) {
		p := startPicker(t, binary, repo, codexHome, claudeHome)
		p.waitFor(t, "no supported recordings")
		if out := p.finish(t, 1); out != "" {
			t.Fatalf("stdout=%q", out)
		}
	})
	path := installSession(t, codexHome, "codex", repo, "current")
	t.Run("cancel", func(t *testing.T) {
		p := startPicker(t, binary, repo, codexHome, claudeHome)
		p.waitFor(t, "Discovery complete")
		p.send(t, "\x1b")
		if out := p.finish(t, 130); out != "" {
			t.Fatalf("stdout=%q", out)
		}
	})
	t.Run("changed source", func(t *testing.T) {
		p := startPicker(t, binary, repo, codexHome, claudeHome)
		p.waitFor(t, "Discovery complete")
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.WriteString("\n"); err != nil {
			t.Fatal(err)
		}
		f.Close()
		p.send(t, "\r")
		p.waitFor(t, "refresh and reselect")
		p.send(t, "\x1b")
		if out := p.finish(t, 130); out != "" {
			t.Fatalf("stdout=%q", out)
		}
	})
	t.Run("no terminal", func(t *testing.T) {
		cmd := exec.Command(binary, "select")
		cmd.Dir = repo
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		out, err := cmd.CombinedOutput()
		e, ok := err.(*exec.ExitError)
		if !ok || e.ExitCode() != 2 || !strings.Contains(string(out), "requires a controlling terminal") {
			t.Fatalf("%v: %s", err, out)
		}
	})
}
