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
		heading := "Context events:\n"
		for _, e := range r.Events {
			if e.Kind == run.Action {
				heading = "Run events:\n"
				break
			}
		}
		b.WriteString(heading)
		for i, e := range r.Events {
			if e.Kind == run.Acquisition {
				fmt.Fprintf(&b, "%d. Acquisition %s: %s\n", i+1, e.AcquisitionID, e.Target)
			} else if e.Kind == run.Action {
				operation := e.Operation
				if operation == "" {
					operation = run.ModifyFile
				}
				fmt.Fprintf(&b, "%d. Action %s: %s %s\n", i+1, e.ActionID, operation, e.Target)
			} else {
				text := e.Text
				if e.Image != nil {
					text = fmt.Sprintf("Image: %s, %d bytes (content not interpreted)", e.Image.MediaType, e.Image.ByteLength)
				}
				if text == "" {
					text = "(empty)"
				}
				label, id := "Observation", e.AcquisitionID
				if e.Kind == run.Result {
					label, id = "Result", e.ActionID
				}
				fmt.Fprintf(&b, "%d. %s for %s:\n%s", i+1, label, id, text)
				if !strings.HasSuffix(text, "\n") {
					b.WriteString("\n")
				}
				if e.IsError {
					b.WriteString("Tool reported an error.\n")
				}
			}
		}
		for _, id := range r.MissingResults() {
			fmt.Fprintf(&b, "Diagnostic: Result missing for %s\n", id)
		}
		for _, id := range r.MissingObservations() {
			fmt.Fprintf(&b, "Diagnostic: Observation missing for %s\n", id)
		}
		b.WriteString("\n")
	}
	if len(r.Coverage) > 0 {
		b.WriteString("Context coverage:\n")
		for _, notice := range r.Coverage {
			fmt.Fprintf(&b, "- Source line %d: %s\n", notice.SourceLine, notice.Summary)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Final response:\n%s\n\nExecution state: %s\nCompletion describes execution termination, not verified task success.\n", r.FinalResponse, r.ExecutionState)
	frequency := r.AcquisitionFrequency()
	if frequency.Attempts > 0 {
		fmt.Fprintf(&b, "\nAcquisition measurements:\n- Attempts: %d\n- Matched observations: %d\n- Missing observations: %d\n", frequency.Attempts, frequency.MatchedObservations, frequency.MissingObservations)
	}
	_, err := io.WriteString(w, b.String())
	return err
}
