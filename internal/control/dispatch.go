// Package control transports translated public commands to the run's kernel.
// It imports no provider, and no caller supplies an execution trampoline.
package control

import (
	"runtime"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/kernel"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

// The selected profile marks binding as shared translated run state. It never
// enters native engine storage. The harness installs it before machine init and
// clears it after teardown; the single-active-run invariant prevents overlap.
var binding struct {
	kernel     *kernel.Kernel
	dispatcher *kernel.Dispatcher
	runID      uint64
}

// Bind is harness-only. Both install and release happen on the surviving
// supervisor; application coroutines cannot change the binding.
//
//go:norace
func Bind(k *kernel.Kernel) {
	if simrt.CurrentMachineToken().ID != 0 {
		panic(contracts.ErrInvalidState)
	}
	if binding.kernel != nil {
		if !binding.kernel.TransportClosed() {
			panic(contracts.ErrInvalidState)
		}
		binding.kernel, binding.dispatcher, binding.runID = nil, nil, 0
	}
	if k == nil {
		return
	}
	id, dispatcher, err := k.TransportBinding()
	if err != nil {
		panic(err)
	}
	binding.kernel, binding.dispatcher, binding.runID = k, dispatcher, id
}

//go:norace
func CurrentRef() contracts.MachineRef {
	token := simrt.CurrentMachineToken()
	if token.ID == 0 || binding.kernel == nil || binding.kernel.TransportClosed() {
		return contracts.MachineRef{}
	}
	return contracts.MachineRef{RunID: binding.runID, ID: token.ID}
}

// Submit stamps the current run/epoch and discards caller-supplied context.
// Dispatcher validates live admission and owns the request until completion.
//
//go:norace
func Submit(route kernel.RouteID, inv *kernel.Invocation) error {
	return submit(route, inv, 3)
}

// Call submits a copied, generated request for a public control method. Capture
// that method's caller rather than the generic transport frame in diagnostics.
func Call[T any](
	request interface{ Invocation() kernel.Invocation }, decode func(kernel.Result) (T, error),
) (T, error) {
	inv := request.Invocation()
	if err := submit(inv.Route, &inv, 4); err != nil {
		var zero T
		return zero, err
	}
	return decode(inv.TakeResult())
}

// Require must precede public-side validation of optional feature arguments.
// The dispatcher aborts disabled operations before returning to application code.
func Require(feature contracts.Feature, operation string) error {
	inv := (kernel.RequireRequest{Feature: feature, Operation: operation}).Invocation()
	if err := submit(inv.Route, &inv, 4); err != nil {
		return err
	}
	_, err := kernel.DecodeRequireResult(inv.TakeResult())
	return err
}

//go:norace
func submit(route kernel.RouteID, inv *kernel.Invocation, callerSkip int) error {
	ref := CurrentRef()
	if ref.ID == 0 {
		return contracts.ErrNotSim
	}
	if inv == nil || inv.Route != route {
		return contracts.ErrInvalidConfig
	}
	token := simrt.CurrentMachineToken()
	var pc [1]uintptr
	runtime.Callers(callerSkip, pc[:])
	inv.Context = contracts.CallContext{Epoch: contracts.EpochRef{Machine: ref, Epoch: token.Epoch}, PC: pc[0]}
	return binding.dispatcher.Submit(inv)
}
