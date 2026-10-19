package gosimruntime

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"runtime"
	"strings"
	"time"

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
		e.writeCallerRecord(m.machine.label, goroutineID, 0, strings.TrimSuffix(string(owned), "\n"), slog.String("method", method))
	}); err != nil {
		e.invariant("diagnostic output outside active engine")
	}
}

// Abort fails the current simulation from a translated version adapter. Only
// primitive diagnostic fields cross the native boundary; application recovery
// cannot turn a fatal runtime condition into a successful run.
//
//go:norace
func Abort(diagnostic abi.Diagnostic) {
	var pcs [32]uintptr
	n := runtime.Callers(2, pcs[:])
	if diagnostic.PC == 0 && n > 0 {
		diagnostic.PC = pcs[0]
	}
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		diagnostic.Message += fmt.Sprintf("\n  %s (%s:%d)", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	g := getg()
	g.upcall(func() {
		scheduler := gs.get()
		if scheduler.abortErr == nil {
			scheduler.abortErr = &AbortError{Diagnostic: diagnostic}
		}
	})
	g.park(false)
}

// WriteTrace accepts only copied diagnostic values, never application payloads.
//
//go:norace
func (e *Engine) WriteTrace(token MachineToken, goroutineID uint64, pc uintptr, message string) {
	if err := e.onSupervisor(func() {
		m := e.lookup(token)
		if m == nil {
			panic("trace for unknown machine")
		}
		e.writeCallerRecord(m.machine.label, goroutineID, pc, message)
	}); err != nil {
		e.invariant("trace outside active engine")
	}
}

//go:norace
func (e *Engine) writeCallerRecord(label string, goroutineID uint64, pc uintptr, message string, attrs ...slog.Attr) {
	// The supervisor is the execution owner, but the originating application
	// coroutine is the diagnostic subject. Avoid the supervisor's implicit attrs.
	record := slog.NewRecord(time.Unix(0, Nanotime()).UTC(), slog.LevelInfo, message, pc)
	record.AddAttrs(slog.String("machine", label), slog.Uint64("goroutine", goroutineID), slog.Int("step", Step()))
	record.AddAttrs(attrs...)
	handler := slog.NewJSONHandler(e.scheduler.logger.out, &slog.HandlerOptions{AddSource: true})
	_ = handler.Handle(context.Background(), record)
}

// WriteProviderTrace accepts copied string fields from a translated provider.
//
//go:norace
func (e *Engine) WriteProviderTrace(token MachineToken, goroutineID uint64, pc uintptr, event string, fields []string) {
	// Metadata belongs to the translated supervisor. Copy it before the native
	// upcall, whose scheduler synchronization is deliberately race-invisible.
	// This does not read or suppress instrumentation on application buffers.
	owned := make([]string, len(fields))
	for i := range fields {
		owned[i] = fields[i]
	}
	if err := e.onSupervisor(func() {
		label := "supervisor"
		if m := e.lookup(token); m != nil {
			label = m.machine.label
		}
		attrs := make([]slog.Attr, 0, len(owned)/2)
		for i := 0; i+1 < len(owned); i += 2 {
			attrs = append(attrs, slog.String(owned[i], owned[i+1]))
		}
		e.writeCallerRecord(label, goroutineID, pc, event, attrs...)
	}); err != nil {
		e.invariant("provider trace outside active engine")
	}
}
