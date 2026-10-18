package contracts

import (
	"errors"
	"fmt"
)

var (
	ErrNotSim           = errors.New("not running in a simulation")
	ErrWrongRun         = errors.New("handle belongs to another simulation run")
	ErrMachineNotFound  = errors.New("machine not found or destroyed")
	ErrInvalidState     = errors.New("invalid machine state")
	ErrNotStarted       = errors.New("machine has not started")
	ErrSelfTransition   = errors.New("returning self transition is not permitted")
	ErrInvalidConfig    = errors.New("invalid simulator configuration")
	ErrEnumerationLimit = errors.New("crash-state enumeration limit reached")
)

type FailureCode string

const (
	FeatureDisabled      FailureCode = "FeatureDisabled"
	UnsupportedOperation FailureCode = "UnsupportedOperation"
	InvalidABI           FailureCode = "InvalidABI"
	InvariantFailure     FailureCode = "InvariantFailure"
)

// Failure describes a simulator failure. Adapters must abort the run for these
// codes; they must never turn them into an ignorable Linux errno.
type Failure struct {
	Code       FailureCode
	Diagnostic Diagnostic
}

func (failure *Failure) Error() string {
	d := failure.Diagnostic
	return fmt.Sprintf("%s: feature=%s operation=%s machine=%d epoch=%d pc=%#x: %s", failure.Code, d.Feature, d.Operation, d.Epoch.Machine.ID, d.Epoch.Epoch, d.PC, d.Message)
}
