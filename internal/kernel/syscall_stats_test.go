package kernel

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func counterDispatch(k *Kernel, ref contracts.MachineRef, request interface{ Invocation() Invocation }) Result {
	inv := request.Invocation()
	inv.Context.Epoch = k.machineByID[ref.ID].current.ref
	k.dispatcher.DispatchOne(&inv)
	return inv.TakeResult()
}

func counterRows(t *testing.T, k *Kernel, ref contracts.MachineRef) []contracts.SyscallRecord {
	t.Helper()
	rows, err := k.SyscallStats(ref)
	if err != nil {
		t.Fatal(err)
	}
	for slot, counter := range k.machineByID[ref.ID].syscalls.values {
		var sum uint64
		for _, count := range counter.byDescriptor {
			sum += count
		}
		if sum != counter.total {
			t.Fatalf("slot %d: total %d != buckets %d", slot, counter.total, sum)
		}
	}
	return rows
}

func counterCheck(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSyscallCounterLifecycle(t *testing.T) {
	_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
		ref := createFor(t, k, "counted", idleMain)
		other := createFor(t, k, "independent", idleMain)
		m := k.machineByID[ref.ID]
		if m.syscalls.enabled || len(counterRows(t, k, ref)) != 0 {
			t.Fatal("new machine has monitoring state")
		}
		counterCheck(t, k.SetSyscallCounting(ref, false))
		counterCheck(t, k.ResetSyscallStats(ref))
		info, err := k.CountableSyscalls(ref)
		counterCheck(t, err)
		counterCheck(t, k.Start(ref))
		counterDispatch(k, ref, GetpidRequest{})
		if len(counterRows(t, k, ref)) != 0 {
			t.Fatal("disabled attempt counted")
		}
		for range 2 {
			counterCheck(t, k.SetSyscallCounting(ref, true))
			counterDispatch(k, ref, GetpidRequest{})
		}
		for range 2 {
			counterCheck(t, k.SetSyscallCounting(ref, false))
		}
		counterDispatch(k, ref, GetpidRequest{})
		want := []contracts.SyscallRecord{{Name: "getpid", Group: "rt", DescriptorGroup: "none", Count: 2}}
		if got := counterRows(t, k, ref); !reflect.DeepEqual(got, want) {
			t.Fatalf("paused counts: %+v", got)
		}
		copy := counterRows(t, k, ref)
		copy[0].Count, copy[0].Name = 99, "modified"
		info[0].Name = "modified"
		if got := counterRows(t, k, ref); !reflect.DeepEqual(got, want) {
			t.Fatal("snapshot aliases state", got)
		}
		fresh, err := k.CountableSyscalls(ref)
		counterCheck(t, err)
		if fresh[0].Name == "modified" || len(counterRows(t, k, other)) != 0 {
			t.Fatal("catalogue aliases output or machines share counters")
		}
		counterCheck(t, k.ResetSyscallStats(ref))
		counterDispatch(k, ref, GetpidRequest{})
		if m.syscalls.enabled || len(counterRows(t, k, ref)) != 0 {
			t.Fatal("reset enabled a paused counter")
		}
		counterCheck(t, k.SetSyscallCounting(ref, true))
		counterDispatch(k, ref, GetpidRequest{})
		counterCheck(t, k.ResetSyscallStats(ref))
		counterCheck(t, k.ResetSyscallStats(ref))
		counterDispatch(k, ref, GetpidRequest{})
		if !m.syscalls.enabled || counterRows(t, k, ref)[0].Count != 1 {
			t.Fatal("enabled reset did not resume counting")
		}
		for _, kind := range []contracts.StopKind{contracts.StopGraceful, contracts.StopCrash} {
			epoch := m.current.ref
			before := counterRows(t, k, ref)
			counterCheck(t, k.Stop(ref, kind))
			if !reflect.DeepEqual(counterRows(t, k, ref), before) || !m.syscalls.enabled {
				t.Fatal("stop/cleanup changed statistics")
			}
			counterCheck(t, k.Restart(ref))
			if m.current.ref == epoch || !m.syscalls.enabled {
				t.Fatal("restart lost machine state")
			}
			counterDispatch(k, ref, GetpidRequest{})
			if counterRows(t, k, ref)[0].Count != before[0].Count+1 {
				t.Fatal("restart did not resume")
			}
		}
		counterCheck(t, k.Stop(ref, contracts.StopCrash))
		counterCheck(t, k.Destroy(ref))
		for _, test := range []struct {
			ref  contracts.MachineRef
			want error
		}{{ref, contracts.ErrMachineNotFound}, {contracts.MachineRef{}, contracts.ErrWrongRun}, {contracts.MachineRef{RunID: ref.RunID + 1, ID: other.ID}, contracts.ErrWrongRun}} {
			for _, call := range []func() error{
				func() error { return k.SetSyscallCounting(test.ref, true) },
				func() error { return k.SetSyscallCounting(test.ref, false) },
				func() error { return k.ResetSyscallStats(test.ref) },
				func() error {
					rows, err := k.SyscallStats(test.ref)
					if rows != nil {
						t.Error("error returned rows")
					}
					return err
				},
				func() error {
					rows, err := k.CountableSyscalls(test.ref)
					if rows != nil {
						t.Error("error returned catalogue")
					}
					return err
				},
			} {
				if err := call(); !errors.Is(err, test.want) {
					t.Fatalf("invalid handle: %v want %v", err, test.want)
				}
			}
		}
	})
	counterCheck(t, err)
}

