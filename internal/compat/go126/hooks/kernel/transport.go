//go:build go1.26 && !go1.27

// Package kernel contains translated Go 1.26 adapters, never concrete providers.
package kernel

import (
	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/control"
	kernel "github.com/neria-cloud/gosim/internal/kernel"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type Request interface{ Invocation() kernel.Invocation }

func Call[T any](request Request, decode func(kernel.Result) (T, error)) (T, error) {
	inv := request.Invocation()
	if err := control.Submit(inv.Route, &inv); err != nil {
		var zero T
		return zero, err
	}
	return decode(inv.TakeResult())
}

// Require runs before optional-domain adapters dereference application pointers
// or return local validation errors. Ignoring an error cannot bypass this gate.
func Require(feature contracts.Feature, operation string) {
	_, err := Call(kernel.RequireRequest{Feature: feature, Operation: operation}, kernel.DecodeRequireResult)
	if err != nil {
		Reject(operation)
	}
}

func Reject(operation string) {
	ref := control.CurrentRef()
	token := simrt.CurrentMachineToken()
	simrt.Abort(
		abi.Diagnostic{
			Kind:      string(contracts.UnsupportedOperation),
			Feature:   "rt",
			Operation: operation,
			Message:   "operation is not modeled by the Go 1.26 profile",
			RunID:     ref.RunID,
			MachineID: ref.ID,
			Epoch:     token.Epoch,
		},
	)
}

func checked[T any](value T, err error) T {
	if err != nil {
		Reject("invalid kernel adapter response")
	}
	return value
}
