package gosimruntime

import (
	"math"
	"strings"

	"github.com/neria-cloud/gosim/internal/hostabi"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// SyscallEnter/Exit suppress synthetic synchronization from the transport.
// Actual application I/O remains visible through the existing memory-view ABI.
func SyscallEnter() { hostabi.RaceDisable() }
func SyscallExit()  { hostabi.RaceEnable() }

// WakeSemaphore is the non-yielding completion half used by the supervisor
// and native event callbacks. Semacquire remains the application wait half.
//
//go:norace
func (e *Engine) WakeSemaphore(view abi.ValueView[uint32]) {
	if err := e.onSupervisor(func() {
		addr := (*uint32)(view.UnsafePointer())
		sema := getsema(addr, false)
		if sema == nil {
			if view.Get() == math.MaxUint32 {
				panic("completion semaphore overflow")
			}
			view.Set(view.Get() + 1)
			return
		}
		sema.q.dequeue()
		if sema.q.first == nil {
			freesema(addr)
		}
	}); err != nil {
		e.invariant("completion outside active engine")
	}
}

// RetireSemaphore is called after its epoch's coroutines have been canceled.
// Empty native wait records must not keep invocation memory alive.
//
//go:norace
func (e *Engine) RetireSemaphore(view abi.ValueView[uint32]) {
	if err := e.onSupervisor(func() {
		addr := (*uint32)(view.UnsafePointer())
		if sema := getsema(addr, false); sema != nil {
			panic("retiring semaphore with live waiters")
		}
		view.Set(0)
	}); err != nil {
		e.invariant("retire semaphore outside active engine")
	}
}

// WriteDiagnostic emits through the declared host log sink, never a host fd.
//
//go:norace
func (e *Engine) WriteDiagnostic(token MachineToken, goroutineID uint64, fd int, data abi.ByteView) {
	if err := e.onSupervisor(func() {
		m := e.lookup(token)
		if m == nil || m.canceled || (fd != 1 && fd != 2) {
			panic("invalid diagnostic output")
		}
		owned := make([]byte, data.Len())
		data.Read(owned)
		method := "stdout"
		if fd == 2 {
			method = "stderr"
		}
		e.scheduler.logger.slog.Info(strings.TrimSuffix(string(owned), "\n"), "method", method, "machine", m.machine.label, "goroutine", goroutineID)
	}); err != nil {
		e.invariant("diagnostic output outside active engine")
	}
}
