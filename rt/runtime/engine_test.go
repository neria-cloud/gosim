package gosimruntime

import (
	"errors"
	"math"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := NewEngine(EngineConfig{Seed: 42, TimeoutNS: int64(time.Second), ReplayRevision: ReplayRevision})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

func TestEnginePreparePublishCancelDispose(t *testing.T) {
	e := testEngine(t)
	var booted, deferred, fired atomic.Bool
	var oldCallback func(*Timer)
	err := e.Run(func() {
		token, err := e.PrepareMachine(1, 1, "first")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.PrepareMachine(1, 2, "collision"); !errors.Is(err, ErrMachineToken) {
			t.Errorf("duplicate machine accepted: %v", err)
		}
		Yield()
		if booted.Load() {
			t.Error("prepared machine executed application init")
		}
		e.AbortMachine(token)
		e.AbortMachine(token)
		if _, err := e.PrepareMachine(1, 1, "reused epoch"); !errors.Is(err, ErrMachineToken) {
			t.Errorf("reused aborted epoch: %v", err)
		}
		token, err = e.PrepareMachine(1, 2, "retry")
		if err != nil {
			t.Fatal(err)
		}
		ready, block := NewChan[struct{}](0), NewChan[struct{}](0)
		e.PublishMachine(token, func() {
			booted.Store(true)
			defer deferred.Store(true)
			ready.Send(struct{}{})
			block.Recv()
		})
		if booted.Load() {
			t.Error("publication ran boot inside barrier")
		}
		ready.Recv()
		id, err := e.Schedule(token, Nanotime()+100, func() { fired.Store(true) })
		if err != nil || id == 0 {
			t.Fatalf("schedule: %d %v", id, err)
		}
		_ = e.onSupervisor(func() { oldCallback = e.events[id].timer.handler })
		e.CancelMachine(token)
		e.CancelMachine(token)
		_ = e.onSupervisor(func() {
			if m := e.lookup(token); m == nil || m.machine.globals == nil || !m.canceled || len(m.retired) != 1 {
				t.Error("cancellation disposed borrowed globals")
			}
		})
		if _, err := e.Schedule(token, Nanotime(), func() {}); !errors.Is(err, ErrMachineToken) {
			t.Errorf("canceled epoch scheduled work: %v", err)
		}
		e.DisposeMachine(token)
		e.DisposeMachine(token)
		restarted, err := e.PrepareMachine(1, 3, "restart")
		if err != nil {
			t.Fatal(err)
		}
		e.PublishMachine(restarted, func() {})
		_ = e.onSupervisor(func() { oldCallback(nil) })
		e.CancelMachine(token) // old token must not cancel the new incarnation
		e.CancelMachine(restarted)
		e.DisposeMachine(restarted)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !booted.Load() || deferred.Load() || fired.Load() {
		t.Fatalf("boot=%t defer=%t stale event=%t", booted.Load(), deferred.Load(), fired.Load())
	}
	if len(e.scheduler.goroutines) != 0 || len(e.events) != 0 || len(e.machines) != 0 {
		t.Fatal("runtime resources survived disposal")
	}
}

func TestEngineAbortCannotBeRecovered(t *testing.T) {
	e := testEngine(t)
	var after, deferRan atomic.Bool
	var supervisorCleaned atomic.Bool
	err := e.Run(func() {
		token, err := e.PrepareMachine(1, 1, "abort")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			e.CancelMachine(token)
			e.DisposeMachine(token)
			supervisorCleaned.Store(true)
		}()
		e.PublishMachine(token, func() {
			defer func() { _ = recover(); deferRan.Store(true) }()
			e.Abort(abi.Diagnostic{Kind: "FeatureDisabled", Feature: "net", Operation: "socket", MachineID: 1, Epoch: 1})
			after.Store(true)
		})
		NewChan[struct{}](0).Recv()
	})
	var abort *AbortError
	if !errors.As(err, &abort) || abort.Diagnostic.Operation != "socket" || after.Load() || deferRan.Load() {
		t.Fatalf("abort escaped: err=%v after=%t defer=%t", err, after.Load(), deferRan.Load())
	}
	if len(e.scheduler.goroutines) != 0 {
		t.Fatal("abort left suspended goroutines")
	}
	if !supervisorCleaned.Load() || len(e.machines) != 0 {
		t.Fatal("run abort prevented trusted harness cleanup")
	}
}

func TestEngineEventsTimeoutAndSingleActiveRun(t *testing.T) {
	e, inner := testEngine(t), testEngine(t)
	var fired atomic.Int32
	if err := e.Run(func() {
		if err := inner.Run(func() {}); !errors.Is(err, ErrEngineBusy) {
			t.Errorf("nested run accepted: %v", err)
		}
		token, err := e.PrepareMachine(1, 1, "events")
		if err != nil {
			t.Fatal(err)
		}
		e.PublishMachine(token, func() {})
		id, err := e.Schedule(token, Nanotime()+100, func() { fired.Add(10) })
		if err != nil {
			t.Fatal(err)
		}
		e.CancelEvent(id)
		e.CancelEvent(id)
		if _, err := e.Schedule(token, Nanotime()+200, func() {
			if globalPtr.get() != e.supervisor.globals {
				t.Error("provider event ran with application globals")
			}
			fired.Add(1)
		}); err != nil {
			t.Fatal(err)
		}
	}); err != nil || fired.Load() != 1 {
		t.Fatalf("events after boot return: err=%v fired=%d", err, fired.Load())
	}
	if err := e.Run(func() {}); !errors.Is(err, ErrEngineState) {
		t.Fatalf("reused one-shot engine: %v", err)
	}
	if err := inner.Run(func() { NewChan[struct{}](0).Recv() }); !errors.Is(err, ErrTimeout) {
		t.Fatalf("blocked application did not time out: %v", err)
	}
	if err := inner.Close(); !errors.Is(err, ErrTimeout) {
		t.Fatal(err)
	}
	if err := inner.Close(); !errors.Is(err, ErrTimeout) {
		t.Fatal("close did not preserve terminal error")
	}
}

func TestEngineFailureCleanup(t *testing.T) {
	for _, mode := range []string{"panic", "callback-panic", "deadlock", "double-publish", "live-dispose"} {
		t.Run(mode, func(t *testing.T) {
			e := testEngine(t)
			err := e.Run(func() {
				switch mode {
				case "panic":
					panic("test panic")
				case "deadlock":
					_ = e.onSupervisor(func() { e.scheduler.clock.heap = newTimerHeap() })
					NewChan[struct{}](0).Recv()
				default:
					token, err := e.PrepareMachine(1, 1, "invalid")
					if err != nil {
						t.Fatal(err)
					}
					e.PublishMachine(token, func() {})
					if mode == "callback-panic" {
						if _, err := e.Schedule(token, Nanotime()+1, func() { panic("provider invariant") }); err != nil {
							t.Fatal(err)
						}
					} else if mode == "double-publish" {
						e.PublishMachine(token, func() {})
					} else {
						e.DisposeMachine(token)
					}
				}
			})
			if err == nil || len(e.scheduler.goroutines) != 0 || len(e.scheduler.runnable) != 0 {
				t.Fatalf("failure leaked runtime state: %v", err)
			}
			if mode == "deadlock" && !strings.Contains(err.Error(), "all goroutines blocked") {
				t.Fatal(err)
			}
		})
	}
}

func TestEngineRejectsInvalidConfigurationAndExhaustedIDs(t *testing.T) {
	for _, cfg := range []EngineConfig{{}, {TimeoutNS: 1}, {TimeoutNS: -1, ReplayRevision: ReplayRevision}} {
		if _, err := NewEngine(cfg); err == nil {
			t.Fatalf("accepted %+v", cfg)
		}
	}
	e := testEngine(t)
	if _, err := e.PrepareMachine(1, 1, "outside run"); !errors.Is(err, ErrEngineState) {
		t.Fatal(err)
	}
	if err := e.Run(func() {
		if _, err := e.PrepareMachine(0, 1, "zero"); !errors.Is(err, ErrMachineToken) {
			t.Fatal(err)
		}
		token, err := e.PrepareMachine(1, math.MaxUint64, "last epoch")
		if err != nil {
			t.Fatal(err)
		}
		e.PublishMachine(token, func() {})
		_ = e.onSupervisor(func() { e.nextEvent = math.MaxUint64 })
		if _, err := e.Schedule(token, Nanotime()+1, func() {}); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Schedule(token, Nanotime()+1, func() {}); !errors.Is(err, ErrEventExhausted) {
			t.Fatalf("event ID wrapped: %v", err)
		}
		e.CancelMachine(token)
		e.DisposeMachine(token)
		if _, err := e.PrepareMachine(1, 1, "wrapped"); !errors.Is(err, ErrMachineToken) {
			t.Fatalf("epoch wrapped: %v", err)
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func TestEngineCleanupFailurePreservesOriginalFailure(t *testing.T) {
	e := testEngine(t)
	var completed atomic.Bool
	err := e.Run(func() {
		defer func() {
			e.Abort(abi.Diagnostic{Kind: "CleanupFailure", Message: "cleanup error"})
			completed.Store(true)
		}()
		e.Abort(abi.Diagnostic{Kind: "OriginalFailure", Message: "original error"})
	})
	if err == nil || !strings.Contains(err.Error(), "original error") || !strings.Contains(err.Error(), "cleanup error") || !completed.Load() {
		t.Fatalf("incomplete cleanup diagnostics: %v completed=%t", err, completed.Load())
	}
}
