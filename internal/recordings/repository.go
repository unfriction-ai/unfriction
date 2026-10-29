// Package recordings discovers local sources and summarizes them using vendor
// boundary rules. It never normalizes measured tool activity.
package recordings

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/session"
)

type Worktree struct{ Path, Branch string }
type Repository struct {
	Current, Common string
	Worktrees       []Worktree
}
type Association struct {
	Worktree, RecordedBranch, CurrentBranch string
	Current, Uncertain                      bool
	Evidence                                string
}

func git(ctx context.Context, cwd string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", cwd}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.Output()
	return strings.TrimSuffix(string(out), "\n"), err
}
func canonical(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	path = filepath.Clean(path)
	parent := filepath.Dir(path)
	if parent != path {
		return filepath.Join(canonical(parent), filepath.Base(path))
	}
	return path
}
func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func ResolveRepository(ctx context.Context, cwd string) (Repository, error) {
	root, err := git(ctx, cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repository{}, fmt.Errorf("run selection inside a Git worktree, or use inspect with a known path")
	}
	common, err := git(ctx, cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return Repository{}, fmt.Errorf("resolve Git common directory: %w", err)
	}
	inventory, err := git(ctx, cwd, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return Repository{}, fmt.Errorf("list Git worktrees: %w", err)
	}
	repo := Repository{Current: canonical(root), Common: canonical(common)}
	var w Worktree
	for _, field := range strings.Split(inventory, "\x00") {
		if field == "" {
			if w.Path != "" {
				repo.Worktrees = append(repo.Worktrees, w)
				w = Worktree{}
			}
			continue
		}
		if path, ok := strings.CutPrefix(field, "worktree "); ok {
			w.Path = canonical(path)
		}
		if branch, ok := strings.CutPrefix(field, "branch refs/heads/"); ok {
			w.Branch = branch
		}
	}
	return repo, nil
}

func (r Repository) Associate(ctx context.Context, d session.Document) (Association, bool) {
	a := Association{RecordedBranch: d.Branch}
	if d.CWD == "" || !filepath.IsAbs(d.CWD) {
		a.Uncertain = true
		a.Evidence = "recorded working directory unavailable or relative"
		return a, true
	}
	cwd := canonical(d.CWD)
	var match *Worktree
	for i := range r.Worktrees {
		w := &r.Worktrees[i]
		if within(cwd, w.Path) && (match == nil || len(w.Path) > len(match.Path)) {
			match = w
		}
	}
	if match != nil {
		if r.Common != "" {
			existing := cwd
			for {
				_, statErr := os.Stat(existing)
				if statErr == nil {
					break
				}
				if !os.IsNotExist(statErr) || existing == match.Path {
					a.Uncertain = true
					a.Evidence = "registered worktree path unavailable; repository not reverified"
					return a, true
				}
				existing = filepath.Dir(existing)
			}
			common, err := git(ctx, existing, "rev-parse", "--path-format=absolute", "--git-common-dir")
			if err != nil {
				a.Uncertain = true
				a.Evidence = "recorded cwd Git identity unavailable"
				return a, true
			}
			if canonical(common) != r.Common {
				return a, false
			}
		}
		a.Worktree = match.Path
		a.CurrentBranch = match.Branch
		a.Current = match.Path == r.Current
		a.Evidence = "recorded cwd belongs to a known worktree"
		return a, true
	}
	if _, err := os.Stat(cwd); os.IsNotExist(err) {
		a.Uncertain = true
		a.Evidence = "recorded cwd no longer exists; repository unverified"
		return a, true
	} else if err != nil {
		a.Uncertain = true
		a.Evidence = "recorded cwd unreadable; repository unverified"
		return a, true
	}
	common, err := git(ctx, cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err == nil && r.Common != "" && canonical(common) == r.Common {
		a.Uncertain = true
		a.Evidence = "matching Git common directory, worktree absent from inventory"
		return a, true
	}
	return a, false
}
