package claude

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/unfriction-ai/unfriction/internal/run"
)

// Synthetic permutations of supported M3 records exercise adapter association
// and provenance; these are not claims of additional recorded vendor variants.
func TestRepeatedFeedbackPreservesSourceOrderAndProvenance(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/claude-action.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	const start = 1
	var events []string
	for i := 1; i <= 3; i++ {
		for _, line := range lines[start : start+4] {
			events = append(events, strings.NewReplacer("read-1", fmt.Sprintf("read-%d", i), "edit-1", fmt.Sprintf("edit-%d", i), "GUIDE.md", "README.md").Replace(line))
		}
	}
	// Requests for the same path are interleaved; results need not arrive in
	// request order. Neither the adapter nor model should sort/group the stream.
	order := []int{0, 2, 4, 6, 8, 10, 7, 1, 11, 5, 3, 9}
	input := append([]string{}, lines[:start]...)
	for _, i := range order {
		input = append(input, events[i])
	}
	input = append(input, lines[start+4:]...)
	r, err := Parse(strings.NewReader(strings.Join(input, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	if r.ExecutionState != run.Completed || len(r.Events) != 12 {
		t.Fatalf("%+v", r)
	}
	kinds := []run.EventKind{run.Acquisition, run.Observation, run.Action, run.Result}
	for position, index := range order {
		got := r.Events[position]
		number := index/4 + 1
		kind := kinds[index%4]
		source, local := fmt.Sprintf("read-%d", number), fmt.Sprintf("a%d", number)
		gotLocal := got.AcquisitionID
		if kind == run.Action || kind == run.Result {
			source, local = fmt.Sprintf("edit-%d", number), fmt.Sprintf("act%d", number)
			gotLocal = got.ActionID
		}
		if got.Kind != kind || got.SourceID != source || gotLocal != local || got.SourceLine != start+position+1 {
			t.Fatalf("event %d: %+v, want %s %s %s line %d", position, got, kind, source, local, start+position+1)
		}
		if (kind == run.Acquisition || kind == run.Action) && got.Target != "README.md" {
			t.Fatalf("target lost: %+v", got)
		}
	}
	if len(r.MissingObservations()) != 0 || len(r.MissingResults()) != 0 {
		t.Fatal("feedback lost")
	}
	// A late duplicate, orphan, or unsupported request must invalidate the whole
	// parse, even after two valid read/action sequences have been reconstructed.
	for _, tc := range []struct{ name, line, want string }{
		{"duplicate request", events[0], "duplicate"},
		{"duplicate result", events[3], "duplicate"},
		{"orphan", strings.ReplaceAll(events[3], "edit-1", "unknown"), "unmatched"},
		{"unsupported", strings.ReplaceAll(events[10], "Edit", "unknown_tool"), "unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := append(append([]string{}, input[:start+12]...), tc.line)
			got, err := Parse(strings.NewReader(strings.Join(invalid, "\n")))
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), fmt.Sprintf("line %d", start+13)) || !reflect.DeepEqual(got, run.Run{}) {
				t.Fatalf("run=%+v err=%v", got, err)
			}
		})
	}
}
