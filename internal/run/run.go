// Package run defines the vendor-independent representation of one task execution.
package run

type ExecutionState string

const (
	Incomplete ExecutionState = "INCOMPLETE"
	Completed  ExecutionState = "COMPLETED"
)

type Run struct {
	Task           string
	FinalResponse  string
	ExecutionState ExecutionState
}
