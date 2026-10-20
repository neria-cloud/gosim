package control

import (
	"errors"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	fsprovider "github.com/neria-cloud/gosim/internal/fs"
	"github.com/neria-cloud/gosim/internal/kernel"
	netprovider "github.com/neria-cloud/gosim/internal/net"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type request interface{ Invocation() kernel.Invocation }

func invoke[T any](p request, decode func(kernel.Result) (T, error)) (T, error) {
	inv := p.Invocation()
	if err := Submit(inv.Route, &inv); err != nil {
		var zero T
		return zero, err
	}
	return decode(inv.TakeResult())
}

func must[T any](t *testing.T, p request, decode func(kernel.Result) (T, error)) T {
	t.Helper()
	value, err := invoke(p, decode)
	if err != nil {
		t.Fatalf("%T: %v", p, err)
	}
	return value
}

func run(t *testing.T, features contracts.FeatureSet, main func()) error {
	t.Helper()
	e, err := simrt.NewEngine(simrt.EngineConfig{Seed: 17, TimeoutNS: 1e9, ReplayRevision: simrt.ReplayRevision})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	return e.Run(func() {
		var providers []contracts.Provider
		if features.Has(contracts.FS) {
			providers = append(providers, fsprovider.NewProvider())
		}
		if features.Has(contracts.Net) {
			providers = append(providers, netprovider.NewProvider())
		}
		identity := abi.HostIdentity{
			Toolchain: "test-toolchain", Profile: "test-profile", HostOS: "darwin", HostArch: "arm64",
			SourceDigest: strings.Repeat("a", 64),
		}
		config := kernel.Config{
			Identity: identity, RuntimeRevision: strings.Repeat("b", 64),
			BuildDigest: strings.Repeat("c", 64), Features: features, TimeoutNS: 1e9,
		}
		k, err := kernel.New(config, e, providers)
		if err != nil {
			t.Fatal(err)
		}
		Bind(k)
		defer func() {
			if err := k.Close(); err != nil {
				t.Error(err)
			}
			Bind(nil)
		}()
		if err := k.Run(main); err != nil {
			t.Error(err)
		}
	})
}

func TestTransportLifecycleAndRunIdentity(t *testing.T) {
	var previous contracts.MachineRef
	for range 2 {
		err := run(t, contracts.FeatureSet(contracts.RT), func() {
			root := CurrentRef()
			if root.ID == 0 || root.RunID == 0 {
				t.Fatal(root)
			}
			if previous.ID != 0 {
				if root.RunID == previous.RunID {
					t.Fatal("run ID reused")
				}
				_, err := invoke(kernel.LabelRequest{Machine: previous}, kernel.DecodeLabelResult)
				if !errors.Is(err, contracts.ErrWrongRun) {
					t.Fatal(err)
				}
			}
			previous = root
			// Caller context is stamped by transport, not trusted from the envelope.
			inv := kernel.GetpidRequest{}.Invocation()
			inv.Context.Epoch.Machine = contracts.MachineRef{RunID: 999, ID: 999}
			if err := Submit(inv.Route, &inv); err != nil {
				t.Fatal(err)
			}
			if inv.Context.Epoch.Machine != root {
				t.Fatal("forged caller context retained")
			}
			if result, err := kernel.DecodeGetpidResult(inv.TakeResult()); err != nil || result.PID != 42 {
				t.Fatal(result, err)
			}
			entered := simrt.NewChan[contracts.MachineRef](0)
			child := must(t, kernel.CreateRequest{
				Label: "child", Main: func() { entered.Send(CurrentRef()); simrt.NewChan[int](0).Recv() },
			}, kernel.DecodeCreateResult).Machine
			state := must(t, kernel.StateRequest{Machine: child}, kernel.DecodeStateResult).State
			if state != kernel.StateNew {
				t.Fatal(state)
			}
			if label := must(t, kernel.LabelRequest{Machine: child}, kernel.DecodeLabelResult).Label; label != "child" {
				t.Fatal(label)
			}
			must(t, kernel.StartRequest{Machine: child}, kernel.DecodeStartResult)
			if got := entered.Recv(); got != child {
				t.Fatal("child transport binding", got, child)
			}
			must(t, kernel.StopRequest{Machine: child, Kind: contracts.StopGraceful}, kernel.DecodeStopResult)
			exit := must(t, kernel.WaitRequest{Machine: child}, kernel.DecodeWaitResult).Exit
			if !exit.Valid || exit.Epoch != 1 || exit.Kind != contracts.StopGraceful {
				t.Fatal(exit)
			}
			must(t, kernel.RestartRequest{Machine: child}, kernel.DecodeRestartResult)
			entered.Recv()
			must(t, kernel.StopRequest{Machine: child, Kind: contracts.StopCrash}, kernel.DecodeStopResult)
			exit = must(t, kernel.WaitRequest{Machine: child}, kernel.DecodeWaitResult).Exit
			if exit.Epoch != 2 || exit.Kind != contracts.StopCrash {
				t.Fatal(exit)
			}
			must(t, kernel.SetMainRequest{Machine: child, Main: func() {}}, kernel.DecodeSetMainResult)
			must(t, kernel.RestartRequest{Machine: child}, kernel.DecodeRestartResult)
			if exit := must(t, kernel.WaitRequest{Machine: child}, kernel.DecodeWaitResult).Exit; exit.Epoch != 3 {
				t.Fatal(exit)
			}
			must(t, kernel.DestroyRequest{Machine: child}, kernel.DecodeDestroyResult)
			_, err := invoke(kernel.LabelRequest{Machine: child}, kernel.DecodeLabelResult)
			if !errors.Is(err, contracts.ErrMachineNotFound) {
				t.Fatal(err)
			}
			_, err = invoke(kernel.StopRequest{Machine: root}, kernel.DecodeStopResult)
			if !errors.Is(err, contracts.ErrSelfTransition) {
				t.Fatal(err)
			}
			_, err = invoke(kernel.WaitRequest{Machine: root}, kernel.DecodeWaitResult)
			if !errors.Is(err, contracts.ErrSelfTransition) {
				t.Fatal(err)
			}
			info := must(t, kernel.InfoRequest{}, kernel.DecodeInfoResult)
			if info.Seed != 17 || info.Profile != "test-profile" || info.RuntimeRevision != strings.Repeat("b", 64) ||
				len(info.Features) != 1 || info.Certified {
				t.Fatal(info)
			}
			info.Features[0] = "mutation"
			if must(t, kernel.InfoRequest{}, kernel.DecodeInfoResult).Features[0] != "rt" {
				t.Fatal("run info leaked mutable state")
			}
			_, err = invoke(kernel.SetTimeoutRequest{}, kernel.DecodeSetTimeoutResult)
			if !errors.Is(err, contracts.ErrInvalidConfig) {
				t.Fatal(err)
			}
			must(t, kernel.SetTimeoutRequest{TimeoutNS: 1e6}, kernel.DecodeSetTimeoutResult)
			func() {
				defer func() {
					if recover() == nil {
						t.Error("application rebind accepted")
					}
				}()
				Bind(nil)
			}()
			if CurrentRef() != root {
				t.Fatal("failed rebind changed current run")
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if CurrentRef() != (contracts.MachineRef{}) || binding.kernel != nil {
			t.Fatal("run binding retained")
		}
	}
	inv := kernel.InfoRequest{}.Invocation()
	if err := Submit(inv.Route, &inv); !errors.Is(err, contracts.ErrNotSim) {
		t.Fatal(err)
	}
}

func TestTransportProviderControls(t *testing.T) {
	err := run(t, contracts.FeatureSet(contracts.RT|contracts.FS|contracts.Net), func() {
		root := CurrentRef()
		child := must(t, kernel.CreateRequest{
			Label: "storage", Main: func() { simrt.NewChan[int](0).Recv() },
		}, kernel.DecodeCreateResult).Machine
		must(t, kernel.FSConfigureMachineRequest{
			Machine: child, Config: contracts.FSConfig{WorkingDir: "/app"},
		}, kernel.DecodeFSConfigureMachineResult)
		must(t, kernel.FSSetCrashModeRequest{Machine: child, Mode: 1}, kernel.DecodeFSSetCrashModeResult)
		must(t, kernel.FSSetSyncFaultRequest{
			Machine: child, Fault: contracts.FSFaultConfig{},
		}, kernel.DecodeFSSetSyncFaultResult)
		must(t, kernel.NetConfigureRequest{
			Config: contracts.NetConfig{MinNS: 1, MaxNS: 2, Lambda: 1, Repair: 1},
		}, kernel.DecodeNetConfigureResult)
		addr := [4]byte{11, 0, 0, 2}
		configureAddress := kernel.NetConfigureMachineRequest{Machine: child, Address: addr}
		must(t, configureAddress, kernel.DecodeNetConfigureMachineResult)
		must(t, kernel.StartRequest{Machine: child}, kernel.DecodeStartResult)
		machineAddress := must(t, kernel.NetAddressRequest{Machine: child}, kernel.DecodeNetAddressResult).Address
		if machineAddress != addr {
			t.Fatal(machineAddress)
		}
		from := must(t, kernel.NetAddressRequest{Machine: root}, kernel.DecodeNetAddressResult).Address
		link := contracts.NetLink{From: from, To: addr, Direction: 1}
		must(t, kernel.NetSetLinkPolicyRequest{
			Link: link, Policy: contracts.NetLinkPolicy{SetLatency: true, MinNS: 2, MaxNS: 3, Lambda: 1},
		}, kernel.DecodeNetSetLinkPolicyResult)
		must(t, kernel.NetResetLinkPolicyRequest{Link: link}, kernel.DecodeNetResetLinkPolicyResult)
		must(t, kernel.NetSetConnectivityRequest{Link: link, Connected: false}, kernel.DecodeNetSetConnectivityResult)
		must(t, kernel.NetSetConnectivityRequest{Link: link, Connected: true}, kernel.DecodeNetSetConnectivityResult)
		must(t, kernel.FSInspectInodeRequest{Machine: child, Inode: 1}, kernel.DecodeFSInspectInodeResult)
		must(t, kernel.StopRequest{Machine: child, Kind: contracts.StopCrash}, kernel.DecodeStopResult)
		enumerateRequest := kernel.FSEnumerateRequest{Machine: child, MaxStates: 10}
		iterator := must(t, enumerateRequest, kernel.DecodeFSEnumerateResult).Iterator
		next := must(t, kernel.FSNextSnapshotRequest{
			RunID: root.RunID, Iterator: iterator,
		}, kernel.DecodeFSNextSnapshotResult)
		if !next.OK || next.Snapshot == 0 {
			t.Fatal(next)
		}
		must(
			t,
			kernel.FSCloseIteratorRequest{RunID: root.RunID, Iterator: iterator},
			kernel.DecodeFSCloseIteratorResult,
		)
		if _, err := invoke(kernel.FSRestoreRequest{
			Machine: child, SnapshotRunID: root.RunID + 1, Snapshot: next.Snapshot,
		}, kernel.DecodeFSRestoreResult); !errors.Is(err, contracts.ErrWrongRun) {
			t.Fatal(err)
		}
		must(t, kernel.FSRestoreRequest{
			Machine: child, SnapshotRunID: root.RunID, Snapshot: next.Snapshot,
		}, kernel.DecodeFSRestoreResult)
		closeSnapshot := kernel.FSCloseSnapshotRequest{RunID: root.RunID, Snapshot: next.Snapshot}
		must(t, closeSnapshot, kernel.DecodeFSCloseSnapshotResult)
		must(t, kernel.DestroyRequest{Machine: child}, kernel.DecodeDestroyResult)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTransportAbortsDisabledAndForgedCalls(t *testing.T) {
	for _, kind := range []string{"fs", "net", "payload", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			after, deferred := false, false
			err := run(t, contracts.FeatureSet(contracts.RT), func() {
				defer func() { _ = recover(); deferred = true }()
				var inv kernel.Invocation
				switch kind {
				case "fs":
					inv = kernel.FSSetCrashModeRequest{Machine: CurrentRef()}.Invocation()
				case "net":
					inv = kernel.NetAddressRequest{Machine: CurrentRef()}.Invocation()
				case "payload":
					inv = kernel.GetpidRequest{}.Invocation()
					inv.Args[0] = func() { t.Error("callback executed") }
				case "unknown":
					inv.Route = kernel.RouteID(99999)
				}
				_ = Submit(inv.Route, &inv)
				after = true
			})
			var failure *simrt.AbortError
			if !errors.As(err, &failure) || after || deferred {
				t.Fatalf("abort %v after=%v defer=%v", err, after, deferred)
			}
			want := "FeatureDisabled"
			if kind == "payload" {
				want = "InvalidABI"
			}
			if kind == "unknown" {
				want = "UnsupportedOperation"
			}
			if failure.Diagnostic.Kind != want {
				t.Fatal(err)
			}
		})
	}
}

func TestTransportSelfExitCancelsCaller(t *testing.T) {
	for _, kind := range []contracts.StopKind{contracts.StopGraceful, contracts.StopCrash} {
		after, deferred := false, false
		err := run(t, contracts.FeatureSet(contracts.RT), func() {
			defer func() { deferred = true }()
			inv := kernel.ExitSelfRequest{Kind: kind}.Invocation()
			_ = Submit(inv.Route, &inv)
			after = true
		})
		if err != nil || after || deferred {
			t.Fatalf("self exit %v after=%v defer=%v", err, after, deferred)
		}
	}
}

func TestChildSelfExitKeepsControllerAlive(t *testing.T) {
	for _, kind := range []contracts.StopKind{contracts.StopGraceful, contracts.StopCrash} {
		after, deferred, controller := false, false, false
		err := run(t, contracts.FeatureSet(contracts.RT), func() {
			child := must(t, kernel.CreateRequest{Label: "self-exit", Main: func() {
				defer func() { deferred = true }()
				inv := kernel.ExitSelfRequest{Kind: kind}.Invocation()
				_ = Submit(inv.Route, &inv)
				after = true
			}}, kernel.DecodeCreateResult).Machine
			must(t, kernel.StartRequest{Machine: child}, kernel.DecodeStartResult)
			exit := must(t, kernel.WaitRequest{Machine: child}, kernel.DecodeWaitResult).Exit
			if !exit.Valid || exit.Epoch != 1 || exit.Kind != kind {
				t.Fatal(exit)
			}
			controller = true
		})
		if err != nil || after || deferred || !controller {
			t.Fatalf("child self exit %v after=%v defer=%v controller=%v", err, after, deferred, controller)
		}
	}
}
