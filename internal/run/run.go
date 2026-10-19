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
	Coverage       []Coverage
	Task           string
	FinalResponse  string
	ExecutionState ExecutionState
}

// Event keeps source order; SourceID and SourceLine retain input provenance.
// AcquisitionID and ActionID are local IDs, independent of vendor ID formats.
// IsError records a tool-reported error, not a verified task outcome.
type Event struct {
	Kind          EventKind
	AcquisitionID string
	ActionID      string
	IsError       bool
	Operation     ActionOperation
	Image         *ImageObservation
	Target        string
	Text          string
	SourceID      string
	SourceLine    int
}

// ImageObservation describes received image data without retaining or interpreting it.
type ImageObservation struct {
	MediaType  string
	ByteLength int
}

type ActionOperation string

const (
	ModifyFile ActionOperation = "Modify"
	CreateFile ActionOperation = "Create"
)

type EventKind string

const (
	Acquisition EventKind = "acquisition"
	Observation EventKind = "observation"
	Action      EventKind = "action"
	Result      EventKind = "result"
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
		if (e.Kind == Acquisition || e.Kind == Action) && e.SourceID == sourceID {
			return fmt.Errorf("duplicate acquisition ID %q", sourceID)
		}
		if e.Kind == Acquisition {
			count++
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
			if e.Kind == Action || e.Kind == Result {
				return fmt.Errorf("expected action result for %q", sourceID)
			}
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

// Coverage discloses recognized but uninterpreted context without retaining its
// sensitive contents or attributing it to an agent-selected acquisition.
type Coverage struct {
	SourceLine int
	Summary    string
}

// Act records an attempted file modification, never evidence of task correctness.
func (r *Run) Act(sourceID, target string, line int) error {
	if r.Task == "" || r.FinalResponse != "" || r.ExecutionState == Completed {
		return fmt.Errorf("outside action phase")
	}
	if sourceID == "" {
		return fmt.Errorf("missing action ID")
	}
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("missing action target")
	}
	count := 0
	for _, e := range r.Events {
		if (e.Kind == Acquisition || e.Kind == Action) && e.SourceID == sourceID {
			return fmt.Errorf("duplicate action ID %q", sourceID)
		}
		if e.Kind == Action {
			count++
		}
	}
	r.Events = append(r.Events, Event{Kind: Action, Operation: ModifyFile, ActionID: fmt.Sprintf("act%d", count+1), Target: target, SourceID: sourceID, SourceLine: line})
	return nil
}

// RecordResult associates a tool response with the originating request kind.
func (r *Run) RecordResult(sourceID, text string, isError bool, line int) error {
	if r.Task == "" || r.FinalResponse != "" || r.ExecutionState == Completed {
		return fmt.Errorf("outside action phase")
	}
	var request *Event
	for i := range r.Events {
		e := &r.Events[i]
		if e.SourceID != sourceID {
			continue
		}
		if e.Kind == Result {
			return fmt.Errorf("duplicate action result for %q", sourceID)
		}
		if e.Kind == Action || e.Kind == Acquisition {
			request = e
		}
	}
	if request == nil {
		return fmt.Errorf("unmatched observation or action result %q", sourceID)
	}
	if request.Kind == Acquisition {
		return r.Observe(sourceID, text, line)
	}
	r.Events = append(r.Events, Event{Kind: Result, ActionID: request.ActionID, Text: text, IsError: isError, SourceID: sourceID, SourceLine: line})
	return nil
}

func (r Run) MissingResults() []string {
	observed := map[string]bool{}
	for _, e := range r.Events {
		if e.Kind == Result {
			observed[e.ActionID] = true
		}
	}
	var missing []string
	for _, e := range r.Events {
		if e.Kind == Action && !observed[e.ActionID] {
			missing = append(missing, e.ActionID)
		}
	}
	return missing
}

// Create records an attempted file creation, not proof that the file was created.
func (r *Run) Create(sourceID, target string, line int) error {
	if err := r.Act(sourceID, target, line); err != nil {
		return err
	}
	r.Events[len(r.Events)-1].Operation = CreateFile
	return nil
}

// ObserveImage associates nontext evidence with an acquisition. Image bytes stay
// at the adapter boundary; the domain retains metadata and source provenance.
func (r *Run) ObserveImage(sourceID string, image ImageObservation, isError bool, line int) error {
	if strings.TrimSpace(image.MediaType) == "" || image.ByteLength <= 0 {
		return fmt.Errorf("invalid image observation metadata")
	}
	if err := r.Observe(sourceID, "", line); err != nil {
		return err
	}
	event := &r.Events[len(r.Events)-1]
	event.Image = &image
	event.IsError = isError
	return nil
}
