//go:build verify

// Bounded exhaustive verification of Claude Code turn isolation.
//
// Run with: go test -tags verify ./internal/claude -run Verify
//
// The check enumerates every small session graph over a fixed record alphabet,
// runs ParseTurn for every recorded prompt, and compares the outcome with an
// independent oracle computed from the abstract graph (not from the adapter's
// code). The verify-claude-turn-isolation skill documents the properties,
// bounds, and coverage record.
package claude

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/unfriction-ai/unfriction/internal/run"
)

// Record kinds. Each maps to one physical JSONL line in vGraph.json.
const (
	kPrompt = iota // user prompt string: starts a turn
	kCall          // assistant Read tool_use with id t<i>, stop_reason tool_use
	kFinal         // assistant text, stop_reason end_turn
	kResult        // user tool_result for tool id t<target>
	kSystem        // system turn_duration ancestry node (bookkeeping)
)

const (
	parentNull    = -1
	parentMissing = -2
)

type vRecord struct {
	kind      int
	parent    int // record index, parentNull, or parentMissing
	target    int // kResult only: index whose tool id t<target> is referenced
	sidechain bool
}

type vGraph []vRecord

func toolID(target int) string {
	if target < 0 {
		return "t-unrecorded"
	}
	return fmt.Sprintf("t%d", target)
}

func (g vGraph) json(i int) string {
	r := g[i]
	parent := "null"
	switch {
	case r.parent == parentMissing:
		parent = `"gone"`
	case r.parent >= 0:
		parent = fmt.Sprintf(`"n%d"`, r.parent)
	}
	side := ""
	if r.sidechain {
		side = `,"isSidechain":true`
	}
	head := fmt.Sprintf(`"uuid":"n%d","parentUuid":%s,"sessionId":"s"%s`, i, parent, side)
	switch r.kind {
	case kPrompt:
		return fmt.Sprintf(`{"type":"user",%s,"message":{"role":"user","content":"prompt n%d"}}`, head, i)
	case kCall:
		return fmt.Sprintf(`{"type":"assistant",%s,"message":{"role":"assistant","content":[{"type":"tool_use","id":"t%d","name":"Read","input":{"file_path":"f%d.md"}}],"stop_reason":"tool_use"}}`, head, i, i)
	case kFinal:
		return fmt.Sprintf(`{"type":"assistant",%s,"message":{"role":"assistant","content":[{"type":"text","text":"final n%d"}],"stop_reason":"end_turn"}}`, head, i)
	case kResult:
		return fmt.Sprintf(`{"type":"user",%s,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"%s","content":"payload n%d"}]}}`, head, toolID(r.target), i)
	default:
		return fmt.Sprintf(`{"type":"system","subtype":"turn_duration",%s}`, head)
	}
}

func (g vGraph) recording() string {
	lines := make([]string, len(g))
	for i := range g {
		lines[i] = g.json(i)
	}
	return strings.Join(lines, "\n")
}

// oracle is computed only from the abstract graph.
type oracle struct {
	g       vGraph
	cyclic  bool
	missing bool
	owner   []int // prompt index owning each record, -1 when none (or cyclic)
}

func newOracle(g vGraph) oracle {
	o := oracle{g: g, owner: make([]int, len(g))}
	for i := range g {
		seen := map[int]bool{}
		o.owner[i] = -1
		for n := i; ; {
			if seen[n] {
				o.cyclic = true
				break
			}
			seen[n] = true
			if g[n].kind == kPrompt {
				o.owner[i] = n
				break
			}
			if g[n].parent == parentMissing {
				o.missing = true
				break
			}
			if g[n].parent == parentNull {
				break
			}
			n = g[n].parent
		}
		// Cycles and dangling parents anywhere in the chain, prompt or not.
		seen = map[int]bool{}
		for n := i; n >= 0; n = g[n].parent {
			if seen[n] {
				o.cyclic = true
				break
			}
			seen[n] = true
			if g[n].parent == parentMissing {
				o.missing = true
			}
		}
	}
	return o
}

func (o oracle) ancestorOf(a, b int) bool { // a is a strict ancestor of b
	for n := o.g[b].parent; n >= 0; n = o.g[n].parent {
		if n == a {
			return true
		}
		if n == b {
			return false
		}
	}
	return false
}

