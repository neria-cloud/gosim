package fs

import (
	"errors"
	"math"
	"os"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/fs/model"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type testPorts struct {
	draws map[contracts.MachineRef]*countingRandom
}
type countingRandom struct {
	contracts.Random
	draws int
}

func (r *countingRandom) Uint64N(n uint64) uint64 { r.draws++; return r.Random.Uint64N(n) }
func (p *testPorts) NowNanos() int64              { return 1 }
func (p *testPorts) Schedule(contracts.EpochRef, int64, func()) (contracts.EventID, error) {
	panic("filesystem lifecycle scheduled work")
}
func (p *testPorts) Cancel(contracts.EventID)                           {}
func (p *testPorts) CancelEpoch(contracts.EpochRef)                     {}
func (p *testPorts) Emit(contracts.EpochRef, string, []contracts.Field) {}
func (p *testPorts) ports() contracts.Ports {
	return contracts.Ports{Clock: p, Events: p, Trace: p, RandomFor: func(feature contracts.Feature, ref contracts.MachineRef) contracts.Random {
		if feature != contracts.FS {
			panic("filesystem requested another domain")
		}
		if r := p.draws[ref]; r != nil {
			return r
		}
		r := &countingRandom{Random: simrt.NewDomainRandom(42, "fs", ref.ID)}
		p.draws[ref] = r
		return r
	}}
}

// Lifecycle preparation must not use descriptors before FileOps opens a file.
type noDescriptors struct{ contracts.DescriptorTable }

func newSession(t *testing.T) (*Session, *testPorts) {
	t.Helper()
	ports := &testPorts{draws: make(map[contracts.MachineRef]*countingRandom)}
	session, err := NewProvider().Open(ports.ports())
	if err != nil {
		t.Fatal(err)
	}
	s := session.(*Session)
	t.Cleanup(s.Close)
	return s, ports
}

func prepare(t *testing.T, s *Session, ref contracts.MachineRef, epoch uint64, restart bool) *Prepared {
	t.Helper()
	p, err := s.Prepare(contracts.StartSpec{Ref: contracts.EpochRef{Machine: ref, Epoch: epoch}, Restart: restart}, noDescriptors{})
	if err != nil {
		t.Fatal(err)
	}
	return p.(*Prepared)
}

func start(t *testing.T, s *Session, ref contracts.MachineRef, epoch uint64, restart bool) *Instance {
	t.Helper()
	return prepare(t, s, ref, epoch, restart).Publish().(*Instance)
}