type counterFileOps struct{ contracts.FileOps }

func (*counterFileOps) StorageDelayNS(contracts.CallContext) int64 { return 0 }
func (*counterFileOps) Fstat(contracts.CallContext, int) (contracts.Stat, error) {
	return contracts.Stat{}, syscall.EBADF
}
func (*counterFileOps) Sync(contracts.CallContext, int, bool) error { return syscall.EBADF }
func (*counterFileOps) FcntlLock(contracts.CallContext, int, int, *contracts.Lock) error {
	return syscall.EBADF
}

func TestSyscallCounterOwnersAndAliases(t *testing.T) {
	var log []string
	_, _, providers := providersFor(&log)
	_, err := fixture(t, contracts.FeatureSet(contracts.RT|contracts.FS|contracts.Net), providers, func(k *Kernel) {
		ref := createFor(t, k, "owners", idleMain)
		counterCheck(t, k.Start(ref))
		m := k.machineByID[ref.ID]
		m.current.bindings.Files = &counterFileOps{}
		counterCheck(t, k.SetSyscallCounting(ref, true))
		for _, owner := range []contracts.Feature{contracts.RT, contracts.FS, contracts.Net} {
			fd, err := m.current.descriptors.Allocate(&dataResource{owner: owner, readErr: syscall.EAGAIN})
			counterCheck(t, err)
			for range 2 {
				result := counterDispatch(k, ref, ReadRequest{FD: fd})
				if result.Errno != uintptr(syscall.EAGAIN) {
					t.Fatal("lost errno", result)
				}
			}
			if owner == contracts.Net {
				counterDispatch(k, ref, FstatRequest{FD: fd})
			}
			counterDispatch(k, ref, CloseRequest{FD: fd})
			counterDispatch(k, ref, CloseRequest{FD: fd})
		}
		counterDispatch(k, ref, ReadRequest{FD: 999})
		counterDispatch(k, ref, GetpidRequest{})
		counterDispatch(k, ref, SyncRequest{FD: 999})
		counterDispatch(k, ref, SyncRequest{FD: 999, DataOnly: true})
		counterDispatch(k, ref, FcntlIntRequest{FD: 999})
		lock := contracts.Lock{}
		counterDispatch(k, ref, FcntlLockRequest{FD: 999, Lock: abi.NewValueView(&lock)})
		before := counterRows(t, k, ref)
		// Queries and a poll request must not observe themselves.
		counterDispatch(k, ref, SyscallStatsRequest{Machine: ref})
		counterDispatch(k, ref, CountableSyscallsRequest{Machine: ref})
		counterDispatch(k, ref, SetSyscallCountingRequest{Machine: ref, Enabled: true})
		counterDispatch(k, ref, PollOpenRequest{FD: 0})
		got := counterRows(t, k, ref)
		if !reflect.DeepEqual(got, before) {
			t.Fatal("controls/poll counted", got)
		}
		want := []contracts.SyscallRecord{
			{"read", "rt", true, "rt", 2},
			{"read", "rt", true, "fs", 2},
			{"read", "rt", true, "net", 2},
			{"read", "rt", true, "unknown", 1},
			{"close", "rt", true, "rt", 1},
			{"close", "rt", true, "fs", 1},
			{"close", "rt", true, "net", 1},
			{"close", "rt", true, "unknown", 3},
			{"fcntl", "rt", true, "unknown", 2},
			{"getpid", "rt", false, "none", 1},
			{"fsync", "fs", true, "unknown", 1},
			{"fstat", "fs", true, "net", 1},
			{"fdatasync", "fs", true, "unknown", 1},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rows:\n%+v\nwant:\n%+v", got, want)
		}
	})
	counterCheck(t, err)
}