// mustReject returns why selecting prompt p must produce a diagnostic, or "".
func (o oracle) mustReject(p int) string {
	g := o.g
	if o.cyclic {
		return "cyclic ancestry"
	}
	if o.missing {
		return "unresolved ancestry"
	}
	for n := p; n >= 0; n = g[n].parent {
		if g[n].sidechain {
			return "sidechain in selected prompt ancestry"
		}
	}
	var assistants []int
	for i, r := range g {
		if o.owner[i] != p {
			continue
		}
		if r.sidechain {
			return "sidechain record owned by selected turn"
		}
		if r.kind == kCall || r.kind == kFinal {
			assistants = append(assistants, i)
		}
	}
	for k := 1; k < len(assistants); k++ {
		if !o.ancestorOf(assistants[k-1], assistants[k]) {
			return "branched assistant continuations in selected turn"
		}
	}
	for i, r := range g {
		if r.kind != kResult {
			continue
		}
		call := r.target
		if call < 0 || g[call].kind != kCall || call > i {
			if o.owner[i] == p {
				return "selected tool result lacks an earlier recorded request"
			}
			continue
		}
		if o.owner[i] != o.owner[call] && (o.owner[i] == p || o.owner[call] == p) {
			return "tool result crosses the selected turn's ownership"
		}
	}
	return ""
}

type eventKey struct {
	kind run.EventKind
	id   string
	line int
}

// checkReport verifies an accepted report against the oracle.
func (o oracle) checkReport(p int, r run.Run, turnIndex int, prompts []int, earlierUser, previousAgent string) string {
	g := o.g
	if r.Task != fmt.Sprintf("prompt n%d", p) {
		return fmt.Sprintf("task %q is not the selected prompt n%d", r.Task, p)
	}
	if turnIndex < 0 || turnIndex >= len(prompts) || prompts[turnIndex] != p {
		return fmt.Sprintf("selected turn position %d does not name prompt n%d", turnIndex, p)
	}
	want := map[eventKey]bool{}
	final := ""
	for i, rec := range g {
		if o.owner[i] != p {
			continue
		}
		switch rec.kind {
		case kCall:
			want[eventKey{run.Acquisition, fmt.Sprintf("t%d", i), i + 1}] = true
		case kResult:
			want[eventKey{run.Observation, toolID(rec.target), i + 1}] = true
		case kFinal:
			final = fmt.Sprintf("final n%d", i)
		}
	}
	got := map[eventKey]bool{}
	for _, e := range r.Events {
		k := eventKey{e.Kind, e.SourceID, e.SourceLine}
		if !want[k] {
			return fmt.Sprintf("report contains %s %s from line %d, which turn n%d does not own", e.Kind, e.SourceID, e.SourceLine, p)
		}
		got[k] = true
	}
	for k := range want {
		if !got[k] {
			return fmt.Sprintf("report omits %s %s from line %d owned by turn n%d", k.kind, k.id, k.line, p)
		}
	}
	if r.FinalResponse != final || (final != "") != (r.ExecutionState == run.Completed) {
		return fmt.Sprintf("final response %q / state %s, owned final is %q", r.FinalResponse, r.ExecutionState, final)
	}
	wantUser, wantAgent := "", ""
	if parent := g[p].parent; parent >= 0 {
		if u := o.owner[parent]; u >= 0 && u < p {
			wantUser = fmt.Sprintf("prompt n%d", u)
			var finals []string
			for i, rec := range g {
				if o.owner[i] == u && rec.kind == kFinal {
					finals = append(finals, fmt.Sprintf("final n%d", i))
				}
			}
			wantAgent = strings.Join(finals, "\n")
		}
	}
	if earlierUser != wantUser || previousAgent != wantAgent {
		return fmt.Sprintf("earlier context user=%q agent=%q, recorded ancestry gives user=%q agent=%q", earlierUser, previousAgent, wantUser, wantAgent)
	}
	return ""
}

func (g vGraph) describe() string {
	var b strings.Builder
	for i, r := range g {
		parent := "null"
		switch {
		case r.parent == parentMissing:
			parent = "gone"
		case r.parent >= 0:
			parent = fmt.Sprintf("n%d", r.parent)
		}
		kind := [...]string{"user prompt", "assistant Read call", "assistant end_turn text", "user tool_result", "system turn_duration"}[r.kind]
		if r.kind == kResult {
			kind += " for " + toolID(r.target)
		}
		if r.sidechain {
			kind += " (sidechain)"
		}
		fmt.Fprintf(&b, "\n  line %d: n%d %s, parentUuid %s", i+1, i, kind, parent)
	}
	return b.String() + "\n  JSONL:\n" + g.recording()
}

