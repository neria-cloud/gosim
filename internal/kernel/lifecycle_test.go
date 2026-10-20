package kernel

import (
	"errors"
	"math"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

func configFor(features contracts.FeatureSet) Config {
	return Config{Features: features, TimeoutNS: int64(time.Second), BuildDigest: strings.Repeat("a", 64)}
}

func engineFor(t *testing.T) *simrt.Engine {
	t.Helper()
	e, err := simrt.NewEngine(
		simrt.EngineConfig{Seed: 42, TimeoutNS: int64(time.Second), ReplayRevision: simrt.ReplayRevision},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}
func idleMain() { simrt.NewChan[uint64](0).Recv() }
func createFor(t *testing.T, k *Kernel, label string, main func()) contracts.MachineRef {
	t.Helper()
	ref, err := k.Create(label, main)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func fixture(
	t *testing.T,
	features contracts.FeatureSet,
	providers []contracts.Provider,
	body func(*Kernel),
) (*Kernel, error) {
	t.Helper()
	e := engineFor(t)
	var k *Kernel
	err := e.Run(func() {
		var err error
		k, err = New(configFor(features), e, providers)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := k.Close(); err != nil {
				k.cleanupFailure("fixture cleanup", err)
			}
		}()
		body(k)
	})
	return k, err
}

func providersFor(log *[]string) (*recordingProvider, *recordingProvider, []contracts.Provider) {
	fs, net := &recordingProvider{feature: contracts.FS, log: log}, &recordingProvider{feature: contracts.Net, log: log}
	return fs, net, []contracts.Provider{net, fs}
}

func TestOpenOrderingAndReverseUnwind(t *testing.T) {
	for _, failure := range []string{"open-fs", "open-net", "controls-fs", "controls-net"} {
		t.Run(failure, func(t *testing.T) {
			var log []string
			fs, net, providers := providersFor(&log)
			switch failure {
			case "open-fs":
				fs.openErr = errInjected
			case "open-net":
				net.openErr = errInjected
			case "controls-fs":
				fs.badControls = true
			case "controls-net":
				net.badControls = true
			}
			e := engineFor(t)
			if err := e.Run(func() {
				if _, err := New(configFor(contracts.FeatureSet(contracts.RT|contracts.FS|contracts.Net)), e, providers); err == nil {
					t.Error("invalid open succeeded")
				}
			}); err != nil {
				t.Fatal(err)
			}
			want := map[string][]string{
				"open-fs":      {"fs:open"},
				"open-net":     {"fs:open", "fs:controls", "net:open", "fs:close"},
				"controls-fs":  {"fs:open", "fs:controls", "fs:close"},
				"controls-net": {"fs:open", "fs:controls", "net:open", "net:controls", "net:close", "fs:close"},
			}[failure]
			if !slices.Equal(log, want) {
				t.Fatalf("open order: %v", log)
			}
		})
	}
}

func TestPreparationRollbackConsumesEpoch(t *testing.T) {
	for _, failure := range []string{"prepare-fs", "prepare-net", "bindings-fs", "bindings-net"} {
		t.Run(failure, func(t *testing.T) {
			var log []string
			fs, net, providers := providersFor(&log)
			var boot atomic.Bool
			_, err := fixture(
				t,
				contracts.FeatureSet(contracts.RT|contracts.FS|contracts.Net),
				providers,
				func(k *Kernel) {
					ref := createFor(t, k, "first", func() { boot.Store(true); idleMain() })
					switch failure {
					case "prepare-fs":
						fs.prepareErr = errInjected
					case "prepare-net":
						net.prepareErr = errInjected
					case "bindings-fs":
						fs.badBindings = true
					case "bindings-net":
						net.badBindings = true
					}
					log = nil
					if _, err := k.BeginStart(ref, false); err == nil {
						t.Fatal("fault accepted")
					}
					m, _ := k.Lookup(ref)
					if m.state != StateNew || m.nextEpoch != 2 || m.current != nil || m.lastExit.Valid {
						t.Fatalf("bad rollback %+v", m)
					}
					if boot.Load() {
						t.Fatal("init ran before commit")
					}
					if len(k.transactions) != 0 {
						t.Fatal("transaction retained")
					}
					if failure == "prepare-net" &&
						!slices.Equal(
							log,
							[]string{"fs:prepare:1:1", "net:prepare:1:1", "fs:release:1:3", "fs:abort"},
						) {
						t.Fatalf("bad rollback order %v", log)
					}
					if strings.HasPrefix(failure, "bindings") &&
						!slices.Equal(
							log[len(log)-4:],
							[]string{"fs:release:1:3", "net:release:1:3", "net:abort", "fs:abort"},
						) {
						t.Fatalf("reverse rollback order %v", log)
					}
					fs.prepareErr, net.prepareErr, fs.badBindings, net.badBindings = nil, nil, false, false
					tx, err := k.BeginStart(ref, false)
					if err != nil || tx.run.ref.Epoch != 2 {
						t.Fatal("retry reused failed epoch", err)
					}
					simrt.Yield()
					if boot.Load() {
						t.Fatal("prepare started machine")
					}
					if err := tx.Commit(); err != nil {
						t.Fatal(err)
					}
					if boot.Load() {
						t.Fatal("boot inside publication")
					}
					if err := tx.Commit(); !errors.Is(err, ErrTxnState) {
						t.Fatal("double publication", err)
					}
					if err := k.Stop(ref, contracts.StopCrash); err != nil {
						t.Fatal(err)
					}
					if err := k.Stop(ref, contracts.StopGraceful); err != nil {
						t.Fatal("terminal stop not idempotent")
					}
					if m.lastExit.Epoch != 2 || m.lastExit.Kind != contracts.StopCrash {
						t.Fatal("wrong terminal record")
					}
				},
			)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAbortedTransactionAndRestartPreserveTerminalState(t *testing.T) {
	var log []string
	fs, net, providers := providersFor(&log)
	_, err := fixture(t, contracts.FeatureSet(contracts.RT|contracts.FS|contracts.Net), providers, func(k *Kernel) {
		ref := createFor(t, k, "persistent", idleMain)
		tx, err := k.BeginStart(ref, false)
		if err != nil {
			t.Fatal(err)
		}
		tx.Abort()
		tx.Abort()
		if err := tx.Commit(); !errors.Is(err, ErrTxnState) {
			t.Fatal("committed aborted transaction")
		}
		if err := k.Start(ref); err != nil {
			t.Fatal(err)
		}
		if err := k.Stop(ref, contracts.StopGraceful); err != nil {
			t.Fatal(err)
		}
		m, _ := k.Lookup(ref)
		exit := m.lastExit
		net.prepareErr = errInjected
		if err := k.Restart(ref); !errors.Is(err, errInjected) {
			t.Fatal(err)
		}
		if m.state != StateStopped || m.lastExit != exit || m.current != nil {
			t.Fatal("failed restart changed terminal state")
		}
		net.prepareErr = nil
		if err := k.Restart(ref); err != nil {
			t.Fatal(err)
		}
		if m.current.ref.Epoch != 4 || m.lastExit != exit {
			t.Fatal("restart reused epoch or lost exit")
		}
		if len(fs.resources) != 4 {
			t.Fatal("unexpected filesystem preparations")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitCapturesEpochAndStaleHandles(t *testing.T) {
	var firstRunID uint64
	for run := 0; run < 2; run++ {
		_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
			if firstRunID != 0 && firstRunID == k.runID {
				t.Fatal("run ID reused")
			}
			firstRunID = k.runID
			observer := createFor(t, k, "observer", idleMain)
			target := createFor(t, k, "target", idleMain)
			if err := k.Start(observer); err != nil {
				t.Fatal(err)
			}
			ctx := contracts.CallContext{Epoch: k.machineByID[observer.ID].current.ref}
			if _, err := k.Wait(ctx, target); !errors.Is(err, contracts.ErrNotStarted) {
				t.Fatal(err)
			}
			if err := k.Start(target); err != nil {
				t.Fatal(err)
			}
			old := k.machineByID[target.ID].current.ref
			id, err := k.Wait(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			if err := k.Stop(target, contracts.StopCrash); err != nil {
				t.Fatal(err)
			}
			if err := k.Restart(target); err != nil {
				t.Fatal(err)
			}
			if k.Accepts(old) || !k.waits[id].exit.Valid || k.waits[id].exit.Epoch != old.Epoch {
				t.Fatal("wait followed restart")
			}
			if _, err := k.Wait(ctx, observer); !errors.Is(err, contracts.ErrSelfTransition) {
				t.Fatal(err)
			}
			if _, err := k.Lookup(contracts.MachineRef{RunID: k.runID + 1, ID: target.ID}); !errors.Is(
				err,
				contracts.ErrWrongRun,
			) {
				t.Fatal(err)
			}
			if err := k.Stop(target, contracts.StopGraceful); err != nil {
				t.Fatal(err)
			}
			terminal, err := k.Wait(ctx, target)
			if err != nil || !k.waits[terminal].exit.Valid {
				t.Fatal("terminal wait", err)
			}
			if err := k.Destroy(target); err != nil {
				t.Fatal(err)
			}
			if err := k.Destroy(target); !errors.Is(err, contracts.ErrMachineNotFound) {
				t.Fatal(err)
			}
			if _, err := k.Lookup(target); !errors.Is(err, contracts.ErrMachineNotFound) {
				t.Fatal(err)
			}
			if err := k.Stop(observer, contracts.StopCrash); err != nil {
				t.Fatal(err)
			}
			if len(k.waits) != 0 {
				t.Fatal("dead observers retained")
			}
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestCleanupFailuresContinueAndUseCrashSemantics(t *testing.T) {
	var log []string
	fs, net, providers := providersFor(&log)
	fs.settleErr, net.releaseErr, net.panicDetach, fs.panicClose = errors.New(
		"settle fault",
	), errors.New(
		"release fault",
	), true, true
	k, err := fixture(t, contracts.FeatureSet(contracts.RT|contracts.FS|contracts.Net), providers, func(k *Kernel) {
		ref := createFor(t, k, "cleanup", idleMain)
		if err := k.Start(ref); err != nil {
			t.Fatal(err)
		}
		log = nil
		_ = k.Stop(ref, contracts.StopGraceful)
		t.Error("cleanup failure returned to execution")
	})
	if err == nil || !strings.Contains(err.Error(), "settle fault") ||
		!strings.Contains(err.Error(), "release fault") ||
		!strings.Contains(err.Error(), "detach failure") ||
		!strings.Contains(err.Error(), "close failure") {
		t.Fatalf("lost cleanup diagnostics: %v", err)
	}
	want := []string{
		"net:quiesce",
		"fs:quiesce",
		"fs:settle:0",
		"net:settle:1",
		"fs:release:1:2",
		"net:release:1:2",
		"net:detach",
		"fs:detach",
		"net:destroy:1",
		"fs:destroy:1",
		"net:close",
		"fs:close",
	}
	if !slices.Equal(log, want) {
		t.Fatalf("cleanup order: %v", log)
	}
	if k.machineByID[1].lastExit.Kind != contracts.StopCrash || !fs.resources[0].released ||
		!net.resources[0].released {
		t.Fatal("incomplete cleanup")
	}
}

func TestPublicationInvariantCannotBeRecovered(t *testing.T) {
	for _, failure := range []string{"first", "second", "nil", "yield"} {
		t.Run(failure, func(t *testing.T) {
			var log []string
			fs, net, providers := providersFor(&log)
			switch failure {
			case "first":
				fs.panicPublish = true
			case "second":
				net.panicPublish = true
			case "nil":
				net.nilInstance = true
			case "yield":
				net.yieldPublish = true
			}
			k, err := fixture(
				t,
				contracts.FeatureSet(contracts.RT|contracts.FS|contracts.Net),
				providers,
				func(k *Kernel) {
					ref := createFor(t, k, "broken", func() { t.Error("published broken machine") })
					_ = k.Start(ref)
					t.Error("publication failure returned")
				},
			)
			if err == nil || !strings.Contains(err.Error(), "publication") {
				t.Fatal(err)
			}
			if k.machineByID[1].lastExit.Kind != contracts.StopCrash {
				t.Fatal("published prefix was rolled back")
			}
			for _, resource := range append(fs.resources, net.resources...) {
				if !resource.released {
					t.Fatal("descriptor leaked")
				}
			}
		})
	}
}

func TestIdentityExhaustionAbortsWithoutReuse(t *testing.T) {
	for _, field := range []string{"machine", "epoch", "wait"} {
		t.Run(field, func(t *testing.T) {
			_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
				ref := createFor(t, k, "last", idleMain)
				switch field {
				case "machine":
					k.nextMachineID = math.MaxUint64
					_ = createFor(t, k, "final", idleMain)
					_, _ = k.Create("overflow", idleMain)
				case "epoch":
					k.machineByID[ref.ID].nextEpoch = math.MaxUint64
					tx, err := k.BeginStart(ref, false)
					if err != nil {
						t.Fatal(err)
					}
					tx.Abort()
					_, _ = k.BeginStart(ref, false)
				case "wait":
					other := createFor(t, k, "observer", idleMain)
					if err := k.Start(ref); err != nil {
						t.Fatal(err)
					}
					if err := k.Start(other); err != nil {
						t.Fatal(err)
					}
					ctx := contracts.CallContext{Epoch: k.machineByID[other.ID].current.ref}
					k.nextWaitID = contracts.WaitID(math.MaxUint64)
					if _, err := k.Wait(ctx, ref); err != nil {
						t.Fatal(err)
					}
					_, _ = k.Wait(ctx, ref)
				}
				t.Error("identity overflow returned")
			})
			if err == nil || !strings.Contains(err.Error(), "exhausted") {
				t.Fatal(err)
			}
		})
	}
}

func TestRollbackCleanupFailureStillReleasesAllOwners(t *testing.T) {
	var log []string
	fs, net, providers := providersFor(&log)
	fs.releaseErr, fs.panicAbort, net.prepareErr = errors.New("rollback release fault"), true, errInjected
	k, err := fixture(t, contracts.FeatureSet(contracts.RT|contracts.FS|contracts.Net), providers, func(k *Kernel) {
		ref := createFor(t, k, "abort", idleMain)
		_, _ = k.BeginStart(ref, false)
		t.Error("rollback failure returned to execution")
	})
	if err == nil || !strings.Contains(err.Error(), "rollback release fault") ||
		!strings.Contains(err.Error(), "abort failure") {
		t.Fatal("lost rollback failures", err)
	}
	if len(k.transactions) != 0 || !fs.resources[0].released || k.machineByID[1].nextEpoch != 2 {
		t.Fatal("rollback leaked ownership")
	}
	if !slices.Contains(log, "net:close") || !slices.Contains(log, "fs:close") {
		t.Fatal("rollback prevented session cleanup")
	}
}

func TestRuntimePrepareFailureRestoresState(t *testing.T) {
	_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
		ref := createFor(t, k, "collision", idleMain)
		occupied, err := k.engine.PrepareMachine(ref.ID, 1, "occupied")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := k.BeginStart(ref, false); !errors.Is(err, simrt.ErrMachineToken) {
			t.Fatal("runtime failure accepted", err)
		}
		m, _ := k.Lookup(ref)
		if m.state != StateNew || m.nextEpoch != 2 || len(k.transactions) != 0 {
			t.Fatal("runtime failure changed state")
		}
		k.engine.AbortMachine(occupied)
		if err := k.Start(ref); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}
