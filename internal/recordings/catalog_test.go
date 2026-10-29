package recordings

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unfriction-ai/unfriction/internal/session"
)

func TestAssociationUsesPathsNotBranchNames(t *testing.T) {
	root := t.TempDir()
	current, other, unrelated := filepath.Join(root, "repo"), filepath.Join(root, "other"), filepath.Join(root, "repo-copy")
	for _, p := range []string{current, other, unrelated} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	repo := Repository{Current: canonical(current), Worktrees: []Worktree{{Path: canonical(current), Branch: "now"}, {Path: canonical(other), Branch: "other-now"}}}
	for _, tc := range []struct {
		name, cwd, branch            string
		included, current, uncertain bool
	}{
		{"current subdirectory", filepath.Join(current, "src"), "recorded", true, true, false},
		{"other worktree", other, "recorded", true, false, false},
		{"unknown branch", current, "", true, true, false},
		{"similar path and branch", unrelated, "now", false, false, false},
		{"missing cwd", "", "", true, false, true},
		{"deleted cwd", filepath.Join(root, "deleted"), "old", true, false, true},
		{"relative cwd", "repo", "now", true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, ok := repo.Associate(context.Background(), session.Document{CWD: tc.cwd, Branch: tc.branch})
			if ok != tc.included || a.Current != tc.current || a.Uncertain != tc.uncertain || ok && a.RecordedBranch != tc.branch {
				t.Fatalf("association=%+v included=%v", a, ok)
			}
		})
	}
}

func TestDiscoveryKeepsSupportedSourcesAndDisclosesFailures(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "codex")
	cwd := filepath.Join(root, "repo")
	if err := os.Mkdir(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	repo := Repository{Current: canonical(cwd), Worktrees: []Worktree{{Path: canonical(cwd)}}}
	dir := filepath.Join(home, "sessions", "2026", "10", "29")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("../../fixtures/sessions/codex.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	b = []byte(strings.ReplaceAll(string(b), "/workspace/example", cwd))
	for name, data := range map[string][]byte{"valid.jsonl": b, "partial.jsonl": append(append([]byte{}, b...), '{')} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var pending, loaded, failed int
	for u := range Discover(context.Background(), repo, Config{CodexHome: home, ClaudeHome: filepath.Join(root, "missing"), Format: "auto", Workers: 2}) {
		if u.Pending {
			pending++
		}
		if u.Entry != nil {
			loaded++
			if len(u.Entry.Document.Records) != 0 {
				t.Fatal("catalog retained tool payloads")
			}
			if len(u.Entry.Document.Turns) != 2 {
				t.Fatal("lost turns")
			}
		}
		if u.Err != nil {
			failed++
		}
	}
	if pending != 2 || loaded != 1 || failed != 1 {
		t.Fatalf("pending=%d loaded=%d failed=%d", pending, loaded, failed)
	}
}

func TestSelectionRevalidationRejectsEqualSizeReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recording.jsonl")
	b, err := os.ReadFile("../../fixtures/sessions/codex.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	entry, err := Load(context.Background(), Source{Path: path, Family: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if err = entry.Revalidate(context.Background()); err != nil {
		t.Fatal(err)
	}
	replacement := path + ".new"
	if err = os.WriteFile(replacement, b, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if err = entry.Revalidate(context.Background()); err == nil {
		t.Fatal("replacement accepted")
	}
}

func TestDiscoveryCancellationBoundsWorkers(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sessions", "2026", "10", "29")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for i := range 10 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%02d.jsonl", i)), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{}, 10)
	var active, maxActive atomic.Int32
	load := func(ctx context.Context, s Source) (Entry, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := maxActive.Load(); n > old; old = maxActive.Load() {
			if maxActive.CompareAndSwap(old, n) {
				break
			}
		}
		entered <- struct{}{}
		<-ctx.Done()
		return Entry{}, ctx.Err()
	}
	updates := discover(ctx, Repository{}, Config{CodexHome: root, Format: "codex", Workers: 2}, load)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for range updates {
		}
	}()
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("workers never started")
		}
	}
	cancel()
	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("discovery did not cancel")
	}
	if maxActive.Load() != 2 || active.Load() != 0 {
		t.Fatalf("max=%d active=%d", maxActive.Load(), active.Load())
	}
}

func TestCurrentSessionsSummarizeWithoutNormalizingUnsupportedTools(t *testing.T) {
	for _, tc := range []struct{ family, file string }{{"codex", "codex-current.jsonl"}, {"claude-code", "claude-current.jsonl"}} {
		t.Run(tc.family, func(t *testing.T) {
			e, err := Load(context.Background(), Source{Path: filepath.Join("..", "..", "fixtures", "sessions", tc.file), Family: tc.family})
			if err != nil {
				t.Fatal(err)
			}
			if len(e.Document.Turns) < 2 || len(e.Document.Records) != 0 {
				t.Fatal("summaries missing or tool payload retained")
			}
		})
	}
}

func TestUnsupportedLayoutIsDisclosed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sessions")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "misplaced.jsonl"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	failures := 0
	for u := range Discover(context.Background(), Repository{}, Config{CodexHome: root, Format: "codex"}) {
		if u.Err != nil {
			failures++
		}
	}
	if failures != 1 {
		t.Fatal("unsupported layout silently ignored")
	}
}

func TestNestedIndependentRepositoryIsExcluded(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	for _, path := range []string{root, nested} {
		cmd := exec.Command("git", "init", "-b", "main", path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	repo, err := ResolveRepository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if association, ok := repo.Associate(context.Background(), session.Document{CWD: nested, Branch: "main"}); ok {
		t.Fatalf("nested repository attributed to parent: %+v", association)
	}
}
