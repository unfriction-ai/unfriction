// Package run defines the vendor-independent representation of one task execution.
package run

import (
	"fmt"
	"strings"
)

type ExecutionState string

const (
	Incomplete ExecutionState = "INCOMPLETE"
	Completed  ExecutionState = "COMPLETED"
)

type Run struct {
	Events         []Event
	Task           string
	FinalResponse  string
	ExecutionState ExecutionState
}

// Event keeps source order; SourceID and SourceLine retain input provenance.
// AcquisitionID is local to the run and does not depend on a vendor's ID format.
type Event struct {
	Kind          EventKind
	AcquisitionID string
	Target        string
	Text          string
	SourceID      string
	SourceLine    int
}

type EventKind string

const (
	Acquisition EventKind = "acquisition"
	Observation EventKind = "observation"
)

func (r *Run) Acquire(sourceID, target string, line int) error {
	if r.Task == "" || r.FinalResponse != "" || r.ExecutionState == Completed {
		return fmt.Errorf("outside acquisition phase")
	}
	if sourceID == "" {
		return fmt.Errorf("missing acquisition ID")
	}
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("missing acquisition target")
	}
	count := 0
	for _, e := range r.Events {
		if e.Kind == Acquisition {
			count++
			if e.SourceID == sourceID {
				return fmt.Errorf("duplicate acquisition ID %q", sourceID)
			}
		}
	}
	r.Events = append(r.Events, Event{Kind: Acquisition, AcquisitionID: fmt.Sprintf("a%d", count+1), Target: target, SourceID: sourceID, SourceLine: line})
	return nil
}

func (r *Run) Observe(sourceID, text string, line int) error {
	if r.Task == "" || r.FinalResponse != "" || r.ExecutionState == Completed {
		return fmt.Errorf("outside acquisition phase")
	}
	id := ""
	for _, e := range r.Events {
		if e.SourceID == sourceID {
			if e.Kind == Observation {
				return fmt.Errorf("duplicate observation for %q", sourceID)
			}
			id = e.AcquisitionID
		}
	}
	if id == "" {
		return fmt.Errorf("unmatched observation %q", sourceID)
	}
	r.Events = append(r.Events, Event{Kind: Observation, AcquisitionID: id, Text: text, SourceID: sourceID, SourceLine: line})
	return nil
}

func (r Run) MissingObservations() []string {
	observed := map[string]bool{}
	for _, e := range r.Events {
		if e.Kind == Observation {
			observed[e.AcquisitionID] = true
		}
	}
	var missing []string
	for _, e := range r.Events {
		if e.Kind == Acquisition && !observed[e.AcquisitionID] {
			missing = append(missing, e.AcquisitionID)
		}
	}
	return missing
}
