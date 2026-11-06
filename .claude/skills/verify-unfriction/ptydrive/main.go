//go:build darwin || linux

// ptydrive runs one command under a fresh pseudo-terminal that is the child's
// controlling terminal (so /dev/tty works), plays an ordered list of
// expect/send steps, and writes evidence files.
//
// Usage (interactive, under a PTY):
//
//	ptydrive -out DIR [-dir CWD] [-env K=V]... [-timeout 5s] [-rows 35] [-cols 140]
//	         [steps...] -- COMMAND [ARGS...]
//
// Steps run in order:
//
//	-expect TEXT      wait until TEXT appears anywhere in the transcript so far
//	-expect-new TEXT  wait until TEXT appears in output received after the last send
//	-send TEXT        type TEXT (Go escapes allowed: \r \t \x1b \x15 ...)
//	-key NAME         enter esc tab up down left right pgup pgdn home end ctrl-c ctrl-u
//	-want-exit N      after the steps, require exit status N (default: report only)
//
// Files written to DIR: command.txt, steps.log, transcript.raw, screen.txt
// (ANSI stripped), stdout.txt, exit.txt, tty-restored.txt, pid.txt.
//
// Non-interactive mode: ptydrive -notty -out DIR [-dir CWD] [-env K=V]... -- CMD...
// runs CMD with no controlling terminal and stdin /dev/null, writes command.txt,
// stdout.txt, stderr.txt, exit.txt, pid.txt, and exits with CMD's status.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
)

type step struct{ kind, arg string }

var keys = map[string]string{
	"enter": "\r", "esc": "\x1b", "tab": "\t", "up": "\x1b[A", "down": "\x1b[B",
	"right": "\x1b[C", "left": "\x1b[D", "pgup": "\x1b[5~", "pgdn": "\x1b[6~",
	"home": "\x1b[H", "end": "\x1b[F", "ctrl-c": "\x03", "ctrl-u": "\x15",
}

type capture struct {
	sync.Mutex
	buf bytes.Buffer
}

func (c *capture) Write(p []byte) (int, error) { c.Lock(); defer c.Unlock(); return c.buf.Write(p) }
func (c *capture) snapshot() []byte {
	c.Lock()
	defer c.Unlock()
	return append([]byte(nil), c.buf.Bytes()...)
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ptydrive: "+format+"\n", a...)
	os.Exit(2)
}

