package kernel

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	fsprovider "github.com/neria-cloud/gosim/internal/fs"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func word(n int) uintptr { return uintptr(n) }
func fsCall(t *testing.T, k *Kernel, route RouteID, scalars [6]uintptr, args [5]any) Result {
	t.Helper()
	inv := &Invocation{Context: currentContext(k), Route: route, Scalars: scalars, Args: args}
	if err := k.dispatcher.Submit(inv); err != nil {
		t.Fatal(err)
	}
	result := inv.TakeResult()
	if result.Errno != 0 {
		t.Fatalf("route %d: errno %d", route, result.Errno)
	}
	return result
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
			fd := fsCall(t, k, RouteOpenat, [6]uintptr{word(-100), 0x40 | 2, 0o600}, [5]any{"file"}).R0
			if n := fsCall(t, k, RouteWrite, [6]uintptr{fd}, [5]any{abi.ByteView{Ptr: []byte("data")}}).R0; n != 4 {
				t.Fatal(n)
			}
			fsCall(t, k, RouteSeek, [6]uintptr{fd, 0, 0}, [5]any{})
			buffer := make([]byte, 4)
			if n := fsCall(t, k, RouteRead, [6]uintptr{fd}, [5]any{abi.ByteView{Ptr: buffer}}).R0; n != 4 || string(buffer) != "data" {
				t.Fatal(n, string(buffer))
			}
			fsCall(t, k, RoutePwrite, [6]uintptr{fd, 1}, [5]any{abi.ByteView{Ptr: []byte("X")}})
			fsCall(t, k, RoutePread, [6]uintptr{fd, 0}, [5]any{abi.ByteView{Ptr: buffer}})
			if string(buffer) != "dXta" {
				t.Fatal(string(buffer))
			}
			fsCall(t, k, RouteSync, [6]uintptr{fd, 1}, [5]any{})
			stat := fsCall(t, k, RouteFstat, [6]uintptr{fd}, [5]any{}).Value.(contracts.Stat)
			if stat.Size != 4 || stat.Mode != 0o100000 {
				t.Fatal(stat)
			}
			fsCall(t, k, RouteRenameat, [6]uintptr{word(-100), word(-100)}, [5]any{"file", "renamed"})
			fsCall(t, k, RouteStatat, [6]uintptr{word(-100)}, [5]any{"renamed"})
			address := fsCall(t, k, RouteMap, [6]uintptr{0, 4, 1, 1, fd, 0}, [5]any{}).R0
			fsCall(t, k, RouteUnmap, [6]uintptr{address, 4}, [5]any{})
			lock := contracts.Lock{Type: 1, PID: 17}
			fsCall(t, k, RouteFcntlLock, [6]uintptr{fd, 37}, [5]any{abi.NewValueView(&lock)})
			if lock.PID != 17 {
				t.Fatal("lock stub changed payload")
			}
			fsCall(t, k, RouteClose, [6]uintptr{fd}, [5]any{})
			fsCall(t, k, RouteUnlinkat, [6]uintptr{word(-100), 0}, [5]any{"renamed"})
			fsCall(t, k, RouteMkdirat, [6]uintptr{word(-100), 0o700}, [5]any{"sub"})
			fsCall(t, k, RouteChdir, [6]uintptr{}, [5]any{"sub"})
			cwd := make([]byte, 64)
			n := fsCall(t, k, RouteGetcwd, [6]uintptr{}, [5]any{abi.ByteView{Ptr: cwd}}).R0
			if string(cwd[:n]) != "/app/sub\x00" {
				t.Fatal(string(cwd[:n]))
			}
			other := fsCall(t, k, RouteOpenat, [6]uintptr{word(-100), 0x40 | 2, 0o600}, [5]any{"size"}).R0
			fsCall(t, k, RouteFallocate, [6]uintptr{other, 0, 0, 16}, [5]any{})
			fsCall(t, k, RouteTruncate, [6]uintptr{other, 8}, [5]any{})
			fsCall(t, k, RouteFlock, [6]uintptr{other, 2}, [5]any{})
			fsCall(t, k, RouteAdvise, [6]uintptr{1}, [5]any{abi.ByteView{Ptr: cwd}})
			directory := fsCall(t, k, RouteOpenat, [6]uintptr{word(-100), 0, 0}, [5]any{"."}).R0
			n = fsCall(t, k, RouteGetdents64, [6]uintptr{directory}, [5]any{abi.ByteView{Ptr: cwd}}).R0
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

func TestSyncFaultCancelsRealCallerBeforeOrAfterDurability(t *testing.T) {
	for _, before := range []float64{0, 1} {
		t.Run(map[bool]string{true: "before", false: "after"}[before == 1], func(t *testing.T) {
			var returned, deferred atomic.Bool
			_, err := fixture(t, contracts.FeatureSet(contracts.RT|contracts.FS), []contracts.Provider{fsprovider.NewProvider()}, func(k *Kernel) {
				ready := simrt.NewChan[int](0)
				ref := createFor(t, k, "sync-fault", func() {
					defer deferred.Store(true)
					fd := ready.Recv()
					fsCall(t, k, RouteSync, [6]uintptr{uintptr(fd)}, [5]any{})
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
			fd := fsCall(t, k, RouteOpenat, [6]uintptr{word(-100), 0x40 | 2, 0o600}, [5]any{"file"}).R0
			fsCall(t, k, RouteSync, [6]uintptr{fd}, [5]any{})
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
		{name: "boolean", features: contracts.FeatureSet(contracts.RT | contracts.FS), route: RouteSync, scalar: [6]uintptr{1, 2}, want: "not boolean"},
		{name: "disabled", features: contracts.FeatureSet(contracts.RT), route: RouteGetcwd, args: [5]any{abi.ByteView{}}, want: "FeatureDisabled"},
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
