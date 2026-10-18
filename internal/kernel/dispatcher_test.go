package kernel

import (
	"errors"
	"math"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func currentContext(k *Kernel) contracts.CallContext {
	token := simrt.CurrentMachineToken()
	return contracts.CallContext{Epoch: contracts.EpochRef{Machine: contracts.MachineRef{RunID: k.runID, ID: token.ID}, Epoch: token.Epoch}}
}

func TestDispatcherRealApplicationRoundTripAndEventWake(t *testing.T) {
	e := engineFor(t)
	var completed atomic.Bool
	var k *Kernel
	err := e.Run(func() {
		var err error
		k, err = New(configFor(contracts.FeatureSet(contracts.RT)), e, nil)
		if err != nil {
			t.Fatal(err)
		}
		done := simrt.NewChan[bool](0)
		var fd int
		ref := createFor(t, k, "round-trip", func() {
			call := func(route RouteID, scalars [6]uintptr, args [5]any) Result {
				inv := &Invocation{Context: currentContext(k), Route: route, Scalars: scalars, Args: args}
				if err := k.dispatcher.Submit(inv); err != nil {
					t.Fatal(err)
				}
				if inv.Args != ([5]any{}) {
					t.Fatal("completion retained borrowed arguments")
				}
				result := inv.TakeResult()
				if inv.Result != (Result{}) {
					t.Fatal("result retained after extraction")
				}
				return result
			}
			if result := call(RouteGetpid, [6]uintptr{}, [5]any{}); result.R0 != 42 || result.Errno != 0 {
				t.Fatal(result)
			}
			bytes := []byte("hi\n")
			if result := call(RouteWrite, [6]uintptr{1}, [5]any{abi.ByteView{Ptr: bytes}}); result.R0 != 3 || result.Errno != 0 {
				t.Fatal(result)
			}
			if result := call(RouteRead, [6]uintptr{999}, [5]any{abi.ByteView{Ptr: bytes}}); result.Errno != uintptr(syscall.EBADF) {
				t.Fatal("unknown fd was not EBADF", result)
			}
			poll := call(RoutePollOpen, [6]uintptr{uintptr(fd)}, [5]any{})
			if poll.R0 != 0 || poll.R1 == 0 {
				t.Fatal("poll open", poll)
			}
			if result := call(RoutePollWait, [6]uintptr{poll.R1, 'r'}, [5]any{}); result.R0 != 0 || result.Errno != 0 {
				t.Fatal("event wake", result)
			}
			if result := call(RoutePollClose, [6]uintptr{poll.R1}, [5]any{}); result.R0 != 0 {
				t.Fatal(result)
			}
			completed.Store(true)
			done.Send(true)
		})
		tx, err := k.BeginStart(ref, false)
		if err != nil {
			t.Fatal(err)
		}
		r := &dataResource{owner: contracts.RT}
		fd, err = tx.run.descriptors.Allocate(r)
		if err != nil {
			t.Fatal(err)
		}
		r.fd = fd
		r.onSubscribe = func(ctx contracts.CallContext, sink contracts.ReadinessSink) {
			if _, err := k.Ports().Events.Schedule(ctx.Epoch, k.Ports().Clock.NowNanos()+100, func() { sink.Notify(ctx.Epoch, fd, contracts.ReadReady) }); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if err := k.Run(func() { done.Recv() }); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || !completed.Load() {
		t.Fatal("application dispatch failed", err)
	}
	if len(k.dispatcher.live) != 0 || len(k.dispatcher.envelopes) != 0 || len(k.polls) != 0 {
		t.Fatal("run retained invocation ownership")
	}
}

func TestDispatcherWaitWakesSurvivingRoot(t *testing.T) {
	e := engineFor(t)
	var completed atomic.Bool
	err := e.Run(func() {
		k, err := New(configFor(contracts.FeatureSet(contracts.RT)), e, nil)
		if err != nil {
			t.Fatal(err)
		}
		ref := createFor(t, k, "waited", func() { simrt.Sleep(500) })
		if err := k.Start(ref); err != nil {
			t.Fatal(err)
		}
		if err := k.Run(func() {
			inv := &Invocation{Context: currentContext(k), Route: RouteWait, Scalars: [6]uintptr{uintptr(ref.RunID), uintptr(ref.ID)}}
			if err := k.dispatcher.Submit(inv); err != nil {
				t.Fatal(err)
			}
			exit := takeExit(inv)
			if !exit.Valid || exit.Epoch != 1 || exit.Kind != contracts.StopGraceful {
				t.Fatal("wrong captured exit", exit)
			}
			completed.Store(true)
		}); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || !completed.Load() {
		t.Fatal("wait did not complete", err)
	}
}

// Generated result decoders copy typed values before exposing them to callers.
//
//go:norace
func takeExit(inv *Invocation) ExitRecord { return inv.TakeResult().Value.(ExitRecord) }

func TestCanceledInvocationDropsMemoryBeforeLateCompletion(t *testing.T) {
	_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
		ref := createFor(t, k, "borrower", idleMain)
		if err := k.Start(ref); err != nil {
			t.Fatal(err)
		}
		run := k.machineByID[ref.ID].current
		data := []byte("unchanged")
		inv := &Invocation{Context: contracts.CallContext{Epoch: run.ref}, Route: RouteRead, Args: [5]any{abi.ByteView{Ptr: data}}, Result: Result{Value: &data}}
		if err := k.dispatcher.admit(inv); err != nil {
			t.Fatal(err)
		}
		id := inv.Context.InvocationID
		lease := k.dispatcher.live[id]
		k.engine.CancelMachine(token(run))
		k.dispatcher.CancelEpoch(run.ref)
		if inv.Args != ([5]any{}) || inv.Result != (Result{}) || lease.invocation != nil || inv.Sema != 0 {
			t.Fatal("cancellation retained borrowed invocation memory")
		}
		if err := k.dispatcher.Complete(id, Result{Value: abi.ByteView{Ptr: data}, R0: 9}); err != nil {
			t.Fatal(err)
		}
		if string(data) != "unchanged" || inv.Result != (Result{}) || inv.Sema != 0 {
			t.Fatal("late completion touched canceled envelope")
		}
		if len(k.traces) == 0 || k.traces[len(k.traces)-1].event != "discard-canceled-completion" {
			t.Fatal("stale completion was not traced")
		}
		if err := k.Stop(ref, contracts.StopCrash); err != nil {
			t.Fatal(err)
		}
		if len(k.dispatcher.canceled) != 0 {
			t.Fatal("drained epoch retained cancellation metadata")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDispatcherRejectsDuplicateCompletionUnknownRoutesAndPayloads(t *testing.T) {
	for _, failure := range []string{"duplicate", "unknown", "payload", "exhaustion", "mode", "errno"} {
		t.Run(failure, func(t *testing.T) {
			_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
				ref := createFor(t, k, "invalid", idleMain)
				if err := k.Start(ref); err != nil {
					t.Fatal(err)
				}
				ctx := contracts.CallContext{Epoch: k.machineByID[ref.ID].current.ref}
				inv := &Invocation{Context: ctx, Route: RouteGetpid}
				switch failure {
				case "duplicate":
					k.dispatcher.DispatchOne(inv)
					_ = k.dispatcher.Complete(inv.Context.InvocationID, Result{})
				case "unknown":
					inv.Route = RouteID(math.MaxUint32)
					k.dispatcher.DispatchOne(inv)
				case "payload":
					inv.Args[0] = func() { t.Error("arbitrary callback executed") }
					k.dispatcher.DispatchOne(inv)
				case "exhaustion":
					k.dispatcher.nextID = math.MaxUint64
					k.dispatcher.DispatchOne(inv)
					k.dispatcher.DispatchOne(&Invocation{Context: ctx, Route: RouteGetpid})
				case "mode":
					inv.Route, inv.Scalars[1] = RoutePollWait, 256+'r'
					k.dispatcher.DispatchOne(inv)
				case "errno":
					r := &dataResource{owner: contracts.RT, readErr: errors.Join(syscall.EINVAL, errors.New("provider invariant"))}
					fd, err := k.machineByID[ref.ID].current.descriptors.Allocate(r)
					if err != nil {
						t.Fatal(err)
					}
					inv.Route, inv.Scalars[0], inv.Args[0] = RouteRead, uintptr(fd), abi.ByteView{}
					k.dispatcher.DispatchOne(inv)
				}
				t.Error("invalid dispatcher operation returned to application")
			})
			if err == nil {
				t.Fatal("invalid dispatcher operation succeeded")
			}
			want := map[string]string{"duplicate": "duplicate completion", "unknown": "UnsupportedOperation", "payload": "InvalidABI", "exhaustion": "exhausted", "mode": "InvalidABI", "errno": "unmodeled syscall error"}[failure]
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("wrong failure: %v", err)
			}
		})
	}
}

func TestDispatcherRejectsWrongCaller(t *testing.T) {
	e := engineFor(t)
	if err := e.Run(func() {
		k, err := New(configFor(contracts.FeatureSet(contracts.RT)), e, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := k.Run(func() {
			ctx := currentContext(k)
			ctx.Epoch.Epoch++
			if err := k.dispatcher.Submit(&Invocation{Context: ctx, Route: RouteGetpid}); !errors.Is(err, contracts.ErrInvalidState) {
				t.Error("wrong epoch admitted", err)
			}
		}); err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCrashCancelsRealBlockedPollWithoutWake(t *testing.T) {
	e := engineFor(t)
	var resumed, deferred, crashed atomic.Bool
	var k *Kernel
	err := e.Run(func() {
		var err error
		k, err = New(configFor(contracts.FeatureSet(contracts.RT)), e, nil)
		if err != nil {
			t.Fatal(err)
		}
		var fd int
		ref := createFor(t, k, "blocked", func() {
			defer deferred.Store(true)
			inv := &Invocation{Context: currentContext(k), Route: RoutePollOpen, Scalars: [6]uintptr{uintptr(fd)}}
			if err := k.dispatcher.Submit(inv); err != nil {
				t.Fatal(err)
			}
			id := inv.TakeResult().R1
			inv.Route, inv.Scalars = RoutePollWait, [6]uintptr{id, 'r'}
			if err := k.dispatcher.Submit(inv); err != nil {
				t.Fatal(err)
			}
			resumed.Store(true)
		})
		tx, err := k.BeginStart(ref, false)
		if err != nil {
			t.Fatal(err)
		}
		r := &dataResource{owner: contracts.RT}
		fd, err = tx.run.descriptors.Allocate(r)
		if err != nil {
			t.Fatal(err)
		}
		r.fd = fd
		r.onSubscribe = func(ctx contracts.CallContext, _ contracts.ReadinessSink) {
			if _, err := k.Ports().Events.Schedule(ctx.Epoch, k.Ports().Clock.NowNanos()+100, func() {
				if len(k.dispatcher.live) != 1 {
					t.Error("fixture did not block in real poll")
				}
				if err := k.Stop(ref, contracts.StopCrash); err != nil {
					t.Error(err)
				}
				crashed.Store(true)
			}); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if err := k.Run(func() { simrt.Sleep(500) }); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || !crashed.Load() || resumed.Load() || deferred.Load() {
		t.Fatalf("blocked crash: %v crash=%t resumed=%t deferred=%t", err, crashed.Load(), resumed.Load(), deferred.Load())
	}
	if len(k.dispatcher.live) != 0 || len(k.dispatcher.canceled) != 0 || len(k.polls) != 0 {
		t.Fatal("blocked crash retained metadata")
	}
}