// enumerate calls visit for every graph with 1..n records. Each record picks a
// kind and a parent: null, a missing uuid, or any record (itself and later
// records included, so cycles and forward edges occur). Each tool_result then
// references any recorded call (earlier or later) or a tool id no record has.
// Every graph is visited once with no sidechain and once per record marked as
// the single sidechain record.
func enumerate(n int, visit func(vGraph)) {
	for size := 1; size <= n; size++ {
		g := make(vGraph, size)
		var shape, targets func(i int)
		targets = func(i int) {
			if i == size {
				visit(append(vGraph(nil), g...))
				for s := range g {
					sg := append(vGraph(nil), g...)
					sg[s].sidechain = true
					visit(sg)
				}
				return
			}
			if g[i].kind != kResult {
				targets(i + 1)
				return
			}
			g[i].target = parentNull // a tool id that no record carries
			targets(i + 1)
			for c := range g {
				if g[c].kind == kCall {
					g[i].target = c
					targets(i + 1)
				}
			}
		}
		shape = func(i int) {
			if i == size {
				targets(0)
				return
			}
			for parent := parentMissing; parent < size; parent++ {
				for _, kind := range []int{kPrompt, kCall, kFinal, kResult, kSystem} {
					g[i] = vRecord{kind: kind, parent: parent}
					shape(i + 1)
				}
			}
		}
		shape(0)
	}
}

// bound is the largest recording size enumerated. UNFRICTION_VERIFY_RECORDS
// overrides the per-test default for every test.
func bound(t *testing.T, n int) int {
	if v := os.Getenv("UNFRICTION_VERIFY_RECORDS"); v != "" {
		var err error
		if n, err = strconv.Atoi(v); err != nil || n < 1 {
			t.Fatalf("UNFRICTION_VERIFY_RECORDS=%q must be a positive integer", v)
		}
	}
	return n
}

// parallel runs check over every enumerated graph and reports the first
// (smallest-first per worker) counterexamples.
func parallel(t *testing.T, n int, check func(vGraph) string) (cases int64) {
	jobs := make(chan vGraph, 1024)
	var mu sync.Mutex
	var failures []string
	var count int64
	var wg sync.WaitGroup
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for g := range jobs {
				atomic.AddInt64(&count, 1)
				if msg := check(g); msg != "" {
					mu.Lock()
					failures = append(failures, fmt.Sprintf("%d records: %s%s", len(g), msg, g.describe()))
					mu.Unlock()
				}
			}
		}()
	}
	enumerate(n, func(g vGraph) { jobs <- g })
	close(jobs)
	wg.Wait()
	sort.Slice(failures, func(i, j int) bool { return len(failures[i]) < len(failures[j]) })
	for i, f := range failures {
		if i == 3 {
			t.Errorf("... and %d more counterexamples", len(failures)-3)
			break
		}
		t.Errorf("counterexample: %s", f)
	}
	return count
}

func prompts(g vGraph) []int {
	var ps []int
	for i, r := range g {
		if r.kind == kPrompt {
			ps = append(ps, i)
		}
	}
	return ps
}

// TestVerifyTurnIsolation: a selected turn's report contains exactly the
// records whose parentUuid chain reaches that prompt first, and unsafe
// ancestry yields a diagnostic instead of a report.
func TestVerifyTurnIsolation(t *testing.T) {
	n := bound(t, 4) // 4 records is the smallest bound holding a complete prompt/call/result/final turn
	var accepted int64
	cases := parallel(t, n, func(g vGraph) string {
		o := newOracle(g)
		ps := prompts(g)
		input := g.recording()
		for _, p := range ps {
			r, d, index, err := ParseTurn(strings.NewReader(input), fmt.Sprintf("n%d", p))
			reason := o.mustReject(p)
			if err != nil {
				continue // a diagnostic is always a safe outcome
			}
			if reason != "" {
				return fmt.Sprintf("turn n%d accepted despite %s", p, reason)
			}
			if index >= len(d.Turns) {
				return fmt.Sprintf("turn n%d accepted with position %d of %d", p, index, len(d.Turns))
			}
			var docPrompts []int
			for _, turn := range d.Turns {
				k, _ := strconv.Atoi(strings.TrimPrefix(turn.ID, "n"))
				docPrompts = append(docPrompts, k)
			}
			if msg := o.checkReport(p, r, index, docPrompts, d.Turns[index].EarlierUser, d.Turns[index].PreviousAgent); msg != "" {
				return msg
			}
			atomic.AddInt64(&accepted, 1)
		}
		return ""
	})
	t.Logf("checked %d recordings of up to %d records; %d selected turns produced a report", cases, n, accepted)
	if accepted == 0 {
		t.Fatal("no selected turn was ever accepted; the check is vacuous")
	}
}

