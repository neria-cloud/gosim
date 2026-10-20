package contracts

import (
	"fmt"
)

// Sentinel identifies a domain failure by value across isolated machine and
// supervisor globals. Pointer-based errors.New values would differ per machine.
type Sentinel string

func (e Sentinel) Error() string { return string(e) }

var (
	ErrNotSim           error = Sentinel("not running in a simulation")
	ErrWrongRun         error = Sentinel("handle belongs to another simulation run")
	ErrMachineNotFound  error = Sentinel("machine not found or destroyed")
	ErrInvalidState     error = Sentinel("invalid machine state")
	ErrNotStarted       error = Sentinel("machine has not started")
	ErrSelfTransition   error = Sentinel("returning self transition is not permitted")
	ErrInvalidConfig    error = Sentinel("invalid simulator configuration")
	ErrEnumerationLimit error = Sentinel("crash-state enumeration limit reached")
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
	return fmt.Sprintf(
		"%s: feature=%s operation=%s machine=%d epoch=%d pc=%#x: %s",
		failure.Code,
		d.Feature,
		d.Operation,
		d.Epoch.Machine.ID,
		d.Epoch.Epoch,
		d.PC,
		d.Message,
	)
}

// CrashRequest is an internal provider outcome consumed by the kernel before
// syscall completion. It is never an application errno or a recoverable panic.
type CrashRequest struct {
	Epoch     EpochRef
	Operation string
}

func (r *CrashRequest) Error() string { return "injected machine crash during " + r.Operation }
