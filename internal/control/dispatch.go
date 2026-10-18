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
// TODO(migration): Task 21 applies the selected profile's SharedGlobals record.
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
	ref := CurrentRef()
	if ref.ID == 0 {
		return contracts.ErrNotSim
	}
	if inv == nil || inv.Route != route {
		return contracts.ErrInvalidConfig
	}
	token := simrt.CurrentMachineToken()
	var pc [1]uintptr
	runtime.Callers(2, pc[:])
	inv.Context = contracts.CallContext{Epoch: contracts.EpochRef{Machine: ref, Epoch: token.Epoch}, PC: pc[0]}
	return binding.dispatcher.Submit(inv)
}
