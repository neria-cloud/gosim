package kernel

import (
	"errors"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	fsprovider "github.com/neria-cloud/gosim/internal/fs"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func word(n int) uintptr { return uintptr(n) }
func fsCall[T any](t *testing.T, k *Kernel, request interface{ Invocation() Invocation }, decode func(Result) (T, error)) T {
	return netCall(t, k, request, 0, decode)
}

func TestFilesystemApplicationDispatch(t *testing.T) {
	e := engineFor(t)
	var completed atomic.Bool
	err := e.Run(func() {
		k, err := New(configFor(contracts.FeatureSet(contracts.RT|contracts.FS)), e, []contracts.Provider{fsprovider.NewProvider()})
		if err != nil {
			t.Fatal(err)
		}
		err = k.Run(func() {
			fd := fsCall(t, k, OpenatRequest{DirFD: int(-100), Path: "file", Flags: int(0x40 | 2), Mode: uint32(0o600)}, DecodeOpenatResult).FD
			if n := fsCall(t, k, WriteRequest{FD: int(fd), From: abi.ByteView{Ptr: []byte("data")}}, DecodeWriteResult).Count; n != 4 {
				t.Fatal(n)
			}
			fsCall(t, k, SeekRequest{FD: int(fd), Offset: int64(0), Whence: int(0)}, DecodeSeekResult)
			buffer := make([]byte, 4)
			if n := fsCall(t, k, ReadRequest{FD: int(fd), Into: abi.ByteView{Ptr: buffer}}, DecodeReadResult).Count; n != 4 || string(buffer) != "data" {
				t.Fatal(n, string(buffer))
			}
			fsCall(t, k, PwriteRequest{FD: int(fd), From: abi.ByteView{Ptr: []byte("X")}, Offset: int64(1)}, DecodePwriteResult)
			fsCall(t, k, PreadRequest{FD: int(fd), Into: abi.ByteView{Ptr: buffer}, Offset: int64(0)}, DecodePreadResult)
			if string(buffer) != "dXta" {
				t.Fatal(string(buffer))
			}
			fsCall(t, k, SyncRequest{FD: int(fd), DataOnly: true}, DecodeSyncResult)
			stat := fsCall(t, k, FstatRequest{FD: int(fd)}, DecodeFstatResult).Stat
			if stat.Size != 4 || stat.Mode != 0o100600 || stat.Nlink != 1 {
				t.Fatal(stat)
			}
			fsCall(t, k, RenameatRequest{OldDirFD: int(-100), OldPath: "file", NewDirFD: int(-100), NewPath: "renamed"}, DecodeRenameatResult)
			fsCall(t, k, StatatRequest{DirFD: int(-100), Path: "renamed", Flags: int(0)}, DecodeStatatResult)
			address := fsCall(t, k, MapRequest{Address: uintptr(0), Length: uintptr(4), Protection: int(1), Flags: int(1), FD: int(fd), Offset: int64(0)}, DecodeMapResult).Address
			fsCall(t, k, UnmapRequest{Address: uintptr(address), Length: uintptr(4)}, DecodeUnmapResult)
			lock := contracts.Lock{Type: 1, PID: 17}
			fsCall(t, k, FcntlLockRequest{FD: int(fd), Command: int(37), Lock: abi.NewValueView(&lock)}, DecodeFcntlLockResult)
			if lock.PID != 17 {
				t.Fatal("lock stub changed payload")
			}
			fsCall(t, k, CloseRequest{FD: int(fd)}, DecodeCloseResult)
			fsCall(t, k, UnlinkatRequest{DirFD: int(-100), Path: "renamed", Flags: int(0)}, DecodeUnlinkatResult)
			fsCall(t, k, MkdiratRequest{DirFD: int(-100), Path: "sub", Mode: uint32(0o700)}, DecodeMkdiratResult)
			fsCall(t, k, ChdirRequest{Path: "sub"}, DecodeChdirResult)
			cwd := make([]byte, 64)
			n := fsCall(t, k, GetcwdRequest{Into: abi.ByteView{Ptr: cwd}}, DecodeGetcwdResult).Count
			if string(cwd[:n]) != "/app/sub\x00" {
				t.Fatal(string(cwd[:n]))
			}
			other := fsCall(t, k, OpenatRequest{DirFD: int(-100), Path: "size", Flags: int(0x40 | 2), Mode: uint32(0o600)}, DecodeOpenatResult).FD
			fsCall(t, k, FallocateRequest{FD: int(other), Mode: uint32(0), Offset: int64(0), Length: int64(16)}, DecodeFallocateResult)
			fsCall(t, k, TruncateRequest{FD: int(other), Length: int64(8)}, DecodeTruncateResult)
			fsCall(t, k, FlockRequest{FD: int(other), How: int(2)}, DecodeFlockResult)
			fsCall(t, k, AdviseRequest{Data: abi.ByteView{Ptr: cwd}, Advice: int(1)}, DecodeAdviseResult)
			directory := fsCall(t, k, OpenatRequest{DirFD: int(-100), Path: ".", Flags: int(0), Mode: uint32(0)}, DecodeOpenatResult).FD
			n = fsCall(t, k, Getdents64Request{FD: int(directory), Into: abi.ByteView{Ptr: cwd}}, DecodeGetdents64Result).Count
			if n == 0 {
				t.Fatal("directory syscall returned no entry")
			}
			completed.Store(true)
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := k.dispatcher.Drain(); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || !completed.Load() {
		t.Fatal("application filesystem dispatch failed", err)
	}
}

func TestFilesystemLatencyDelaysCompletionAndClearsBorrowedInput(t *testing.T) {
	const delay = int64(100)
	var inspected atomic.Bool
	_, err := fixture(t, contracts.FeatureSet(contracts.RT|contracts.FS), []contracts.Provider{fsprovider.NewProvider()}, func(k *Kernel) {
		done := simrt.NewChan[bool](0)
		var control contracts.FSControl
		var ref contracts.MachineRef
		ref = createFor(t, k, "delayed-files", func() {
			start := simrt.Nanotime()
			fd := fsCall(t, k, OpenatRequest{DirFD: -100, Path: "delayed", Flags: 0x40 | 2, Mode: 0o600}, DecodeOpenatResult).FD
			if elapsed := simrt.Nanotime() - start; elapsed != delay {
				t.Fatalf("create delay = %d", elapsed)
			}
			start = simrt.Nanotime()
			existing := fsCall(t, k, OpenatRequest{DirFD: -100, Path: "delayed", Flags: 0x40 | 2, Mode: 0o600}, DecodeOpenatResult).FD
			if elapsed := simrt.Nanotime() - start; elapsed != 0 {
				t.Fatalf("existing O_CREAT delay = %d", elapsed)
			}
			fsCall(t, k, CloseRequest{FD: existing}, DecodeCloseResult)
			start = simrt.Nanotime()
			fsCall(t, k, FstatRequest{FD: fd}, DecodeFstatResult)
			if elapsed := simrt.Nanotime() - start; elapsed != 0 {
				t.Fatalf("metadata read delay = %d", elapsed)
			}
			inv := WriteRequest{FD: fd, From: abi.ByteView{Ptr: []byte("x")}}.Invocation()
			inv.Context = currentContext(k)
			if _, err := k.Ports().Events.Schedule(inv.Context.Epoch, simrt.Nanotime()+delay/2, func() {
				if inv.Args != ([5]any{}) || len(k.dispatcher.timers) != 1 {
					t.Error("delayed completion retained borrowed arguments or lost timer")
				}
				inspected.Store(true)
			}); err != nil {
				t.Fatal(err)
			}
			start = simrt.Nanotime()
			if err := k.dispatcher.Submit(&inv); err != nil {
				t.Fatal(err)
			}
			if result, err := DecodeWriteResult(inv.TakeResult()); err != nil || result.Count != 1 {
				t.Fatalf("delayed write: %+v, %v", result, err)
			}
			if elapsed := simrt.Nanotime() - start; elapsed != delay {
				t.Fatalf("write delay = %d", elapsed)
			}
			buffer := make([]byte, 1)
			start = simrt.Nanotime()
			if n := fsCall(t, k, PreadRequest{FD: fd, Into: abi.ByteView{Ptr: buffer}, Offset: 0}, DecodePreadResult).Count; n != 1 || string(buffer) != "x" {
				t.Fatalf("delayed read: n=%d data=%q", n, buffer)
			}
			if elapsed := simrt.Nanotime() - start; elapsed != delay {
				t.Fatalf("read delay = %d", elapsed)
			}
			start = simrt.Nanotime()
			netCall(t, k, PwriteRequest{FD: fd, From: abi.ByteView{Ptr: []byte("y")}, Offset: 1024}, syscall.ENOSPC, DecodePwriteResult)
			if elapsed := simrt.Nanotime() - start; elapsed != delay {
				t.Fatalf("ENOSPC delay = %d", elapsed)
			}
			if err := control.SetReadOnly(ref, true); err != nil {
				t.Fatal(err)
			}
			start = simrt.Nanotime()
			netCall(t, k, PwriteRequest{FD: fd, From: abi.ByteView{Ptr: []byte("z")}, Offset: 0}, syscall.EROFS, DecodePwriteResult)
			if elapsed := simrt.Nanotime() - start; elapsed != delay {
				t.Fatalf("EROFS delay = %d", elapsed)
			}
			start = simrt.Nanotime()
			fsCall(t, k, WriteRequest{FD: 1, From: abi.ByteView{Ptr: []byte("diagnostic")}}, DecodeWriteResult)
			if elapsed := simrt.Nanotime() - start; elapsed != 0 {
				t.Fatalf("non-filesystem write delay = %d", elapsed)
			}
			done.Send(true)
		})
		var err error
		control, err = k.FilesystemControl(contracts.CallContext{}, ref.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if err := control.ConfigureMachine(ref, contracts.FSConfig{WorkingDir: "/app", CapacityBytes: 2 * 1024, LatencyNS: delay}); err != nil {
			t.Fatal(err)
		}
		if err := k.Start(ref); err != nil {
			t.Fatal(err)
		}
		if err := k.Run(func() { done.Recv() }); err != nil {
			t.Fatal(err)
		}
		if err := k.dispatcher.Drain(); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || !inspected.Load() {
		t.Fatalf("filesystem latency: %v inspected=%v", err, inspected.Load())
	}
}

func TestFilesystemDelayedCompletionCanceledOnStop(t *testing.T) {
	var canceled, resumed atomic.Bool
	k, err := fixture(t, contracts.FeatureSet(contracts.RT|contracts.FS), []contracts.Provider{fsprovider.NewProvider()}, func(k *Kernel) {
		var ref contracts.MachineRef
		ref = createFor(t, k, "cancel-delayed-file", func() {
			fd := fsCall(t, k, OpenatRequest{DirFD: -100, Path: "file", Flags: 0x40 | 2, Mode: 0o600}, DecodeOpenatResult).FD
			inv := WriteRequest{FD: fd, From: abi.ByteView{Ptr: []byte("data")}}.Invocation()
			inv.Context = currentContext(k)
			if _, err := k.Ports().Events.Schedule(inv.Context.Epoch, simrt.Nanotime()+50, func() {
				if inv.Args != ([5]any{}) || len(k.dispatcher.timers) != 1 {
					t.Error("pending write retained borrowed input or lost timer")
				}
				if err := k.Stop(ref, contracts.StopCrash); err != nil {
					t.Error(err)
				}
				if len(k.dispatcher.timers) != 0 || len(k.dispatcher.live) != 0 {
					t.Error("stopped epoch retained delayed completion")
				}
				canceled.Store(true)
			}); err != nil {
				t.Fatal(err)
			}
			_ = k.dispatcher.Submit(&inv)
			resumed.Store(true)
		})
		control, err := k.FilesystemControl(contracts.CallContext{}, ref.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if err := control.ConfigureMachine(ref, contracts.FSConfig{WorkingDir: "/app", LatencyNS: 100}); err != nil {
			t.Fatal(err)
		}
		if err := k.Start(ref); err != nil {
			t.Fatal(err)
		}
		if err := k.Run(func() { simrt.Sleep(500) }); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || !canceled.Load() || resumed.Load() || len(k.dispatcher.timers) != 0 {
		t.Fatalf("delayed cancellation: %v canceled=%v resumed=%v", err, canceled.Load(), resumed.Load())
	}
}

func TestSyncFaultCancelsRealCallerBeforeOrAfterDurability(t *testing.T) {
	for _, before := range []float64{0, 1} {
		t.Run(map[bool]string{true: "before", false: "after"}[before == 1], func(t *testing.T) {
			var returned, deferred atomic.Bool
			_, err := fixture(t, contracts.FeatureSet(contracts.RT|contracts.FS), []contracts.Provider{fsprovider.NewProvider()}, func(k *Kernel) {
				ready := simrt.NewChan[int](0)
				ref := createFor(t, k, "sync-fault", func() {
					defer deferred.Store(true)
					fd := ready.Recv()
					fsCall(t, k, SyncRequest{FD: int(uintptr(fd)), DataOnly: false}, DecodeSyncResult)
					returned.Store(true)
				})
				if err := k.Start(ref); err != nil {
					t.Fatal(err)
				}
				ctx := contracts.CallContext{Epoch: k.machineByID[ref.ID].current.ref}
				files, err := k.Filesystem(ctx)
				if err != nil {
					t.Fatal(err)
				}
				fd, err := files.Openat(ctx, -100, "file", 0x40|2, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := files.Pwrite(ctx, fd, abi.ByteView{Ptr: []byte("base")}, 0); err != nil {
					t.Fatal(err)
				}
				if err := files.Sync(ctx, fd, false); err != nil {
					t.Fatal(err)
				}
				dir, err := files.Openat(ctx, -100, ".", 0, 0)
				if err != nil {
					t.Fatal(err)
				}
				if err := files.Sync(ctx, dir, false); err != nil {
					t.Fatal(err)
				}
				if _, err := files.Pwrite(ctx, fd, abi.ByteView{Ptr: []byte("next")}, 0); err != nil {
					t.Fatal(err)
				}
				controls, err := k.FilesystemControl(contracts.CallContext{}, ref.RunID)
				if err != nil {
					t.Fatal(err)
				}
				if err := controls.SetSyncFault(ref, contracts.FSFaultConfig{Probability: 1, BeforeProbability: before}); err != nil {
					t.Fatal(err)
				}
				// Receive the application's actual blocked invocation and let the same
				// dispatcher used by Kernel.Run consume the provider crash outcome.
				ready.Send(fd)
				inv := k.dispatcher.incoming.Recv()
				k.dispatcher.DispatchOne(inv)
				if k.machineByID[ref.ID].state != StateCrashed || returned.Load() || deferred.Load() {
					t.Fatal("fault did not cancel application")
				}
				if err := k.dispatcher.Drain(); err != nil {
					t.Fatal(err)
				}
				if err := k.Restart(ref); err != nil {
					t.Fatal(err)
				}
				ctx = contracts.CallContext{Epoch: k.machineByID[ref.ID].current.ref}
				files, err = k.Filesystem(ctx)
				if err != nil {
					t.Fatal(err)
				}
				newFD, err := files.Openat(ctx, -100, "file", 0, 0)
				if err != nil {
					t.Fatal(err)
				}
				data := make([]byte, 4)
				if _, err := files.Pread(ctx, newFD, abi.ByteView{Ptr: data}, 0); err != nil {
					t.Fatal(err)
				}
				want := "next"
				if before == 1 {
					want = "base"
				}
				if string(data) != want {
					t.Fatalf("fault phase persisted %q, want %q", data, want)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

type (
	faultOnPrepareProvider struct{ contracts.Provider }
	faultOnPrepareSession  struct{ contracts.ProviderSession }
)

func (p faultOnPrepareProvider) Open(ports contracts.Ports) (contracts.ProviderSession, error) {
	s, err := p.Provider.Open(ports)
	if err != nil {
		return nil, err
	}
	return faultOnPrepareSession{s}, nil
}

func (s faultOnPrepareSession) Prepare(spec contracts.StartSpec, fds contracts.DescriptorTable) (contracts.PreparedMachine, error) {
	if err := s.Controls().FSControl.SetSyncFault(spec.Ref.Machine, contracts.FSFaultConfig{Probability: 1, BeforeProbability: 1}); err != nil {
		return nil, err
	}
	return s.ProviderSession.Prepare(spec, fds)
}

func TestRootSyncFaultTerminatesRunWithoutResuming(t *testing.T) {
	e := engineFor(t)
	var returned, deferred atomic.Bool
	err := e.Run(func() {
		k, err := New(configFor(contracts.FeatureSet(contracts.RT|contracts.FS)), e, []contracts.Provider{faultOnPrepareProvider{fsprovider.NewProvider()}})
		if err != nil {
			t.Fatal(err)
		}
		err = k.Run(func() {
			defer deferred.Store(true)
			fd := fsCall(t, k, OpenatRequest{DirFD: int(-100), Path: "file", Flags: int(0x40 | 2), Mode: uint32(0o600)}, DecodeOpenatResult).FD
			fsCall(t, k, SyncRequest{FD: int(fd), DataOnly: false}, DecodeSyncResult)
			returned.Store(true)
		})
		if err != nil {
			t.Fatal(err)
		}
		if !k.closed || k.machineByID[k.root].lastExit.Kind != contracts.StopCrash {
			t.Fatal("root crash did not end run")
		}
		if err := k.dispatcher.Drain(); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || returned.Load() || deferred.Load() {
		t.Fatal("root syscall resumed", err)
	}
}

func TestFilesystemMalformedPayloadsAndDisabledFeature(t *testing.T) {
	for _, test := range []struct {
		name     string
		features contracts.FeatureSet
		route    RouteID
		args     [5]any
		scalar   [6]uintptr
		want     string
	}{
		{name: "callable", features: contracts.FeatureSet(contracts.RT | contracts.FS), route: RouteOpenat, args: [5]any{func() {}}, want: "InvalidABI"},
		{name: "extra", features: contracts.FeatureSet(contracts.RT | contracts.FS), route: RouteFstat, args: [5]any{"extra"}, want: "InvalidABI"},
		{name: "boolean", features: contracts.FeatureSet(contracts.RT | contracts.FS), route: RouteSync, scalar: [6]uintptr{1, 2}, want: "InvalidABI"},
		{name: "disabled", features: contracts.FeatureSet(contracts.RT), route: RouteGetcwd, args: [5]any{GetcwdRequest{}}, want: "FeatureDisabled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var providers []contracts.Provider
			if test.features.Has(contracts.FS) {
				providers = []contracts.Provider{fsprovider.NewProvider()}
			}
			_, err := fixture(t, test.features, providers, func(k *Kernel) {
				ref := createFor(t, k, "malformed", idleMain)
				if err := k.Start(ref); err != nil {
					t.Fatal(err)
				}
				k.dispatcher.DispatchOne(&Invocation{Context: contracts.CallContext{Epoch: k.machineByID[ref.ID].current.ref}, Route: test.route, Args: test.args, Scalars: test.scalar})
			})
			var abort *simrt.AbortError
			if !errors.As(err, &abort) || !strings.Contains(err.Error(), test.want) {
				t.Fatal("invalid syscall did not abort", err)
			}
		})
	}
}
