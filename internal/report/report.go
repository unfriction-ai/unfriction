package report

import (
	"fmt"
	"io"

	"github.com/unfriction-ai/unfriction/internal/run"
)

func Write(w io.Writer, r run.Run) error {
	_, err := fmt.Fprintf(w, "Task:\n%s\n\nFinal response:\n%s\n\nExecution state: %s\nCompletion describes execution termination, not verified task success.\n", r.Task, r.FinalResponse, r.ExecutionState)
	return err
}
