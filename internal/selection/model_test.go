package selection

import (
	"context"
	"errors"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unfriction-ai/unfriction/internal/recordings"
	"github.com/unfriction-ai/unfriction/internal/session"
)

func entry(path, worktree, timestamp string, current bool, turns ...session.Turn) recordings.Entry {
	return recordings.Entry{Source: recordings.Source{Path: path, Family: "codex"}, Association: recordings.Association{Worktree: worktree, Current: current}, Document: session.Document{ID: path, Timestamp: timestamp, Turns: turns}}
}
func TestRowsPreserveGroupAndTurnChronology(t *testing.T) {
	entries := []recordings.Entry{
		entry("other", "other", "2026-10-29T14:00:00Z", false, session.Turn{ID: "x"}),
		entry("old", "here", "2026-10-28T12:00:00Z", true, session.Turn{ID: "z"}, session.Turn{ID: "a"}),
		entry("missing", "here", "", true, session.Turn{ID: "x"}),
		entry("new", "here", "2026-10-29T12:00:00Z", true, session.Turn{ID: "x"}),
	}
	rows := Rows(entries, "")
	var got []string
	for _, r := range rows {
		got = append(got, r.Entry.Source.Path+":"+r.Turn.ID)
	}
	if strings.Join(got, ",") != "new:x,old:z,old:a,missing:x,other:x" {
		t.Fatal(got)
	}
}
func TestContextSearchFollowsRecordedAncestry(t *testing.T) {
	e := entry("source", "here", "", true,
		session.Turn{ID: "one", Prompt: "Implement validation"},
		session.Turn{ID: "unrelated", Prompt: "Secret sibling"},
		session.Turn{ID: "two", Prompt: "Continue", PreviousTurnID: "one"},
		session.Turn{ID: "three", Prompt: "Go", PreviousTurnID: "two", PreviousAgent: "Ready"})
	rows := Rows([]recordings.Entry{e}, "validation")
	if len(rows) != 3 || rows[2].Turn.ID != "three" || rows[2].MatchingContext != "Implement validation" {
		t.Fatalf("%+v", rows)
	}
	rows = Rows([]recordings.Entry{e}, "secret")
	if len(rows) != 1 || rows[0].Turn.ID != "unrelated" {
		t.Fatal("search crossed ancestry")
	}
}
func TestCommandsAreLiteralAndControlsAreEscaped(t *testing.T) {
	path := "/tmp/space ' dollar $HOME `touch NO`\n-leading.jsonl"
	e := entry(path, "here", "", true, session.Turn{ID: "turn'quoted"})
	r := Rows([]recordings.Entry{e}, "")[0]
	command, err := Command(r, "zsh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(command, " -- '") || !strings.Contains(command, `'"'"'`) {
		t.Fatal(command)
	}
	if _, err = Command(r, "fish"); err == nil {
		t.Fatal("advertised undelivered shell")
	}
	safe := Safe("prompt\x1b[2J\n\r\u202eend")
	if strings.ContainsAny(safe, "\x1b\n\r\u202e") {
		t.Fatalf("unsafe rendering %q", safe)
	}
	if r.Turn.ID != "turn'quoted" {
		t.Fatal("mutated source identity")
	}
}

func TestCommandRoundTripsShellArguments(t *testing.T) {
	for _, shell := range []string{"sh", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			shellPath, err := exec.LookPath(shell)
			if err != nil {
				t.Skip("shell unavailable")
			}
			root := t.TempDir()
			stub := filepath.Join(root, "unfriction")
			if err = os.WriteFile(stub, []byte("#!/bin/sh\nprintf '%s\\000' \"$@\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(root, "executed")
			path := "/tmp/- path ' \" $HOME `touch " + marker + "` $(touch " + marker + ")\nrecord.jsonl"
			e := entry(path, "here", "", true, session.Turn{ID: "turn'\n$USER"})
			row := Rows([]recordings.Entry{e}, "")[0]
			output := "posix"
			if shell == "zsh" {
				output = "zsh"
			}
			command, err := Command(row, output)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(shellPath, "-c", command)
			cmd.Env = append(os.Environ(), "PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			want := strings.Join([]string{"inspect", "--format", "codex", "--turn", "codex:" + row.Turn.ID, "--", path, ""}, "\x00")
			if string(out) != want {
				t.Fatalf("argv changed: %q want %q", out, want)
			}
			if _, err = os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("selection text executed")
			}
		})
	}
}

func TestProgressAndNeighborsRemainAvailable(t *testing.T) {
	e := entry("source", "here", "", true, session.Turn{ID: "first", Prompt: "validation"}, session.Turn{ID: "second", Prompt: "Go", PreviousTurnID: "first"}, session.Turn{ID: "third", Prompt: "Done", PreviousTurnID: "second"})
	m := picker{ctx: context.Background(), width: 120, height: 30}
	next, _ := m.Update(discoveryMsg{update: recordings.Update{Source: e.Source, Pending: true}})
	m = next.(picker)
	next, _ = m.Update(discoveryMsg{update: recordings.Update{Source: recordings.Source{Path: "bad-root"}, Err: errors.New("unreadable")}})
	m = next.(picker)
	if !strings.Contains(m.View(), "1 pending") || !strings.Contains(m.View(), "Pending summary: codex: source") {
		t.Fatal(m.View())
	}
	next, _ = m.Update(discoveryMsg{update: recordings.Update{Source: e.Source, Entry: &e}})
	m = next.(picker)
	m.query = "Go"
	m.rebuild(false)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(picker)
	if m.rows[m.cursor].Turn.ID != "first" || m.query != "" {
		t.Fatal("could not navigate to earlier chronological turn")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(picker)
	if !m.cancelled || m.accepted != nil {
		t.Fatal("cancel selected a turn")
	}
}

func BenchmarkRowsLongSession(b *testing.B) {
	turns := make([]session.Turn, 5000)
	for i := range turns {
		turns[i] = session.Turn{ID: fmt.Sprint(i), Prompt: "Continue"}
		if i > 0 {
			turns[i].PreviousTurnID = turns[i-1].ID
		}
	}
	turns[0].Prompt = "Implement validation"
	entries := []recordings.Entry{entry("source", "here", "", true, turns...)}
	for _, query := range []string{"", "validation"} {
		b.Run("query="+query, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Rows(entries, query)
			}
		})
	}
}

func TestPreviewShowsDistantMatchAndFullDetails(t *testing.T) {
	prompt := strings.Repeat("setup ", 40) + "validation target at the end"
	e := entry("source", "here", "", true, session.Turn{ID: "first", Prompt: prompt}, session.Turn{ID: "second", Prompt: "Go", PreviousTurnID: "first", EarlierUser: prompt})
	m := picker{width: 80, height: 24, entries: []recordings.Entry{e}, query: "validation"}
	m.rebuild(false)
	m.cursor = 1
	if !strings.Contains(m.View(), "validation target") {
		t.Fatal("matching context is invisible")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(picker)
	if !strings.Contains(m.View(), "Full turn context") {
		t.Fatal("no detail view")
	}
	var seen strings.Builder
	for range 20 {
		seen.WriteString(m.View())
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = next.(picker)
	}
	if !strings.Contains(seen.String(), "validation target") {
		t.Fatal("full context cannot be reached")
	}
}
