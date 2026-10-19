package gosimruntime

import (
	"errors"
	"sync/atomic"
	"testing"
)

func TestMachineInitializerPublicationAndRestart(t *testing.T) {
	e := testEngine(t)
	var initialized atomic.Int32
	var booted atomic.Int32
	if err := e.SetMachineInitializer(func() { initialized.Add(1) }); err != nil {
		t.Fatal(err)
	}
	if err := e.SetMachineInitializer(func() {}); !errors.Is(err, ErrEngineState) {
		t.Fatal("initializer replaced", err)
	}
	if err := e.Run(func() {
		if err := e.SetMachineInitializer(func() {}); !errors.Is(err, ErrEngineState) {
			t.Fatal("active initializer changed", err)
		}
		abandoned, err := e.PrepareMachine(2, 1, "abandoned")
		if err != nil {
			t.Fatal(err)
		}
		e.AbortMachine(abandoned)
		for epoch := uint64(1); epoch <= 2; epoch++ {
			token, err := e.PrepareMachine(1, epoch, "machine")
			if err != nil {
				t.Fatal(err)
			}
			if got := initialized.Load(); got != int32(epoch-1) {
				t.Fatal("prepare ran initializer", got)
			}
			done := NewChan[struct{}](0)
			e.PublishMachine(token, func() {
				if initialized.Load() != int32(epoch) {
					panic("main preceded initializer")
				}
				booted.Add(1)
				done.Send(struct{}{})
			})
			if got := initialized.Load(); got != int32(epoch-1) {
				t.Fatal("publish ran initializer inside barrier", got)
			}
			done.Recv()
			e.CancelMachine(token)
			e.DisposeMachine(token)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if initialized.Load() != 2 || booted.Load() != 2 {
		t.Fatal("startup count", initialized.Load(), booted.Load())
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if e.machineInitializer != nil {
		t.Fatal("closed engine retained initializer")
	}
	if err := e.SetMachineInitializer(func() {}); !errors.Is(err, ErrEngineState) {
		t.Fatal("closed initializer accepted", err)
	}
}

func TestInfrastructureInitializationPerEngine(t *testing.T) {
	var calls atomic.Int32
	info := &Globals{Infrastructure: true}
	for run := int32(1); run <= 2; run++ {
		e := testEngine(t)
		info.Initializers = func(shared bool) {
			var guarded bool
			func() { defer func() { guarded = recover() != nil }(); checkTransition() }()
			if shared || !guarded {
				panic("infrastructure initialization permits yielding or shared initialization")
			}
			calls.Add(1)
			e.initializeInfrastructure(info) // writes during initialization reenter safely
		}
		if err := e.Run(func() {
			e.initializeInfrastructure(info)
			e.initializeInfrastructure(info)
			checkTransition()
			var appRan bool
			func() {
				defer func() {
					if recover() == nil {
						panic("supervisor accepted application globals")
					}
				}()
				e.initializeInfrastructure(&Globals{Initializers: func(bool) { appRan = true }})
			}()
			if appRan {
				panic("application initializer executed")
			}
		}); err != nil {
			t.Fatal(err)
		}
		if calls.Load() != run {
			t.Fatal("not initialized once per engine", calls.Load())
		}
		if err := e.Close(); err != nil {
			t.Fatal(err)
		}
		if e.initializedGlobals != nil {
			t.Fatal("closed engine retained package state")
		}
	}
}
