// Package selection owns contextual search, ordering, and the shared picker.
package selection

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/unfriction-ai/unfriction/internal/recordings"
	"github.com/unfriction-ai/unfriction/internal/session"
)

type Row struct {
	Entry           recordings.Entry
	Turn            session.Turn
	Position        int
	MatchingContext string
}

func (r Row) Key() string {
	return r.Entry.Source.Family + "\x00" + r.Entry.Source.Path + "\x00" + r.Turn.ID
}
func start(d session.Document) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, d.Timestamp)
	if t.IsZero() && len(d.Turns) > 0 {
		t, _ = time.Parse(time.RFC3339Nano, d.Turns[0].Timestamp)
	}
	return t
}
func Rows(entries []recordings.Entry, query string) []Row {
	sorted := append([]recordings.Entry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Association.Uncertain != b.Association.Uncertain {
			return !a.Association.Uncertain
		}
		if a.Association.Current != b.Association.Current {
			return a.Association.Current
		}
		if a.Association.Worktree != b.Association.Worktree {
			return a.Association.Worktree < b.Association.Worktree
		}
		if a.Document.Branch != b.Document.Branch {
			return a.Document.Branch < b.Document.Branch
		}
		at, bt := start(a.Document), start(b.Document)
		if !at.Equal(bt) {
			return at.After(bt)
		}
		if a.Document.ID != b.Document.ID {
			return a.Document.ID < b.Document.ID
		}
		if a.Source.Family != b.Source.Family {
			return a.Source.Family < b.Source.Family
		}
		return a.Source.Path < b.Source.Path
	})
	query = strings.ToLower(query)
	var rows []Row
	for _, e := range sorted {
		contexts := matchingAncestors(e.Document.Turns, query)
		for i, t := range e.Document.Turns {
			r := Row{Entry: e, Turn: t, Position: i + 1}
			text := strings.Join([]string{e.Source.Family, e.Source.Path, e.Document.ID, e.Document.CWD, e.Document.Branch, e.Association.Worktree, e.Association.CurrentBranch, e.Document.Timestamp, t.Timestamp, t.Prompt, t.PreviousAgent, t.EarlierUser}, "\n")
			matches := strings.Contains(strings.ToLower(text), query)
			r.MatchingContext = contexts[t.ID]
			matches = matches || r.MatchingContext != ""
			if matches {
				rows = append(rows, r)
			}
		}
	}
	return rows
}

// Safe preserves the source text while rendering control characters literally.
func Safe(s string) string {
	var out strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			fmt.Fprintf(&out, "\\u%04x", r)
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'" }
func Command(r Row, output string) (string, error) {
	if output != "posix" && output != "zsh" {
		return "", fmt.Errorf("unsupported output %q; use posix or zsh (Bash/fish adapters are not yet delivered)", output)
	}
	for _, value := range []string{r.Entry.Source.Path, r.Turn.ID} {
		if strings.ContainsRune(value, 0) {
			return "", fmt.Errorf("selection contains a NUL byte and cannot be represented in shell arguments")
		}
	}
	return "unfriction inspect --format " + quote(r.Entry.Source.Family) + " --turn " + quote(r.Entry.Source.Family+":"+r.Turn.ID) + " -- " + quote(r.Entry.Source.Path) + "\n", nil
}

// Each verified ancestry edge is visited once per query. Empty queries need no
// ancestry work at all, so loading a long session cannot cause a quadratic walk.
func matchingAncestors(turns []session.Turn, query string) map[string]string {
	result := map[string]string{}
	if query == "" {
		return result
	}
	byID := map[string]session.Turn{}
	for _, t := range turns {
		byID[t.ID] = t
	}
	resolved := map[string]bool{}
	for _, t := range turns {
		var chain []session.Turn
		seen := map[string]bool{}
		for id := t.ID; id != "" && !resolved[id] && !seen[id]; {
			n, ok := byID[id]
			if !ok {
				break
			}
			seen[id] = true
			chain = append(chain, n)
			id = n.PreviousTurnID
		}
		for i := len(chain) - 1; i >= 0; i-- {
			n := chain[i]
			parent := byID[n.PreviousTurnID]
			if strings.Contains(strings.ToLower(parent.Prompt), query) {
				result[n.ID] = parent.Prompt
			} else {
				result[n.ID] = result[n.PreviousTurnID]
			}
			resolved[n.ID] = true
		}
	}
	return result
}

func snippet(text, query string, width int) string {
	text = Safe(text)
	runes := []rune(text)
	width = max(12, width)
	if len(runes) <= width {
		return text
	}
	start := 0
	lower := strings.ToLower(text)
	if query != "" {
		if at := strings.Index(lower, strings.ToLower(query)); at >= 0 {
			start = max(0, utf8.RuneCountInString(lower[:at])-width/3)
		}
	}
	end := min(len(runes), start+width-2)
	value := string(runes[start:end])
	if start > 0 {
		value = "…" + value
	}
	if end < len(runes) {
		value += "…"
	}
	return value
}
