package run

// RecordedDuration is validated vendor operation timing in seconds. Seconds
// preserves the source's precision, including rounded recorded zero.
// This is not necessarily the complete process lifetime.
type RecordedDuration struct {
	Seconds string
}

// AcquisitionTiming retains request identity and the line supplying duration.
// Nil Duration means unavailable, including a request with no observation.
type AcquisitionTiming struct {
	AcquisitionID string
	SourceID      string
	SourceLine    int
	Duration      *RecordedDuration
}

func (r Run) AcquisitionTimings() []AcquisitionTiming {
	var timings []AcquisitionTiming
	for _, pair := range r.AcquisitionAssociations() {
		timing := AcquisitionTiming{AcquisitionID: pair.Request.AcquisitionID, SourceID: pair.Request.SourceID, SourceLine: pair.Request.SourceLine}
		if pair.Observation != nil {
			timing.SourceLine = pair.Observation.SourceLine
			timing.Duration = pair.Observation.Duration
		}
		timings = append(timings, timing)
	}
	return timings
}
