package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/run"
)

func Write(w io.Writer, r run.Run) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Task:\n%s\n\n", r.Task)
	if len(r.Events) > 0 {
		b.WriteString("Context events:\n")
		for i, e := range r.Events {
			if e.Kind == run.Acquisition {
				fmt.Fprintf(&b, "%d. Acquisition %s: %s\n", i+1, e.AcquisitionID, e.Target)
			} else {
				text := e.Text
				if text == "" {
					text = "(empty)"
				}
				fmt.Fprintf(&b, "%d. Observation for %s:\n%s", i+1, e.AcquisitionID, text)
				if !strings.HasSuffix(text, "\n") {
					b.WriteString("\n")
				}
			}
		}
		for _, id := range r.MissingObservations() {
			fmt.Fprintf(&b, "Diagnostic: Observation missing for %s\n", id)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Final response:\n%s\n\nExecution state: %s\nCompletion describes execution termination, not verified task success.\n", r.FinalResponse, r.ExecutionState)
	_, err := io.WriteString(w, b.String())
	return err
}
