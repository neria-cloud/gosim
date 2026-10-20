package kernel

import (
	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

// TransportBinding is available only while the supervisor constructs the run.
// Application entry points receive no kernel pointer and cannot rebind dispatch.
func (k *Kernel) TransportBinding() (uint64, *Dispatcher, error) {
	if simrt.CurrentMachineToken().ID != 0 || k.running || k.closed || k.root != 0 {
		return 0, nil, contracts.ErrInvalidState
	}
	if err := k.engine.BeginTransition(); err != nil {
		return 0, nil, err
	}
	k.engine.EndTransition()
	return k.runID, k.dispatcher, nil
}

// TransportClosed lets the harness release its binding after kernel cleanup.
// It exposes no machine/provider state to application wrappers.
//
//go:norace
func (k *Kernel) TransportClosed() bool { return k.closed }

func (k *Kernel) Label(ref contracts.MachineRef) (string, error) {
	m, err := k.Lookup(ref)
	if err != nil {
		return "", err
	}
	return m.label, nil
}

func (k *Kernel) MachineState(ref contracts.MachineRef) (State, error) {
	m, err := k.Lookup(ref)
	if err != nil {
		return 0, err
	}
	return m.state, nil
}

func (k *Kernel) SetTimeout(ns int64) error {
	if ns <= 0 {
		return contracts.ErrInvalidConfig
	}
	if err := k.engine.SetTimeout(ns); err != nil {
		return contracts.ErrInvalidConfig
	}
	k.config.TimeoutNS = ns
	return nil
}

// runInfo copies kernel-owned metadata without creating application synchronization.
//
//go:norace
func (k *Kernel) runInfo() InfoResult {
	features := []string{"rt"}
	if k.features.Has(contracts.FS) {
		features = append(features, "fs")
	}
	if k.features.Has(contracts.Net) {
		features = append(features, "net")
	}

	return InfoResult{
		Seed: simrt.Seed(), Toolchain: k.config.Identity.Toolchain, Profile: k.config.Identity.Profile,
		AdapterRevision: k.config.Identity.SourceDigest, RuntimeRevision: k.config.RuntimeRevision,
		Features: features, HostOS: k.config.Identity.HostOS, HostArch: k.config.Identity.HostArch,
		SimulationOS: "linux", SimulationArch: k.config.Identity.HostArch, BuildDigest: k.config.BuildDigest,
		ReplayRevision: simrt.ReplayRevision,
	}
}

func (d *Dispatcher) decodeMachineWait(ctx contracts.CallContext, ref contracts.MachineRef) (Result, bool, error) {
	id, err := d.kernel.Wait(ctx, ref)
	if err != nil {
		return controlRouteResult(WaitResult{}, err)
	}
	wait := d.kernel.waits[id]
	if wait.exit.Valid {
		delete(d.kernel.waits, id)
		return controlRouteResult(WaitResult{Exit: wait.exit}, nil)
	}
	return Result{}, true, nil
}

func (k *Kernel) exitSelf(ctx contracts.CallContext, kind contracts.StopKind) error {
	if !k.Accepts(ctx.Epoch) || (kind != contracts.StopGraceful && kind != contracts.StopCrash) {
		return contracts.ErrInvalidConfig
	}
	if ctx.Epoch.Machine.ID == k.root {
		k.closeKind = kind
		return k.Close()
	}
	return k.stop(k.machineByID[ctx.Epoch.Machine.ID], kind)
}
