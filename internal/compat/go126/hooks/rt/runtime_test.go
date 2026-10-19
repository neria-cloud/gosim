//go:build go1.26 && !go1.27

package rt

import (
	"errors"
	"testing"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

func TestSimulatedProcessorQueries(t *testing.T) {
	if Runtime_NumCPU() != 1 {
		t.Fatal("host CPU count escaped")
	}
	for _, n := range []int{-1, 0, 1} {
		if Runtime_GOMAXPROCS(n) != 1 {
			t.Fatalf("GOMAXPROCS(%d)", n)
		}
	}
	// Finalizer registration deliberately retains no callback in the native GC.
	Runtime_SetFinalizer(new(int), func(*int) { t.Error("native finalizer invoked") })
}

func TestParallelProcessorRequestAborts(t *testing.T) {
	e, err := gosimruntime.NewEngine(gosimruntime.EngineConfig{Seed: 1, TimeoutNS: 1e9, ReplayRevision: gosimruntime.ReplayRevision})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	after, deferred := false, false
	err = e.Run(func() {
		token, err := e.PrepareMachine(1, 1, "parallelism")
		if err != nil {
			t.Fatal(err)
		}
		e.PublishMachine(token, func() {
			defer func() { _ = recover(); deferred = true }()
			Runtime_GOMAXPROCS(2)
			after = true
		})
		gosimruntime.NewChan[struct{}](0).Recv()
	})
	var aborted *gosimruntime.AbortError
	if !errors.As(err, &aborted) || aborted.Diagnostic.Operation != "runtime.GOMAXPROCS" || after || deferred {
		t.Fatalf("parallelism err=%v after=%v defer=%v", err, after, deferred)
	}
}
