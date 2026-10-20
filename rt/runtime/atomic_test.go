package gosimruntime

import (
	"sync/atomic"
	"testing"
)

func TestAtomicSchedulingRespectsNestedProcPin(t *testing.T) {
	e := testEngine(t)
	var progressed atomic.Bool
	if err := e.Run(func() {
		start := NewChan[struct{}](0)
		Go(func() { start.Recv(); progressed.Store(true) })
		start.Close()
		if err := e.BeginTransition(); err != nil {
			t.Fatal(err)
		}
		for range 100 {
			AtomicYield()
		}
		if progressed.Load() {
			t.Fatal("implicit atomic preemption split a supervisor transition")
		}
		e.EndTransition()
		if ProcPin() != 0 || ProcPin() != 0 {
			t.Fatal("unexpected logical processor")
		}
		for range 100 {
			AtomicYield()
		}
		if progressed.Load() {
			t.Fatal("atomic operation yielded while pinned")
		}
		ProcUnpin()
		for range 100 {
			AtomicYield()
		}
		if progressed.Load() {
			t.Fatal("nested pin released too early")
		}
		ProcUnpin()
		for range 10000 {
			AtomicYield()
			if progressed.Load() {
				return
			}
		}
		t.Fatal("atomic operation did not yield after unpinning")
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAtomicSchedulingInEventCallback(t *testing.T) {
	e := testEngine(t)
	fired := false
	if err := e.Run(func() {
		token, err := e.PrepareMachine(1, 1, "atomic callback")
		if err != nil {
			t.Fatal(err)
		}
		e.PublishMachine(token, func() {})
		if _, err := e.Schedule(token, Nanotime()+1, func() {
			if getg() != nil {
				t.Fatal("callback unexpectedly has an application goroutine")
			}
			ProcPin()
			AtomicYield()
			ProcUnpin()
			AtomicYield()
			fired = true
		}); err != nil {
			t.Fatal(err)
		}
		Sleep(2)
	}); err != nil {
		t.Fatal(err)
	}
	if !fired {
		t.Fatal("callback did not execute")
	}
}
