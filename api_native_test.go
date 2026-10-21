//go:build !sim

package gosim_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	simfs "github.com/neria-cloud/gosim/fs"
	simnet "github.com/neria-cloud/gosim/net"
	"github.com/neria-cloud/gosim/rt"
)

func TestNativeControls(t *testing.T) {
	t.Parallel()
	m := gosim.CurrentMachine()
	if gosim.IsSim() || m.ID() != 0 || m.RunID() != 0 || !reflect.DeepEqual(rt.Info(), rt.RunInfo{}) {
		t.Fatal("native run metadata")
	}
	var iterator *simfs.CrashIterator
	monitor := m.SyscallsMonitor()
	if monitor == nil {
		t.Fatal("native accessor returned nil")
	}
	for name, call := range map[string]func() error{
		"syscalls-start": monitor.Start, "syscalls-stop": monitor.Stop, "syscalls-reset": monitor.Reset,
		"syscalls-counts": func() error {
			result, err := monitor.Counts()
			if result != nil {
				t.Error("native counts")
			}
			return err
		},
		"syscalls-stats": func() error {
			result, err := monitor.Stats()
			if result != nil {
				t.Error("native stats")
			}
			return err
		},
		"syscalls-list": func() error {
			result, err := gosim.CountableSyscalls()
			if result != nil {
				t.Error("native list")
			}
			return err
		},
		"new":   func() error { _, e := gosim.NewMachine(gosim.MachineConfig{}); return e },
		"label": func() error { _, e := m.Label(); return e }, "state": func() error { _, e := m.State(); return e },
		"start": m.Start, "stop": m.Stop, "crash": m.Crash, "restart": m.Restart, "destroy": m.Destroy,
		"main": func() error { return m.SetMain(nil) }, "wait": func() error { _, e := m.Wait(); return e },
		"fs-config": func() error { return simfs.ConfigureMachine(m, simfs.Config{}) },
		"fs-crash":  func() error { return simfs.SetCrashMode(m, 255) },
		"fs-sync":   func() error { return simfs.SetSyncFault(m, simfs.SyncFault{Probability: -1}) },
		"inode":     func() error { _, e := simfs.InspectInode(m, -1); return e },
		"enumerate": func() error { _, e := simfs.CrashStates(m, simfs.EnumerateOptions{}); return e },
		"next":      func() error { _, _, e := iterator.Next(); return e }, "iterator-close": iterator.Close,
		"restore": func() error { return simfs.Restore(m, simfs.Snapshot{}) }, "snapshot-close": (simfs.Snapshot{}).Close,
		"net-config":  func() error { return simnet.Configure(simnet.Config{}) },
		"net-machine": func() error { return simnet.ConfigureMachine(m, simnet.MachineConfig{}) },
		"address":     func() error { _, e := simnet.Address(m); return e },
		"policy":      func() error { return simnet.SetLinkPolicy(simnet.Link{}, simnet.LinkPolicy{}) },
		"reset":       func() error { return simnet.ResetLinkPolicy(simnet.Link{}) },
		"partition":   func() error { return simnet.Partition(simnet.Link{}) }, "repair": func() error { return simnet.Repair(simnet.Link{}) },
		"timeout": func() error { return rt.SetTimeout(-1) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, gosim.ErrNotSim) {
				t.Fatal(err)
			}
		})
	}
	for name, call := range map[string]func(){"exit": gosim.ExitSelf, "crash": gosim.CrashSelf} {
		t.Run(name+"-self", func(t *testing.T) {
			defer func() {
				e, ok := recover().(error)
				if !ok || !errors.Is(e, gosim.ErrNotSim) {
					t.Fatalf("panic: %v", e)
				}
			}()
			call()
			t.Fatal("returned")
		})
	}
}

func TestControlDefaults(t *testing.T) {
	t.Parallel()
	c := simnet.DefaultConfig()
	if c.Latency.Min != 0 || c.Latency.Max != 100*time.Millisecond || c.Latency.Lambda != 5 || c.Loss.FailProbability != 0 || c.Loss.RepairProbability != 1 {
		t.Fatal(c)
	}
	c.Latency.Lambda = 1
	if simnet.DefaultConfig().Latency.Lambda != 5 || simfs.DefaultConfig().WorkingDir != "/app" {
		t.Fatal("defaults")
	}
}
