package fs

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/kernel"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

type captureProvider struct {
	Provider
	session *Session
}

func (p *captureProvider) Open(ports contracts.Ports) (contracts.ProviderSession, error) {
	session, err := p.Provider.Open(ports)
	if err == nil {
		p.session = session.(*Session)
	}
	return session, err
}

func withKernel(t *testing.T, extra []contracts.Provider, body func(*kernel.Kernel, *Session)) error {
	t.Helper()
	engine, err := simrt.NewEngine(
		simrt.EngineConfig{Seed: 42, TimeoutNS: int64(time.Second), ReplayRevision: simrt.ReplayRevision},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	return engine.Run(func() {
		provider := &captureProvider{}
		features := contracts.FeatureSet(contracts.RT | contracts.FS)
		if len(extra) > 0 {
			features |= contracts.FeatureSet(contracts.Net)
		}
		k, err := kernel.New(
			kernel.Config{Features: features, TimeoutNS: int64(time.Second), BuildDigest: strings.Repeat("a", 64)},
			engine,
			append([]contracts.Provider{provider}, extra...),
		)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { must(t, k.Close()) }()
		body(k, provider.session)
	})
}

func create(t *testing.T, k *kernel.Kernel) contracts.MachineRef {
	t.Helper()
	ref, err := k.Create("test", func() { simrt.NewChan[int](0).Recv() })
	must(t, err)
	return ref
}

func controls(t *testing.T, k *kernel.Kernel, ref contracts.MachineRef) contracts.FSControl {
	t.Helper()
	c, err := k.FilesystemControl(contracts.CallContext{}, ref.RunID)
	must(t, err)
	return c
}

func TestKernelFilesystemControlStateAndRunGuards(t *testing.T) {
	var oldRef contracts.MachineRef
	var oldSnapshot uint64
	var oldControl contracts.FSControl
	must(t, withKernel(t, nil, func(k *kernel.Kernel, s *Session) {
		ref := create(t, k)
		c := controls(t, k, ref)
		oldControl = c
		oldRef = ref
		must(t, c.ConfigureMachine(ref, contracts.FSConfig{WorkingDir: "/app"}))
		if _, err := c.Enumerate(ref, 1); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("enumerated New", err)
		}
		tx, err := k.BeginStart(ref, false)
		must(t, err)
		if err := c.SetCrashMode(ref, 1); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("control during preparation", err)
		}
		tx.Abort()
		must(t, k.Start(ref))
		if err := c.ConfigureMachine(ref, contracts.FSConfig{WorkingDir: "/etc"}); !errors.Is(
			err,
			contracts.ErrInvalidState,
		) {
			t.Fatal("configured running machine", err)
		}
		must(t, c.SetSyncFault(ref, contracts.FSFaultConfig{Probability: 0.1}))
		instance := s.machines[ref.ID].active
		epochControl, err := k.FilesystemControl(contracts.CallContext{Epoch: instance.ref}, ref.RunID)
		must(t, err)
		inode := write(t, instance.model, "/app/recover", "live")
		info, err := c.InspectInode(ref, inode)
		must(t, err)
		if !info.MemExists || info.Ops == 0 {
			t.Fatal("live inode control missed pending state", info)
		}
		must(t, k.Stop(ref, contracts.StopCrash))
		if err := epochControl.SetCrashMode(ref, 0); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("stale caller control accepted", err)
		}
		iterator, err := c.Enumerate(ref, 100)
		must(t, err)
		snapshot, ok, err := c.NextSnapshot(iterator)
		must(t, err)
		if !ok {
			t.Fatal("missing snapshot")
		}
		oldSnapshot = snapshot
		target := create(t, k)
		must(t, c.Restore(target, snapshot))
		must(t, k.Destroy(ref))
		if err := c.SetCrashMode(ref, 0); !errors.Is(err, contracts.ErrMachineNotFound) {
			t.Fatal("destroyed target accepted", err)
		}
		must(t, c.CloseIterator(iterator))
		must(t, c.CloseSnapshot(snapshot))
		must(t, c.CloseSnapshot(snapshot))
		must(t, k.Start(target))
		if read(t, s.machines[target.ID].active.model, "/app/recover") != "live" {
			t.Fatal("restored state lost after source destroy")
		}
		must(t, k.Stop(target, contracts.StopGraceful))
		if _, err := c.Enumerate(target, 1); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("enumerated graceful stop", err)
		}
		if err := c.ConfigureMachine(target, contracts.FSConfig{WorkingDir: "/app"}); !errors.Is(
			err,
			contracts.ErrInvalidState,
		) {
			t.Fatal("configured after first start", err)
		}
	}))
	if err := oldControl.CloseSnapshot(oldSnapshot); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal("closed-run control usable", err)
	}
	must(t, withKernel(t, nil, func(k *kernel.Kernel, _ *Session) {
		ref := create(t, k)
		if _, err := k.FilesystemControl(contracts.CallContext{}, oldRef.RunID); !errors.Is(
			err,
			contracts.ErrWrongRun,
		) {
			t.Fatal("cross-run snapshot control accepted", err)
		}
		if err := controls(t, k, ref).SetCrashMode(oldRef, 0); !errors.Is(err, contracts.ErrWrongRun) {
			t.Fatal("cross-run target accepted", err)
		}
	}))
}

type failingNet struct {
	contracts.ProviderSession
}
type netControlStub struct{ contracts.NetControl }

func (*failingNet) Feature() contracts.Feature                                { return contracts.Net }
func (p *failingNet) Open(contracts.Ports) (contracts.ProviderSession, error) { return p, nil }
func (*failingNet) Controls() contracts.ControlBindings {
	return contracts.ControlBindings{NetControl: netControlStub{}}
}
func (*failingNet) Close()                       {}
func (*failingNet) Destroy(contracts.MachineRef) {}
func (p *failingNet) Prepare(contracts.StartSpec, contracts.DescriptorTable) (contracts.PreparedMachine, error) {
	return nil, errors.New("injected second-provider failure")
}

func TestSecondProviderFailureReleasesFilesystemPreparation(t *testing.T) {
	must(t, withKernel(t, []contracts.Provider{&failingNet{}}, func(k *kernel.Kernel, s *Session) {
		ref := create(t, k)
		must(t, controls(t, k, ref).ConfigureMachine(ref, contracts.FSConfig{WorkingDir: "/etc"}))
		if err := k.Start(ref); err == nil {
			t.Fatal("expected second-provider failure")
		}
		m := s.machines[ref.ID]
		if m.disk != nil || m.active != nil || m.prepared != nil || m.started {
			t.Fatal("failed transaction published filesystem state")
		}
		must(t, controls(t, k, ref).ConfigureMachine(ref, contracts.FSConfig{WorkingDir: "/app"}))
	}))
}
