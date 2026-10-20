package run

// AcquisitionAssociation pairs a recorded request with its observation, if any.
// Associations retain request order and use reconstructed local IDs; action
// requests and results never participate. Observation is nil for pending reads.
type AcquisitionAssociation struct {
	Request     Event
	Observation *Event
}

// AcquisitionAssociations exposes the existing request/observation relationship
// for measurements without reparsing vendor records or interpreting targets.
// Reconstructed runs have at most one observation per acquisition.
func (r Run) AcquisitionAssociations() []AcquisitionAssociation {
	observations := make(map[string]Event)
	for _, e := range r.Events {
		if e.Kind == Observation {
			observations[e.AcquisitionID] = e
		}
	}
	var associations []AcquisitionAssociation
	for _, e := range r.Events {
		if e.Kind != Acquisition {
			continue
		}
		pair := AcquisitionAssociation{Request: e}
		if observation, ok := observations[e.AcquisitionID]; ok {
			pair.Observation = &observation
		}
		associations = append(associations, pair)
	}
	return associations
}

// AcquisitionFrequency counts requests and their recorded observations. Empty,
// image, and tool-error observations all count as matched. These counts say
// nothing about successful engineering outcomes.
type AcquisitionFrequency struct {
	Attempts            int
	MatchedObservations int
	MissingObservations int
}

func (r Run) AcquisitionFrequency() AcquisitionFrequency {
	var frequency AcquisitionFrequency
	for _, pair := range r.AcquisitionAssociations() {
		frequency.Attempts++
		if pair.Observation != nil {
			frequency.MatchedObservations++
		} else {
			frequency.MissingObservations++
		}
	}
	return frequency
}
