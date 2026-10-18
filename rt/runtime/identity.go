package gosimruntime

import (
	"errors"
	"math"
	"sync/atomic"
)

// Process identity must remain native: translated globals are reset for each
// machine, but a handle from an earlier run must never become valid again.
var processRunID atomic.Uint64

// The scheduler and coroutine stacks are serialized, but intentionally have
// no race happens-before edge for application memory. This is native metadata.
//
//go:norace
func checkTransition() {
	if gs.get().transitionDepth != 0 {
		panic("yield or wait inside supervisor transition")
	}
}

// Only supervisor state has this serialization edge. Application goroutines
// deliberately do not acquire it, preserving the simulated race oracle.
//
//go:norace
func (g *goroutine) releaseSupervisor() {
	if e := gs.get().engine; e != nil && g.machine == e.supervisor {
		e.supervisorToken.Release()
	}
}

//go:norace
func (g *goroutine) acquireSupervisor() {
	if e := gs.get().engine; e != nil && g.machine == e.supervisor {
		e.supervisorToken.Acquire()
	}
}

func NewRunID() (uint64, error) { return reserveRunID(&processRunID) }

func reserveRunID(counter *atomic.Uint64) (uint64, error) {
	for {
		previous := counter.Load()
		if previous == math.MaxUint64 {
			return 0, errors.New("simulation run ID space exhausted")
		}
		if counter.CompareAndSwap(previous, previous+1) {
			return previous + 1, nil
		}
	}
}

// CurrentMachineToken returns zero for the trusted supervisor/event stack.
// Only primitive identities cross into translated lifecycle dispatch.
//
//go:norace
func CurrentMachineToken() MachineToken {
	if gs.get() == nil {
		return MachineToken{}
	}
	if g := getg(); g != nil {
		return g.machine.token
	}
	return MachineToken{}
}

// BeginTransition/EndTransition guard trusted lifecycle operations. Unlike
// controlled nondeterminism they do not suppress a new coroutine's initial
// park, change random results, or permit an accidental voluntary yield.
//
//go:norace
func (e *Engine) BeginTransition() error {
	return e.onSupervisor(func() { e.scheduler.transitionDepth++ })
}

//go:norace
func (e *Engine) EndTransition() {
	if err := e.onSupervisor(func() {
		if e.scheduler.transitionDepth <= 0 {
			panic("unbalanced supervisor transition")
		}
		e.scheduler.transitionDepth--
	}); err != nil {
		e.invariant("transition outside active engine")
	}
}
