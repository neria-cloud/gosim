//go:build go1.25 && !go1.26

package runtimebridge

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

func TestGOROOTDoesNotExposeHostInstallation(t *testing.T) {
	t.Setenv("GOROOT", "/host/private/toolchain")
	if got := Runtime_GOROOT(); got != "" {
		t.Fatalf("simulated GOROOT=%q", got)
	}
}

func TestCleanupRegistrationAndStopAbort(t *testing.T) {
	for name, call := range map[string]func(){
		"runtime.AddCleanup":   func() { Runtime_AddCleanup(new(int), func(int) { t.Error("cleanup invoked") }, 0) },
		"runtime.Cleanup.Stop": func() { RuntimeCleanup{}.Stop() },
	} {
		t.Run(name, func(t *testing.T) {
			e, err := gosimruntime.NewEngine(gosimruntime.EngineConfig{Seed: 1, TimeoutNS: 1e9, ReplayRevision: gosimruntime.ReplayRevision})
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			after, deferred := false, false
			err = e.Run(func() {
				token, err := e.PrepareMachine(1, 1, "cleanup")
				if err != nil {
					t.Fatal(err)
				}
				e.PublishMachine(token, func() { defer func() { _ = recover(); deferred = true }(); call(); after = true })
				gosimruntime.NewChan[struct{}](0).Recv()
			})
			var aborted *gosimruntime.AbortError
			if !errors.As(err, &aborted) || aborted.Diagnostic.Operation != name || after || deferred {
				t.Fatalf("cleanup err=%v after=%v defer=%v", err, after, deferred)
			}
		})
	}
}
