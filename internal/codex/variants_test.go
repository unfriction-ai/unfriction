package codex

import (
	"github.com/unfriction-ai/unfriction/internal/run"
	"os"
	"strings"
	"testing"
)

func TestRecordedPatchVariants(t *testing.T) {
	for _, name := range []string{"codex-create", "codex-delete-lines", "codex-rejected"} {
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile("../../fixtures/tool-variants/" + name + ".jsonl")
			if err != nil {
				t.Fatal(err)
			}
			r, err := Parse(strings.NewReader(string(b)))
			if err != nil {
				t.Fatal(err)
			}
			if r.ExecutionState != run.Completed || len(r.MissingResults()) != 0 {
				t.Fatalf("%+v", r)
			}
			if name == "codex-rejected" {
				last := r.Events[len(r.Events)-1]
				if last.Text != "patch rejected by user" || last.IsError {
					t.Fatalf("rejection was misrepresented: %+v", last)
				}
				lines := strings.Split(strings.TrimSpace(string(b)), "\n")
				r, err = Parse(strings.NewReader(strings.Join(lines[:len(lines)-2], "\n")))
				if err != nil || r.ExecutionState != run.Incomplete {
					t.Fatalf("rejection inferred termination: %+v %v", r, err)
				}
			}
		})
	}
}
