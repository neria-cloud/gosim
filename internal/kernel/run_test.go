package kernel

import (
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

func TestKernelRunSelfExitAndRootLastCleanup(t *testing.T) {
	var log []string
	_, _, providers := providersFor(&log)
	e := engineFor(t)
	var k *Kernel
	var returned atomic.Bool
	err := e.Run(func() {
		var err error
		k, err = New(configFor(contracts.FeatureSet(contracts.RT|contracts.FS|contracts.Net)), e, providers)
		if err != nil {
			t.Fatal(err)
		}
		// The root need not be the highest/lowest ID for root-last teardown.
		child := createFor(t, k, "child", idleMain)
		if err := k.Start(child); err != nil {
			t.Fatal(err)
		}
		if err := k.Run(func() {
			ref := contracts.MachineRef{RunID: k.runID, ID: simrt.CurrentMachineToken().ID}
			for _, transition := range []func() error{
				func() error { return k.Stop(ref, contracts.StopGraceful) },
				func() error { return k.Restart(ref) },
				func() error { return k.Destroy(ref) },
			} {
				if err := transition(); !errors.Is(err, contracts.ErrSelfTransition) {
					t.Errorf("returning self transition: %v", err)
				}
			}
			if err := k.SetMain(ref, idleMain); !errors.Is(err, contracts.ErrInvalidState) {
				t.Errorf("running main replacement: %v", err)
			}
			returned.Store(true)
		}); err != nil {
			t.Error(err)
		}
	})
	if err != nil || !returned.Load() {
		t.Fatal("root did not exit normally", err)
	}
	if k.machineByID[1].lastExit.Kind != contracts.StopGraceful || k.machineByID[2].lastExit.Kind != contracts.StopGraceful {
		t.Fatal("main return did not gracefully finalize")
	}
	var releases []string
	for _, event := range log {
		if strings.Contains(event, ":release:") {
			releases = append(releases, event)
		}
	}
	if !slices.Equal(releases, []string{"fs:release:1:1", "net:release:1:1", "fs:release:2:1", "net:release:2:1"}) {
		t.Fatalf("root did not finalize last: %v", releases)
	}
	if err := k.Close(); err != nil {
		t.Fatal("close not idempotent", err)
	}
}

func TestKernelRequiresActiveEngineAndOwnsIdentity(t *testing.T) {
	e := engineFor(t)
	config := configFor(contracts.FeatureSet(contracts.RT))
	config.Identity.Profile = "development-test"
	if _, err := New(config, e, nil); !errors.Is(err, simrt.ErrEngineState) {
		t.Fatal("inactive engine accepted", err)
	}
	if err := e.Run(func() {
		k, err := New(config, e, nil)
		if err != nil {
			t.Fatal(err)
		}
		config.Features, config.Identity.Profile = contracts.FeatureSet(contracts.RT|contracts.Net), "mutated"
		if k.features != contracts.FeatureSet(contracts.RT) || k.config.Identity.Profile != "development-test" {
			t.Fatal("kernel identity changed with caller configuration")
		}
		if err := k.Close(); err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func TestChildMainReturnAndRootProtection(t *testing.T) {
	e := engineFor(t)
	var k *Kernel
	var child contracts.MachineRef
	err := e.Run(func() {
		var err error
		k, err = New(configFor(contracts.FeatureSet(contracts.RT)), e, nil)
		if err != nil {
			t.Fatal(err)
		}
		child = createFor(t, k, "child", func() {})
		if err := k.Start(child); err != nil {
			t.Fatal(err)
		}
		if err := k.Run(func() {
			// Child exit is sent through the supervisor's internal route.
			// Give the supervisor time to process it without touching its maps.
			for range 20 {
				simrt.Yield()
			}
		}); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if k.machineByID[child.ID].lastExit.Kind != contracts.StopGraceful {
		t.Fatal("child main did not gracefully terminate")
	}
	_, err = fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
		ref := createFor(t, k, "protected", idleMain)
		k.root = ref.ID
		if err := k.Start(ref); err != nil {
			t.Fatal(err)
		}
		if err := k.Stop(ref, contracts.StopCrash); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("root stop accepted", err)
		}
		if err := k.Destroy(ref); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("root destroy accepted", err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunPanicAndDisabledFeatureCrashCleanup(t *testing.T) {
	for _, failure := range []string{"panic", "disabled"} {
		t.Run(failure, func(t *testing.T) {
			e := engineFor(t)
			var k *Kernel
			var recovered, after atomic.Bool
			err := e.Run(func() {
				var err error
				k, err = New(configFor(contracts.FeatureSet(contracts.RT)), e, nil)
				if err != nil {
					t.Fatal(err)
				}
				_ = k.Run(func() {
					if failure == "panic" {
						panic("application init/main failure")
					}
					defer func() { _ = recover(); recovered.Store(true) }()
					k.Require(contracts.CallContext{Epoch: contracts.EpochRef{Machine: contracts.MachineRef{RunID: k.runID, ID: 1}, Epoch: 1}, PC: 42}, contracts.FS, "openat")
					after.Store(true)
				})
			})
			if err == nil || !k.closed || k.machineByID[1].lastExit.Kind != contracts.StopCrash {
				t.Fatalf("run failure escaped cleanup: %v", err)
			}
			if failure == "disabled" {
				var diagnostic *simrt.AbortError
				if !errors.As(err, &diagnostic) || diagnostic.Diagnostic.Kind != "FeatureDisabled" || diagnostic.Diagnostic.PC != 42 || recovered.Load() || after.Load() {
					t.Fatalf("recoverable feature guard: %v", err)
				}
			}
		})
	}
}

func TestTrustedSelfCrashDoesNotRunDefers(t *testing.T) {
	e := engineFor(t)
	var deferred atomic.Bool
	var k *Kernel
	err := e.Run(func() {
		var err error
		k, err = New(configFor(contracts.FeatureSet(contracts.RT)), e, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := k.Run(func() {
			defer deferred.Store(true)
			requestSelfExit(k.exits, simrt.CurrentMachineToken(), contracts.StopCrash)
			t.Error("self-crash returned")
		}); err != nil {
			t.Error(err)
		}
	})
	if err != nil || deferred.Load() || k.machineByID[1].lastExit.Kind != contracts.StopCrash {
		t.Fatalf("self-crash: %v, deferred=%t", err, deferred.Load())
	}
}

func TestPortsEpochAdmissionCopyingAndRandomOwnership(t *testing.T) {
	var log []string
	fs, net, providers := providersFor(&log)
	var stale, live atomic.Bool
	_, err := fixture(t, contracts.FeatureSet(contracts.RT|contracts.FS|contracts.Net), providers, func(k *Kernel) {
		ref := createFor(t, k, "ports", idleMain)
		tx, err := k.BeginStart(ref, false)
		if err != nil {
			t.Fatal(err)
		}
		ports := fs.ports
		if _, err := ports.Events.Schedule(tx.run.ref, ports.Clock.NowNanos(), func() {}); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("staged event accepted")
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		old := k.machineByID[ref.ID].current.ref
		stream := ports.RandomFor(contracts.FS, ref)
		want := simrt.NewDomainRandom(42, "fs", ref.ID)
		if stream.Uint64() != want.Uint64() {
			t.Fatal("wrong fs stream")
		}
		_ = net.ports.RandomFor(contracts.Net, ref).Uint64()
		if stream.Uint64() != want.Uint64() {
			t.Fatal("cross-domain RNG coupling")
		}
		if _, err := ports.Events.Schedule(old, ports.Clock.NowNanos()+100, func() { stale.Store(true) }); err != nil {
			t.Fatal(err)
		}
		if err := k.Stop(ref, contracts.StopCrash); err != nil {
			t.Fatal(err)
		}
		if err := k.Restart(ref); err != nil {
			t.Fatal(err)
		}
		current := k.machineByID[ref.ID].current.ref
		if ports.RandomFor(contracts.FS, ref).Uint64() != want.Uint64() {
			t.Fatal("restart reset fault stream")
		}
		if _, err := ports.Events.Schedule(old, ports.Clock.NowNanos(), func() {}); err == nil {
			t.Fatal("stale event admitted")
		}
		fields := []contracts.Field{{Key: "owner", Value: "saved"}}
		ports.Trace.Emit(current, "test", fields)
		fields[0].Value = "mutated"
		if k.traces[0].fields[0].Value != "saved" {
			t.Fatal("trace retains caller fields")
		}
		if _, err := ports.Events.Schedule(current, ports.Clock.NowNanos()+200, func() { live.Store(true) }); err != nil {
			t.Fatal(err)
		}
		// Virtual sleep advances the real engine to both timer deadlines.
		simrt.Sleep(300)
		if stale.Load() || !live.Load() {
			t.Fatal("event epoch admission failed")
		}
		if len(k.events) != 0 {
			t.Fatal("fired/canceled event retained")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}
