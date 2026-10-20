package run

// AcquisitionCounts counts acquisition events; it does not interpret targets,
// payloads, or outcomes. For a valid run, Attempts = Matched + Missing.
type AcquisitionCounts struct {
	Attempts int // acquisition requests, including those with no recorded result
	Matched  int // observations associated with a request, including images and tool errors
	Missing  int // requests with no observation; len(MissingObservations())
}

func (r Run) AcquisitionCounts() AcquisitionCounts {
	var c AcquisitionCounts
	for _, e := range r.Events {
		switch e.Kind {
		case Acquisition:
			c.Attempts++
		case Observation:
			c.Matched++
		}
	}
	c.Missing = len(r.MissingObservations())
	return c
}