func main() {
	var (
		out, dir   string
		env        []string
		steps      []step
		timeout    = 5 * time.Second
		rows, cols = 35, 140
		wantExit   = -1
		notty      bool
		command    []string
	)
	args := os.Args[1:]
	next := func(i *int, flag string) string {
		*i++
		if *i >= len(args) {
			fail("%s needs a value", flag)
		}
		return args[*i]
	}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--":
			command = args[i+1:]
			i = len(args)
		case "-notty":
			notty = true
		case "-out":
			out = next(&i, a)
		case "-dir":
			dir = next(&i, a)
		case "-env":
			env = append(env, next(&i, a))
		case "-timeout":
			d, err := time.ParseDuration(next(&i, a))
			if err != nil {
				fail("bad -timeout: %v", err)
			}
			timeout = d
		case "-rows", "-cols":
			n, err := strconv.Atoi(next(&i, a))
			if err != nil {
				fail("bad %s: %v", a, err)
			}
			if a == "-rows" {
				rows = n
			} else {
				cols = n
			}
		case "-want-exit":
			n, err := strconv.Atoi(next(&i, a))
			if err != nil {
				fail("bad -want-exit: %v", err)
			}
			wantExit = n
		case "-expect", "-expect-new":
			steps = append(steps, step{a[1:], next(&i, a)})
		case "-send":
			v := next(&i, a)
			if u, err := strconv.Unquote(`"` + strings.ReplaceAll(v, `"`, `\"`) + `"`); err == nil {
				v = u
			}
			steps = append(steps, step{"send", v})
		case "-key":
			v := next(&i, a)
			if _, ok := keys[v]; !ok {
				fail("unknown key %q", v)
			}
			steps = append(steps, step{"key", v})
		default:
			fail("unknown argument %q (did you forget -- before the command?)", a)
		}
	}
	if out == "" || len(command) == 0 {
		fail("usage: ptydrive -out DIR [-dir CWD] [-env K=V]... [steps...] -- COMMAND [ARGS...]")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fail("%v", err)
	}
	write := func(name, s string) { _ = os.WriteFile(filepath.Join(out, name), []byte(s), 0o644) }
	write("command.txt", fmt.Sprintf("cwd: %s\nenv overrides: %q\ncommand: %q\n", dir, env, command))
	if notty {
		if len(steps) > 0 {
			fail("-notty takes no steps")
		}
		os.Exit(runDetached(out, dir, env, command, timeout, write))
	}

	terminal, slave, err := pty.Open()
	if err != nil {
		fail("open pty: %v", err)
	}
	defer terminal.Close()
	if err = pty.Setsize(terminal, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}); err != nil {
		fail("size pty: %v", err)
	}
	// Wrap in sh to record terminal settings before and after, proving restoration.
	before, after := filepath.Join(out, ".stty-before"), filepath.Join(out, ".stty-after")
	script := `b=$1; a=$2; shift 2; stty -g > "$b"; "$@"; r=$?; stty -g > "$a"; exit "$r"`
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", append([]string{"-c", script, "ptydrive-host", before, after}, command...)...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "TERM=xterm-256color"), env...)
	var stdout bytes.Buffer
	cmd.Stdin, cmd.Stderr, cmd.Stdout = slave, slave, &stdout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	if err = cmd.Start(); err != nil {
		fail("start: %v", err)
	}
	write("pid.txt", strconv.Itoa(cmd.Process.Pid)+"\n")

	var screen capture
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 4096)
		for {
			n, err := terminal.Read(buf)
			if n > 0 {
				_, _ = screen.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	var waitErr error
	done := make(chan struct{})
	go func() { waitErr = cmd.Wait(); close(done) }()
	// Close the slave after Wait so the master reader sees EOF, matching
	// acceptance/select_test.go startTerminal cleanup order.
	closeSlave := sync.OnceFunc(func() { _ = slave.Close() })
	drainReader := func() {
		closeSlave()
		select {
		case <-readDone:
		case <-time.After(time.Second):
		}
	}

	var log strings.Builder
	start := time.Now()
	logf := func(format string, a ...any) {
		fmt.Fprintf(&log, "[%6.3fs] ", time.Since(start).Seconds())
		fmt.Fprintf(&log, format+"\n", a...)
	}
	mark := 0
	failed := ""
	for _, s := range steps {
		switch s.kind {
		case "send", "key":
			mark = len(screen.snapshot())
			b := s.arg
			if s.kind == "key" {
				b = keys[s.arg]
			}
			if _, err := terminal.Write([]byte(b)); err != nil {
				failed = fmt.Sprintf("%s %q: %v", s.kind, s.arg, err)
			} else {
				logf("%s %q", s.kind, s.arg)
			}
		case "expect", "expect-new":
			deadline := time.Now().Add(timeout)
			for {
				raw := screen.snapshot()
				if s.kind == "expect-new" {
					raw = raw[min(mark, len(raw)):]
				}
				if bytes.Contains(raw, []byte(s.arg)) || strings.Contains(ansi.Strip(string(raw)), s.arg) {
					logf("%s %q: found", s.kind, s.arg)
					break
				}
				if time.Now().After(deadline) {
					failed = fmt.Sprintf("%s %q: not seen within %s", s.kind, s.arg, timeout)
					break
				}
				select {
				case <-done:
					drainReader()
					raw = screen.snapshot()
					if s.kind == "expect-new" {
						raw = raw[min(mark, len(raw)):]
					}
					if bytes.Contains(raw, []byte(s.arg)) || strings.Contains(ansi.Strip(string(raw)), s.arg) {
						logf("%s %q: found", s.kind, s.arg)
					} else {
						failed = fmt.Sprintf("%s %q: process exited before it appeared", s.kind, s.arg)
					}
				case <-time.After(10 * time.Millisecond):
					continue
				}
				break
			}
		}
		if failed != "" {
			logf("FAIL %s", failed)
			break
		}
	}

	select {
	case <-done:
	case <-time.After(timeout):
		if failed == "" {
			failed = fmt.Sprintf("process did not exit within %s after the last step", timeout)
			logf("FAIL %s", failed)
		}
		cancel()
		<-done
	}
	drainReader()
	_ = terminal.Close()

	code := 0
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		code = exitErr.ExitCode()
	} else if waitErr != nil {
		code = -1
	}
	logf("exit %d", code)
	raw := screen.snapshot()
	write("transcript.raw", string(raw))
	write("screen.txt", ansi.Strip(string(raw)))
	write("stdout.txt", stdout.String())
	write("exit.txt", strconv.Itoa(code)+"\n")
	b1, e1 := os.ReadFile(before)
	b2, e2 := os.ReadFile(after)
	restored := "unknown (process killed before host shell recorded state)"
	if e1 == nil && e2 == nil {
		restored = strconv.FormatBool(bytes.Equal(b1, b2))
	}
	detail := ""
	if restored == "false" {
		detail = fmt.Sprintf("before: %safter:  %s", b1, b2)
	}
	write("tty-restored.txt", restored+"\n"+detail)
	_ = os.Remove(before)
	_ = os.Remove(after)
	if failed == "" && wantExit >= 0 && code != wantExit {
		failed = fmt.Sprintf("exit status %d, want %d", code, wantExit)
		logf("FAIL %s", failed)
	}
	write("steps.log", log.String())
	if failed != "" {
		fmt.Fprintf(os.Stderr, "ptydrive: FAIL %s (evidence in %s)\n", failed, out)
		os.Exit(1)
	}
	fmt.Printf("ptydrive: OK exit=%d tty-restored=%s evidence=%s\n", code, restored, out)
}

// runDetached runs the command in a new session with no controlling terminal
// and stdin from /dev/null, the way a script or CI job sees unfriction. It
// returns the command's exit status (or 124 on timeout).
func runDetached(out, dir string, env, command []string, timeout time.Duration, write func(string, string)) int {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	err := cmd.Start()
	if err != nil {
		fail("start: %v", err)
	}
	write("pid.txt", strconv.Itoa(cmd.Process.Pid)+"\n")
	err = cmd.Wait()
	code := 0
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		code = 124
		fmt.Fprintf(&stderr, "\nptydrive: killed after %s timeout\n", timeout)
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	case err != nil:
		code = -1
	}
	write("stdout.txt", stdout.String())
	write("stderr.txt", stderr.String())
	write("exit.txt", strconv.Itoa(code)+"\n")
	return code
}
