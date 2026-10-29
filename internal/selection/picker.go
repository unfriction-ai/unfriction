package selection

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/unfriction-ai/unfriction/internal/recordings"
)

var (
	ErrCancelled  = errors.New("selection cancelled")
	ErrNoTerminal = errors.New("selection requires a controlling terminal; run interactively or use inspect --turn <selector> -- <path>")
	ErrEmpty      = errors.New("no supported recordings found for this repository; check CODEX_HOME / CLAUDE_CONFIG_DIR or use inspect with a known path")
)

type Options struct {
	CWD     string
	Sources recordings.Config
}
type discoveryMsg struct {
	update recordings.Update
	closed bool
}
type verifiedMsg struct {
	row Row
	err error
}
type picker struct {
	ctx                         context.Context
	repo                        recordings.Repository
	updates                     <-chan recordings.Update
	entries                     []recordings.Entry
	pendingSources              map[recordings.Source]bool
	rows                        []Row
	cursor                      int
	query                       string
	width, height               int
	pending, found, skipped     int
	errors                      []string
	done, validating, cancelled bool
	accepted                    *Row
	failure                     error
	notice                      string
	details                     bool
	previewOffset               int
}

func waitUpdate(ch <-chan recordings.Update) tea.Cmd {
	return func() tea.Msg { u, ok := <-ch; return discoveryMsg{update: u, closed: !ok} }
}
func (m picker) Init() tea.Cmd { return waitUpdate(m.updates) }
func (m *picker) rebuild(preserve bool) {
	key := ""
	if preserve && m.cursor < len(m.rows) {
		key = m.rows[m.cursor].Key()
	}
	m.rows = Rows(m.entries, m.query)
	m.cursor = 0
	if key != "" {
		for i, r := range m.rows {
			if r.Key() == key {
				m.cursor = i
				break
			}
		}
	}
}
func (m picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case discoveryMsg:
		if msg.closed {
			m.done = true
			if len(m.entries) == 0 {
				m.failure = ErrEmpty
				if len(m.errors) > 0 {
					m.failure = fmt.Errorf("%w; skipped sources: %s", ErrEmpty, strings.Join(m.errors, "; "))
				}
				return m, tea.Quit
			}
			return m, nil
		}
		u := msg.update
		if u.Pending {
			if m.pendingSources == nil {
				m.pendingSources = map[recordings.Source]bool{}
			}
			m.pendingSources[u.Source] = true
			m.pending = len(m.pendingSources)
			m.found++
		} else {
			delete(m.pendingSources, u.Source)
			m.pending = len(m.pendingSources)
			if u.Err != nil {
				m.skipped++
				if len(m.errors) < 3 {
					m.errors = append(m.errors, Safe(u.Source.Path+": "+u.Err.Error()))
				}
			}
			if u.Entry != nil {
				m.entries = append(m.entries, *u.Entry)
				m.rebuild(true)
			}
		}
		return m, waitUpdate(m.updates)
	case verifiedMsg:
		m.validating = false
		if msg.err != nil {
			m.notice = msg.err.Error() + "; restart selection to refresh"
			return m, nil
		}
		m.accepted = &msg.row
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
		if m.validating {
			return m, nil
		}
		if msg.String() == "tab" {
			m.details = !m.details
			m.previewOffset = 0
			return m, nil
		}
		if m.details && msg.String() != "enter" {
			page := max(1, m.height-4)
			switch msg.String() {
			case "up", "ctrl+p":
				m.previewOffset = max(0, m.previewOffset-1)
			case "down", "ctrl+n":
				m.previewOffset++
			case "pgup":
				m.previewOffset = max(0, m.previewOffset-page)
			case "pgdown":
				m.previewOffset += page
			case "home":
				m.previewOffset = 0
			case "end":
				m.previewOffset = len(m.detailLines())
			}
			m.previewOffset = min(m.previewOffset, max(0, len(m.detailLines())-page))
			return m, nil
		}
		switch msg.String() {
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "ctrl+n":
			if m.cursor+1 < len(m.rows) {
				m.cursor++
			}
		case "left", "right":
			if len(m.rows) == 0 {
				return m, nil
			}
			current := m.rows[m.cursor]
			target := current.Position - 1
			if msg.String() == "left" {
				target--
			} else {
				target++
			}
			if target >= 0 && target < len(current.Entry.Document.Turns) {
				m.query = ""
				m.rebuild(false)
				for i, r := range m.rows {
					if r.Entry.Source == current.Entry.Source && r.Position == target+1 {
						m.cursor = i
						break
					}
				}
			}
		case "enter":
			if len(m.rows) == 0 {
				return m, nil
			}
			row := m.rows[m.cursor]
			m.validating = true
			m.notice = "Revalidating recording…"
			return m, func() tea.Msg { return verifiedMsg{row: row, err: row.Entry.Revalidate(m.ctx)} }
		case "backspace", "ctrl+h":
			r := []rune(m.query)
			if len(r) > 0 {
				m.query = string(r[:len(r)-1])
				m.rebuild(false)
			}
		case "ctrl+u":
			m.query = ""
			m.rebuild(false)
		default:
			if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
				m.query += string(msg.Runes)
				if msg.Type == tea.KeySpace && len(msg.Runes) == 0 {
					m.query += " "
				}
				m.rebuild(false)
			}
		}
	}
	return m, nil
}
func known(value, label string) string {
	if value == "" {
		return "unknown " + label
	}
	return value
}
func (m picker) View() string {
	if m.details && len(m.rows) > 0 {
		lines := m.detailLines()
		height := max(1, m.height-4)
		first := min(m.previewOffset, max(0, len(lines)-height))
		last := min(len(lines), first+height)
		return "Full turn context\n" + strings.Join(lines[first:last], "\n") + "\n" + Safe(m.notice) + "\n↑↓ / PgUp/PgDn scroll · Tab back · Enter use · Esc cancel\n"
	}

	width := m.width
	if width < 1 {
		width = 80
	}
	var out strings.Builder
	line := func(text string) { out.WriteString(ansi.Truncate(Safe(text), width-1, "…")); out.WriteByte('\n') }
	line("Select recorded work · repository: " + m.repo.Current)
	line("Search: " + m.query)
	status := fmt.Sprintf("Loading · %d sources found · %d pending · %d skipped", m.found, m.pending, m.skipped)
	if m.done {
		status = fmt.Sprintf("Discovery complete · %d sessions · %d skipped", len(m.entries), m.skipped)
	}
	line(status)
	var loading []string
	for source := range m.pendingSources {
		loading = append(loading, source.Family+": "+source.Path)
	}
	sort.Strings(loading)
	for _, source := range loading[:min(3, len(loading))] {
		line("Pending summary: " + source)
	}
	reserved := 3 + min(3, len(loading)) + 9 + len(m.errors) + 2
	if m.notice != "" {
		reserved++
	}
	count := m.height - reserved
	if count < 2 {
		count = 2
	}
	first := 0
	if m.cursor >= count {
		first = m.cursor - count + 1
	}
	last := min(len(m.rows), first+count)
	for i := first; i < last; i++ {
		r := m.rows[i]
		a := r.Entry.Association
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		group := a.Worktree
		if a.Current {
			group = "current worktree"
		}
		if a.Uncertain {
			group = "UNCERTAIN repository candidate"
		}
		line(fmt.Sprintf("%s%s / %s / %s %s / Turn %d of %d: %s", marker, group, known(a.RecordedBranch, "branch"), r.Entry.Source.Family, known(r.Entry.Document.Timestamp, "timestamp"), r.Position, len(r.Entry.Document.Turns), r.Turn.Prompt))
	}
	if len(m.rows) == 0 && len(m.entries) > 0 {
		line("No search matches; Ctrl+U clears the search.")
	}
	if len(m.rows) > 0 {
		r := m.rows[m.cursor]
		a := r.Entry.Association
		line("Session: " + r.Entry.Document.ID + " · Source: " + r.Entry.Source.Path)
		line("User: " + snippet(r.Turn.Prompt, m.query, width-8))
		line("Previous agent: " + snippet(known(r.Turn.PreviousAgent, "context"), m.query, width-18))
		line("Earlier user: " + snippet(known(r.Turn.EarlierUser, "context"), m.query, width-16))
		if r.MatchingContext != "" {
			line("Earlier matching user: " + snippet(r.MatchingContext, m.query, width-25))
		}
		line("Recorded branch: " + known(a.RecordedBranch, "branch") + " · Worktree current branch: " + known(a.CurrentBranch, "branch"))
		line("Association: " + a.Evidence)
		line("Inspection support is checked by inspect; discovery does not validate tool semantics.")
	}
	for _, err := range m.errors {
		line("Skipped: " + err)
	}
	if m.notice != "" {
		line(m.notice)
	}
	line("Type to search · ↑↓ select · ←→ neighbors (clears search)")
	line("Tab full context · Enter use · Esc cancel · Ctrl+U clear")
	return out.String()
}

