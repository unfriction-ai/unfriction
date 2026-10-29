package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/run"
)

func Write(w io.Writer, r run.Run) error {
	var b strings.Builder
	if s := r.Selection; s != nil {
		fmt.Fprintf(&b, "Selected turn:\nSource: %s\nSession: %s\nTurn %d of %d: %s\n", s.Path, s.Session, s.Position, s.Total, s.Turn)
		if s.PreviousAgent != "" {
			fmt.Fprintf(&b, "Previous agent: %s\n", s.PreviousAgent)
		}
		if s.EarlierUser != "" {
			fmt.Fprintf(&b, "Earlier user: %s\n", s.EarlierUser)
		}
		b.WriteString("Surrounding context is explanatory and excluded from selected-turn measurements.\n\n")
	}
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
			if r.Selection != nil {
				fmt.Fprintf(&b, "Recorded event: %s (source line %d)\n", e.SourceID, e.SourceLine)
			}
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
		volume := r.AcquisitionVolume()
		fmt.Fprintf(&b, "- Observed text: %d bytes (UTF-8)\n- Observed images: %d bytes (decoded)\n", volume.TextBytes, volume.ImageBytes)
		targets := r.AcquisitionTargets()
		fmt.Fprintf(&b, "- Distinct targets: %d\n- Repeated requests: %d\n", targets.DistinctTargets, targets.RepeatedRequests)
		b.WriteString("Volume describes normalized observed result payload, not original file size, model tokens, or retained context.\n")
		b.WriteString("Recorded tool-execution duration:\n")
		for _, timing := range r.AcquisitionTimings() {
			if timing.Duration == nil {
				fmt.Fprintf(&b, "- %s: unavailable (source line %d)\n", timing.AcquisitionID, timing.SourceLine)
			} else {
				fmt.Fprintf(&b, "- %s: %s seconds (source line %d)\n", timing.AcquisitionID, timing.Duration.Seconds, timing.SourceLine)
			}
		}
		b.WriteString("Durations describe vendor execution/output-wait intervals, not full process lifetime, model inference, or task elapsed time; no total is inferred.\n")
	}
	if len(r.ModelUsage) > 0 {
		b.WriteString("\nRecorded model usage:\n")
		for i, usage := range r.ModelUsage {
			fmt.Fprintf(&b, "Model usage m%d: %s (source line %d)\n", i+1, usage.Scope, usage.SourceLine)
			if usage.Unavailable != "" {
				fmt.Fprintf(&b, "- unavailable: %s\n", usage.Unavailable)
				continue
			}
			for _, counter := range usage.Counters {
				if counter.Tokens == nil {
					fmt.Fprintf(&b, "- %s: unavailable\n", counter.Name)
				} else {
					fmt.Fprintf(&b, "- %s: %d tokens\n", counter.Name, *counter.Tokens)
				}
			}
		}
		b.WriteString("Vendor-reported model usage is not acquisition-only; session snapshots may include earlier tasks; no token allocation or cost is inferred.\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}
