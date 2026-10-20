package run

// UsageCounter preserves a vendor counter category. Nil is unavailable, while a
// nonnil zero is reported zero. Categories may overlap and are never summed.
type UsageCounter struct {
	Name   string
	Tokens *int64
}

// ModelUsage is vendor-reported computation at its verified scope, never an
// allocation to acquisitions. SourceLine identifies the supplied record. Private
// vendor message IDs are intentionally absent from this representation.
type ModelUsage struct {
	Scope       string
	SourceLine  int
	Counters    []UsageCounter
	Unavailable string
}