// Run uses only the controlling terminal for UI, reserving stdout for the caller.
func Run(ctx context.Context, options Options) (Row, error) {
	terminal, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return Row{}, ErrNoTerminal
	}
	defer terminal.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	repo, err := recordings.ResolveRepository(ctx, options.CWD)
	if err != nil {
		return Row{}, err
	}
	model := picker{ctx: ctx, repo: repo, updates: recordings.Discover(ctx, repo, options.Sources), width: 100, height: 30}
	program := tea.NewProgram(model, tea.WithInput(terminal), tea.WithOutput(terminal), tea.WithContext(ctx), tea.WithAltScreen())
	result, err := program.Run()
	if err != nil {
		return Row{}, err
	}
	final := result.(picker)
	if final.cancelled {
		return Row{}, ErrCancelled
	}
	if final.failure != nil {
		return Row{}, final.failure
	}
	if final.accepted == nil {
		return Row{}, ErrCancelled
	}
	return *final.accepted, nil
}

func (m picker) detailLines() []string {
	if len(m.rows) == 0 {
		return nil
	}
	r := m.rows[m.cursor]
	a := r.Entry.Association
	fields := []string{"Session: " + r.Entry.Document.ID, "Source: " + r.Entry.Source.Path, "Turn: " + r.Turn.ID, "User: " + r.Turn.Prompt, "Previous agent: " + known(r.Turn.PreviousAgent, "context"), "Earlier user: " + known(r.Turn.EarlierUser, "context"), "Earlier matching user: " + r.MatchingContext, "Worktree: " + a.Worktree, "Recorded branch: " + known(a.RecordedBranch, "branch"), "Worktree current branch: " + known(a.CurrentBranch, "branch"), "Association: " + a.Evidence}
	var lines []string
	for _, field := range fields {
		lines = append(lines, strings.Split(ansi.Wrap(Safe(field), max(1, m.width-1), ""), "\n")...)
		lines = append(lines, "")
	}
	return lines
}
