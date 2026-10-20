package run

// AcquisitionTargets measures literal target identity among acquisition requests.
// Repetition describes a revisit; it does not imply unnecessary or wasted work.
type AcquisitionTargets struct {
	DistinctTargets  int
	RepeatedRequests int
}

func (r Run) AcquisitionTargets() AcquisitionTargets {
	targets := make(map[string]struct{})
	for _, event := range r.Events {
		if event.Kind == Acquisition {
			targets[event.Target] = struct{}{}
		}
	}
	distinct := len(targets)
	return AcquisitionTargets{DistinctTargets: distinct, RepeatedRequests: r.AcquisitionFrequency().Attempts - distinct}
}
