package recordings

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/unfriction-ai/unfriction/internal/claude"
	"github.com/unfriction-ai/unfriction/internal/codex"
	"github.com/unfriction-ai/unfriction/internal/session"
)

type Source struct{ Path, Family string }
type Entry struct {
	Source      Source
	Document    session.Document
	Association Association
	fingerprint [32]byte
	identity    os.FileInfo
}
type Update struct {
	Source  Source
	Pending bool
	Entry   *Entry
	Err     error
}
type Config struct {
	CodexHome, ClaudeHome, Format string
	Workers                       int
}

func DefaultConfig() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	c := Config{CodexHome: os.Getenv("CODEX_HOME"), ClaudeHome: os.Getenv("CLAUDE_CONFIG_DIR"), Format: "auto", Workers: 2}
	if c.CodexHome == "" {
		c.CodexHome = filepath.Join(home, ".codex")
	}
	if c.ClaudeHome == "" {
		c.ClaudeHome = filepath.Join(home, ".claude")
	}
	return c, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func Load(ctx context.Context, s Source) (Entry, error) {
	e := Entry{Source: s}
	before, err := os.Lstat(s.Path)
	if err != nil {
		return e, err
	}
	if !before.Mode().IsRegular() {
		return e, fmt.Errorf("recording is not a regular file")
	}
	f, err := os.Open(s.Path)
	if err != nil {
		return e, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return e, fmt.Errorf("recording changed while opening; refresh")
	}
	hash := sha256.New()
	reader := io.TeeReader(contextReader{ctx, f}, hash)
	switch s.Family {
	case "codex":
		e.Document, err = codex.Summarize(reader)
	case "claude-code":
		e.Document, err = claude.Summarize(reader)
	default:
		return e, fmt.Errorf("unsupported recording family")
	}
	// Summaries keep explanatory text, never detailed tool payloads.
	e.Document.Records = nil
	if err != nil {
		return e, fmt.Errorf("unsupported or incomplete recording: %w", err)
	}
	after, err := os.Lstat(s.Path)
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return e, fmt.Errorf("recording changed during discovery; refresh")
	}
	copy(e.fingerprint[:], hash.Sum(nil))
	e.identity = before
	return e, nil
}
func (e Entry) Revalidate(ctx context.Context) error {
	current, err := Load(ctx, e.Source)
	if err != nil {
		return fmt.Errorf("recording unavailable or changed; refresh and reselect: %w", err)
	}
	if !os.SameFile(e.identity, current.identity) || e.fingerprint != current.fingerprint {
		return fmt.Errorf("recording changed; refresh and reselect")
	}
	return nil
}

// Discover streams pending sources and completed summaries with bounded workers.
// The consumer cancels the context when it closes the picker. No cache is used.
func Discover(ctx context.Context, repo Repository, c Config) <-chan Update {
	return discover(ctx, repo, c, Load)
}
func discover(ctx context.Context, repo Repository, c Config, load func(context.Context, Source) (Entry, error)) <-chan Update {
	out := make(chan Update)
	jobs := make(chan Source)
	emit := func(u Update) bool {
		select {
		case out <- u:
			return true
		case <-ctx.Done():
			return false
		}
	}
	workers := c.Workers
	if workers < 1 {
		workers = 2
	}
	if workers > 4 {
		workers = 4
	}
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range jobs {
				if ctx.Err() != nil {
					return
				}
				e, err := load(ctx, s)
				if err != nil {
					if !emit(Update{Source: s, Err: err}) {
						return
					}
					continue
				}
				association, included := repo.Associate(ctx, e.Document)
				e.Association = association
				u := Update{Source: s}
				if included {
					u.Entry = &e
				}
				if !emit(u) {
					return
				}
			}
		}()
	}
	go func() {
		defer func() { close(jobs); wg.Wait(); close(out) }()
		for _, root := range []struct{ home, folder, family string }{{c.CodexHome, "sessions", "codex"}, {c.ClaudeHome, "projects", "claude-code"}} {
			if c.Format != "auto" && c.Format != root.family {
				continue
			}
			base, err := filepath.Abs(filepath.Join(root.home, root.folder))
			if err != nil {
				emit(Update{Err: err})
				continue
			}
			base = canonical(base)
			err = filepath.WalkDir(base, func(path string, d fs.DirEntry, walkErr error) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if walkErr != nil {
					if path == base && os.IsNotExist(walkErr) {
						return nil
					}
					emit(Update{Source: Source{path, root.family}, Err: walkErr})
					return nil
				}
				rel, _ := filepath.Rel(base, path)
				parts := strings.Split(rel, string(filepath.Separator))
				// Claude subagent histories are separate branches, not top-level sessions.
				if d.IsDir() {
					if root.family == "claude-code" && len(parts) >= 2 {
						return filepath.SkipDir
					}
					return nil
				}
				if !strings.HasSuffix(d.Name(), ".jsonl") {
					return nil
				}
				if root.family == "claude-code" && len(parts) != 2 || root.family == "codex" && len(parts) != 4 {
					emit(Update{Source: Source{path, root.family}, Err: fmt.Errorf("unsupported recording layout; use inspect with a known path")})
					return nil
				}
				s := Source{path, root.family}
				if !emit(Update{Source: s, Pending: true}) {
					return ctx.Err()
				}
				select {
				case jobs <- s:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			if err != nil && ctx.Err() == nil {
				emit(Update{Source: Source{base, root.family}, Err: err})
			}
		}
	}()
	return out
}
