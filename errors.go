package gosim

import (
	"fmt"

	"github.com/neria-cloud/gosim/internal/contracts"
)

var (
	ErrNotSim          = contracts.ErrNotSim
	ErrWrongRun        = contracts.ErrWrongRun
	ErrMachineNotFound = contracts.ErrMachineNotFound
	ErrInvalidState    = contracts.ErrInvalidState
	ErrNotStarted      = contracts.ErrNotStarted
	ErrSelfTransition  = contracts.ErrSelfTransition
	ErrInvalidConfig   = contracts.ErrInvalidConfig
)

// FeatureDisabledError describes a run-level failure. Disabled operations abort
// the simulation; they cannot be ignored or recovered by application code.
type FeatureDisabledError struct {
	Feature, Operation, Caller string
	MachineID, Epoch           uint64
}

func (e *FeatureDisabledError) Error() string {
	return fmt.Sprintf(
		"FeatureDisabled: feature=%s operation=%s caller=%s machine=%d epoch=%d",
		e.Feature,
		e.Operation,
		e.Caller,
		e.MachineID,
		e.Epoch,
	)
}