// TestVerifyLinearSessionsAccepted guards against the trivial way to satisfy
// isolation: rejecting everything. Every turn of a linear multi-turn session
// built from supported shapes must produce a report.
func TestVerifyLinearSessionsAccepted(t *testing.T) {
	shapes := [][]int{
		{kPrompt, kFinal},
		{kPrompt, kCall, kResult, kFinal},
		{kPrompt, kSystem, kCall, kResult, kCall, kResult, kFinal},
		{kPrompt, kCall, kResult},
	}
	for _, a := range shapes {
		for _, b := range shapes {
			for _, c := range shapes {
				var g vGraph
				lastCall := -1
				for _, shape := range [][]int{a, b, c} {
					for _, kind := range shape {
						r := vRecord{kind: kind, parent: len(g) - 1}
						if kind == kResult {
							r.target = lastCall
						}
						if kind == kCall {
							lastCall = len(g)
						}
						if len(g) == 0 {
							r.parent = parentNull
						}
						g = append(g, r)
					}
				}
				o := newOracle(g)
				ps := prompts(g)
				for _, p := range ps {
					r, d, index, err := ParseTurn(strings.NewReader(g.recording()), fmt.Sprintf("n%d", p))
					if err != nil {
						t.Fatalf("linear turn n%d rejected: %v%s", p, err, g.describe())
					}
					var docPrompts []int
					for _, turn := range d.Turns {
						k, _ := strconv.Atoi(strings.TrimPrefix(turn.ID, "n"))
						docPrompts = append(docPrompts, k)
					}
					if msg := o.checkReport(p, r, index, docPrompts, d.Turns[index].EarlierUser, d.Turns[index].PreviousAgent); msg != "" {
						t.Fatalf("%s%s", msg, g.describe())
					}
				}
			}
		}
	}
}

// TestVerifyDeterministicOutcome: the same recording bytes and selector always
// give the same outcome: the same report, or the same diagnostic text.
func TestVerifyDeterministicOutcome(t *testing.T) {
	n := bound(t, 3) // six parses per selector; 3 records already reach every ancestry diagnostic
	var known, unknown int64
	parallel(t, n, func(g vGraph) string {
		input := g.recording()
		ids := []string{"absent"}
		for _, p := range prompts(g) {
			ids = append(ids, fmt.Sprintf("n%d", p))
		}
		for _, id := range ids {
			first := ""
			for attempt := 0; attempt < 6; attempt++ {
				r, _, _, err := ParseTurn(strings.NewReader(input), id)
				outcome := fmt.Sprintf("%+v", r)
				if err != nil {
					outcome = "diagnostic: " + err.Error()
				}
				if attempt == 0 {
					first = outcome
				} else if outcome != first {
					tag := "NEW"
					if ancestryDiagnostic(first) && ancestryDiagnostic(outcome) {
						tag = "F1"
						atomic.AddInt64(&known, 1)
					} else {
						atomic.AddInt64(&unknown, 1)
					}
					return fmt.Sprintf("[%s] selecting %s gave different outcomes for the same bytes:\n    run 1: %s\n    run %d: %s", tag, id, first, attempt+1, outcome)
				}
			}
		}
		return ""
	})
	if known+unknown > 0 {
		t.Logf("nondeterministic recordings: %d match known finding F1 (ancestry-validation diagnostic depends on map order), %d are NEW", known, unknown)
	}
}

// ancestryDiagnostic recognizes the parent-edge validation diagnostics in
// readSession, whose reporting node is chosen by Go map iteration (finding F1).
func ancestryDiagnostic(outcome string) bool {
	return strings.HasPrefix(outcome, "diagnostic: ") && (strings.HasSuffix(outcome, ": cycle in recorded ancestry") || strings.HasSuffix(outcome, ": missing required ancestor"))
}