func TestSyscallCounterFailedStartAndDisabledFastPath(t *testing.T) {
	var log []string
	fs, _, _ := providersFor(&log)
	_, err := fixture(t, contracts.FeatureSet(contracts.RT|contracts.FS), []contracts.Provider{fs}, func(k *Kernel) {
		ref := createFor(t, k, "prepare", idleMain)
		counterCheck(t, k.SetSyscallCounting(ref, true))
		fs.prepareErr = errInjected
		if err := k.Start(ref); !errors.Is(err, errInjected) {
			t.Fatal(err)
		}
		m := k.machineByID[ref.ID]
		if !m.syscalls.enabled || len(counterRows(t, k, ref)) != 0 {
			t.Fatal("failed start changed counters")
		}
		fs.prepareErr = nil
		counterCheck(t, k.Start(ref))
		counterDispatch(k, ref, GetpidRequest{})
		counterCheck(t, k.Stop(ref, contracts.StopCrash))
		fs.prepareErr = errInjected
		if err := k.Restart(ref); !errors.Is(err, errInjected) {
			t.Fatal(err)
		}
		if !m.syscalls.enabled || counterRows(t, k, ref)[0].Count != 1 {
			t.Fatal("failed restart lost counts")
		}
		// A disabled counter must not access descriptor state, even before start.
		counterCheck(t, k.SetSyscallCounting(ref, false))
		inv := ReadRequest{FD: 999}.Invocation()
		inv.Context.Epoch.Machine = ref
		k.countSyscall(&inv)
	})
	counterCheck(t, err)
}

func TestSyscallCounterFatalBoundaries(t *testing.T) {
	for _, failure := range []string{"overflow", "owner", "payload", "unknown", "disabled", "stale"} {
		t.Run(failure, func(t *testing.T) {
			var m *Machine
			_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
				ref := createFor(t, k, "fatal", idleMain)
				counterCheck(t, k.Start(ref))
				counterCheck(t, k.SetSyscallCounting(ref, true))
				m = k.machineByID[ref.ID]
				inv := GetpidRequest{}.Invocation()
				inv.Context.Epoch = m.current.ref
				switch failure {
				case "overflow":
					m.syscalls.values[RouteGetpid] = syscallCounter{total: math.MaxUint64, byDescriptor: [contracts.DescriptorGroupLimit]uint64{math.MaxUint64}}
				case "owner":
					m.current.descriptors.entries[999] = &dataResource{owner: 255}
					inv.Route, inv.Args[0] = RouteRead, ReadRequest{FD: 999}
				case "payload":
					inv.Route = RouteSync // wrong typed payload must not reach its type assertion
				case "unknown":
					inv.Route = syscallDataSyncSlot
				case "disabled":
					inv.Route, inv.Args[0] = RouteGetcwd, GetcwdRequest{}
				case "stale":
					inv.Context.Epoch.Epoch++
				}
				k.dispatcher.DispatchOne(&inv)
				t.Error("fatal request returned")
			})
			want := map[string]string{"overflow": "counter overflow", "owner": "descriptor owner", "payload": "InvalidABI", "unknown": "UnsupportedOperation", "disabled": "FeatureDisabled", "stale": "admission"}[failure]
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("error %v want %s", err, want)
			}
			for slot, counter := range m.syscalls.values {
				want := syscallCounter{}
				if failure == "overflow" && RouteID(slot) == RouteGetpid {
					want = syscallCounter{total: math.MaxUint64, byDescriptor: [contracts.DescriptorGroupLimit]uint64{math.MaxUint64}}
				}
				if counter != want {
					t.Fatalf("failure changed slot %d: %+v", slot, counter)
				}
			}
		})
	}
}

func TestSyscallCounterRootTransport(t *testing.T) {
	e := engineFor(t)
	err := e.Run(func() {
		k, err := New(configFor(contracts.FeatureSet(contracts.RT)), e, nil)
		counterCheck(t, err)
		counterCheck(t, k.Run(func() {
			ref := contracts.MachineRef{RunID: k.runID, ID: simrt.CurrentMachineToken().ID}
			fsCall(t, k, SetSyscallCountingRequest{Machine: ref, Enabled: true}, DecodeSetSyscallCountingResult)
			fsCall(t, k, GetpidRequest{}, DecodeGetpidResult)
			rows := fsCall(t, k, SyscallStatsRequest{Machine: ref}, DecodeSyscallStatsResult).Records
			if !reflect.DeepEqual(rows, []contracts.SyscallRecord{{Name: "getpid", Group: "rt", DescriptorGroup: "none", Count: 1}}) {
				t.Fatal(rows)
			}
			fsCall(t, k, ResetSyscallStatsRequest{Machine: ref}, DecodeResetSyscallStatsResult)
			if rows := fsCall(t, k, SyscallStatsRequest{Machine: ref}, DecodeSyscallStatsResult).Records; len(rows) != 0 {
				t.Fatal(rows)
			}
		}))
	})
	counterCheck(t, err)
}