func write(t *testing.T, fs *model.Filesystem, name, value string) int {
	t.Helper()
	inode, err := fs.OpenFile(model.RootInode, name, os.O_CREATE|os.O_RDWR, 0o644, model.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := fs.Write(inode, 0, abi.ByteView{Ptr: []byte(value)}, model.DefaultCapacityBytes); err != nil || n != len(value) {
		t.Fatal("short write")
	}
	if err := fs.CloseFile(inode); err != nil {
		t.Fatal(err)
	}
	return inode
}

func read(t *testing.T, fs *model.Filesystem, name string) string {
	t.Helper()
	inode, err := fs.OpenFile(model.RootInode, name, os.O_RDONLY, 0o644, model.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	defer fs.CloseFile(inode)
	buf := make([]byte, 128)
	n := fs.Read(inode, 0, abi.ByteView{Ptr: buf})
	return string(buf[:n])
}

func stop(t *testing.T, i *Instance, kind contracts.StopKind) {
	t.Helper()
	i.Quiesce()
	if err := i.Settle(kind); err != nil {
		t.Fatal(err)
	}
	i.Detach()
}

var ref1 = contracts.MachineRef{RunID: 1, ID: 1}

func TestSessionControlsBeforeStartAndAtomicConfig(t *testing.T) {
	t.Parallel()
	s, _ := newSession(t)
	if s.Controls().FSControl != s || s.Controls().NetControl != nil {
		t.Fatal("incorrect control ownership")
	}
	for _, cfg := range []contracts.FSConfig{{}, {WorkingDir: "relative"}, {WorkingDir: "/bad\x00"}, {WorkingDir: "/missing"}} {
		if err := s.ConfigureMachine(ref1, cfg); err == nil {
			t.Fatal("accepted invalid config", cfg)
		}
		if len(s.machines) != 0 {
			t.Fatal("invalid config created state")
		}
	}
	if err := s.ConfigureMachine(ref1, contracts.FSConfig{WorkingDir: "/etc"}); err != nil {
		t.Fatal(err)
	}
	if s.machines[1].disk != nil {
		t.Fatal("configuration published an initial disk")
	}
	original := s.machines[1].config
	if err := s.ConfigureMachine(ref1, contracts.FSConfig{WorkingDir: "/missing"}); err == nil || s.machines[1].config != original {
		t.Fatal("configuration was not atomic")
	}
	fault := contracts.FSFaultConfig{Probability: 0.2, BeforeProbability: 0.7}
	if err := s.SetSyncFault(ref1, fault); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{-1, 2, math.NaN(), math.Inf(1)} {
		for _, bad := range []contracts.FSFaultConfig{{Probability: value}, {BeforeProbability: value}} {
			if err := s.SetSyncFault(ref1, bad); !errors.Is(err, contracts.ErrInvalidConfig) || s.machines[1].fault != fault {
				t.Fatal("invalid fault changed state")
			}
		}
	}
	if err := s.SetCrashMode(ref1, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCrashMode(ref1, 2); !errors.Is(err, contracts.ErrInvalidConfig) || s.machines[1].crashMode != 1 {
		t.Fatal("invalid mode changed state")
	}
	p := prepare(t, s, ref1, 1, false)
	if p.instance.workingDir != 2 || s.machines[1].disk != nil {
		t.Fatal("cwd not staged")
	}
	if err := s.ConfigureMachine(ref1, contracts.FSConfig{WorkingDir: "/app"}); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal(err)
	}
	p.Abort()
	p.Abort()
	if err := s.ConfigureMachine(ref1, contracts.FSConfig{WorkingDir: "/app"}); err != nil {
		t.Fatal(err)
	}
	i := start(t, s, ref1, 2, false)
	if i.workingDir != 4 || read(t, i.model, "/etc/hosts") != "# hosts\n127.0.0.1 localhost\n" {
		t.Fatal("Linux fixture changed")
	}
	stop(t, i, contracts.StopCrash)
	if err := s.ConfigureMachine(ref1, original); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal("configured after successful start", err)
	}
	if err := s.SetSyncFault(ref1, contracts.FSFaultConfig{}); err != nil {
		t.Fatal("control unavailable after crash", err)
	}
}

func TestPrepareAbortPreservesDiskAndRandom(t *testing.T) {
	t.Parallel()
	s, ports := newSession(t)
	p := prepare(t, s, ref1, 1, false)
	initial := p.instance.model
	write(t, initial, "/app/tentative", "abort")
	p.Abort()
	if s.machines[1].disk != nil || s.machines[1].started || p.instance.model != nil || len(ports.draws) != 0 {
		t.Fatal("abort published or retained staged state")
	}
	i := start(t, s, ref1, 2, false)
	if _, err := i.model.Stat(model.RootInode, "/app/tentative", model.Identity{}); !errors.Is(err, syscall.ENOENT) {
		t.Fatal("tentative write survived", err)
	}
	inode := write(t, i.model, "/app/stable", "kept")
	stop(t, i, contracts.StopGraceful)
	original := s.machines[1].disk
	p = prepare(t, s, ref1, 3, true)
	write(t, p.instance.model, "/app/stable", "lost")
	p.Abort()
	if s.machines[1].disk != original || read(t, original, "/app/stable") != "kept" {
		t.Fatal("aborted restart changed retained disk")
	}
	info, err := s.InspectInode(ref1, inode)
	if err != nil || !info.DiskExists || info.Ops != 0 {
		t.Fatal(info, err)
	}
	if len(ports.draws) != 0 {
		t.Fatal("prepare/abort consumed fault randomness")
	}
	i = start(t, s, ref1, 4, true)
	stop(t, i, contracts.StopCrash)
}

func TestCrashVersusGracefulAndSettleOnce(t *testing.T) {
	t.Parallel()
	for _, kind := range []contracts.StopKind{contracts.StopCrash, contracts.StopGraceful} {
		s, ports := newSession(t)
		i := start(t, s, ref1, 1, false)
		write(t, i.model, "/app/file", "pending")
		i.Quiesce()
		i.Quiesce()
		if err := i.Settle(kind); err != nil {
			t.Fatal(err)
		}
		disk, image := s.machines[1].disk, s.machines[1].crash
		if err := i.Settle(contracts.StopKind(1 - kind)); err != nil || s.machines[1].disk != disk || s.machines[1].crash != image {
			t.Fatal("repeated settle changed outcome")
		}
		i.Detach()
		i.Detach()
		if kind == contracts.StopGraceful {
			if image != nil || read(t, disk, "/app/file") != "pending" {
				t.Fatal("graceful stop did not flush")
			}
		} else {
			if image == nil {
				t.Fatal("crash image missing")
			}
			if _, err := disk.Stat(model.RootInode, "/app/file", model.Identity{}); !errors.Is(err, syscall.ENOENT) {
				t.Fatal("durable-only crash persisted write", err)
			}
		}
		if len(ports.draws) != 0 {
			t.Fatal("durable-only crash consumed randomness")
		}
		s.Close()
	}
}

func TestSamplePendingSelectedAtCrashAndNotRestart(t *testing.T) {
	t.Parallel()
	s, ports := newSession(t)
	if err := s.SetCrashMode(ref1, 1); err != nil {
		t.Fatal(err)
	}
	i := start(t, s, ref1, 1, false)
	write(t, i.model, "/app/file", "pending")
	if len(ports.draws) != 0 {
		t.Fatal("startup consumed randomness")
	}
	stop(t, i, contracts.StopCrash)
	r := ports.draws[ref1]
	if r == nil || r.draws == 0 {
		t.Fatal("crash did not sample pending operations")
	}
	draws := r.draws
	p := prepare(t, s, ref1, 2, true)
	p.Abort()
	i = start(t, s, ref1, 3, true)
	if r.draws != draws {
		t.Fatal("restart reselected crash outcome")
	}
	stop(t, i, contracts.StopCrash)
	if r.draws != draws {
		t.Fatal("empty pending plan consumed randomness")
	}
}

func TestInvalidAndStaleLifecycle(t *testing.T) {
	t.Parallel()
	s, _ := newSession(t)
	if _, err := NewProvider().Open(contracts.Ports{}); !errors.Is(err, contracts.ErrInvalidConfig) {
		t.Fatal(err)
	}
	p := prepare(t, s, ref1, 1, false)
	if _, err := s.Prepare(contracts.StartSpec{Ref: p.instance.ref}, noDescriptors{}); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal("double preparation", err)
	}
	if _, err := p.instance.Getcwd(contracts.CallContext{Epoch: p.instance.ref}, abi.ByteView{}); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal("staged binding accepted a call", err)
	}
	i := p.Publish().(*Instance)
	p.Abort() // no double ownership transfer
	buffer := make([]byte, 32)
	if _, err := i.Getcwd(contracts.CallContext{Epoch: i.ref}, abi.ByteView{Ptr: buffer}); err != nil {
		t.Fatal("published binding failed", err)
	}
	stop(t, i, contracts.StopGraceful)
	if _, err := i.Getcwd(contracts.CallContext{Epoch: i.ref}, abi.ByteView{}); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal("old binding accepted call", err)
	}
	wrong := ref1
	wrong.RunID++
	if err := s.SetCrashMode(wrong, 0); !errors.Is(err, contracts.ErrWrongRun) {
		t.Fatal(err)
	}
	s.Destroy(ref1)
	s.Destroy(ref1)
	if err := s.SetCrashMode(ref1, 0); !errors.Is(err, contracts.ErrMachineNotFound) {
		t.Fatal(err)
	}
	s.Close()
	s.Close()
	if err := s.SetCrashMode(ref1, 0); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal(err)
	}
	if len(s.machines) != 0 || len(s.handles) != 0 || len(s.destroyed) != 0 || s.ports.RandomFor != nil {
		t.Fatal("session retained resources")
	}
}
