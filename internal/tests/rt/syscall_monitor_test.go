//go:build sim

package behavior_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
)

func monitorCounts(t *testing.T, monitor gosim.SyscallsMonitor) map[string]uint64 {
	t.Helper()
	counts, err := monitor.Counts()
	testsupport.Check(t, err)
	if counts == nil {
		t.Fatal("nil successful counts")
	}
	return counts
}

func monitorStats(t *testing.T, monitor gosim.SyscallsMonitor) map[string]gosim.SyscallStat {
	t.Helper()
	stats, err := monitor.Stats()
	testsupport.Check(t, err)
	if stats == nil {
		t.Fatal("nil successful stats")
	}
	for name, stat := range stats {
		var sum uint64
		for _, count := range stat.ByDescriptor {
			if count == 0 {
				t.Fatal("zero bucket", name)
			}
			sum += count
		}
		if stat.Count == 0 || stat.Count != sum {
			t.Fatal("invalid total", name, stat)
		}
	}
	return stats
}

func TestSyscallMonitorControlsAndCopies(t *testing.T) {
	m := gosim.CurrentMachine()
	monitor, shared := m.SyscallsMonitor(), m.SyscallsMonitor()
	if monitor == nil || shared == nil {
		t.Fatal("nil monitor")
	}
	if counts, stats := monitorCounts(t, monitor), monitorStats(t, monitor); len(counts) != 0 || len(stats) != 0 {
		t.Fatal("not initially empty", counts, stats)
	}
	list, err := monitor.List()
	testsupport.Check(t, err)
	if len(list) < 8 || !slices.IsSortedFunc(list, func(a, b gosim.SyscallInfo) int { return strings.Compare(a.Name, b.Name) }) {
		t.Fatal("invalid catalogue", list)
	}
	originalList := slices.Clone(list)
	for i, info := range list {
		if info.Name == "" || i > 0 && list[i-1].Name == info.Name {
			t.Fatal("invalid name", list)
		}
	}
	_ = syscall.Getpid()
	if len(monitorCounts(t, monitor)) != 0 {
		t.Fatal("accessor/query enabled monitoring")
	}
	for range 2 {
		testsupport.Check(t, monitor.Start())
	}
	_ = syscall.Getpid()
	_ = syscall.Getpid()
	for range 2 {
		testsupport.Check(t, shared.Stop())
	}
	_ = syscall.Getpid()
	wantCounts := map[string]uint64{"getpid": 2}
	wantStats := map[string]gosim.SyscallStat{"getpid": {Count: 2, Group: gosim.SyscallGroupRT, ByDescriptor: map[gosim.DescriptorGroup]uint64{gosim.DescriptorGroupNone: 2}}}
	counts, stats := monitorCounts(t, monitor), monitorStats(t, shared)
	if !reflect.DeepEqual(counts, wantCounts) || !reflect.DeepEqual(stats, wantStats) {
		t.Fatal(counts, stats)
	}
	counts["getpid"], counts["never"] = 999, 1
	stats["getpid"].ByDescriptor[gosim.DescriptorGroupNone] = 999
	stats["never"] = gosim.SyscallStat{Count: 999}
	list[0].Name = "modified"
	freshList, err := shared.List()
	testsupport.Check(t, err)
	if !reflect.DeepEqual(freshList, originalList) || !reflect.DeepEqual(monitorCounts(t, shared), wantCounts) || !reflect.DeepEqual(monitorStats(t, monitor), wantStats) {
		t.Fatal("snapshot aliases state")
	}
	testsupport.Check(t, shared.Start())
	_ = syscall.Getpid()
	if monitorCounts(t, monitor)["getpid"] != 3 {
		t.Fatal("start did not resume")
	}
	for range 2 {
		testsupport.Check(t, shared.Reset())
	}
	if len(monitorStats(t, monitor)) != 0 {
		t.Fatal("reset left counts")
	}
	_ = syscall.Getpid()
	if monitorCounts(t, monitor)["getpid"] != 1 {
		t.Fatal("reset disabled counting")
	}
	testsupport.Check(t, monitor.Stop())
	testsupport.Check(t, monitor.Reset())
	_ = syscall.Getpid()
	if len(monitorCounts(t, monitor)) != 0 {
		t.Fatal("reset enabled counting")
	}
}

func checkInvalidMonitor(t *testing.T, monitor gosim.SyscallsMonitor, want error) {
	t.Helper()
	if monitor == nil {
		t.Fatal("accessor validated the handle")
	}
	for _, call := range []func() error{
		monitor.Start, monitor.Stop, monitor.Reset,
		func() error {
			rows, err := monitor.Counts()
			if rows != nil {
				t.Error("error returned counts")
			}
			return err
		},
		func() error {
			rows, err := monitor.Stats()
			if rows != nil {
				t.Error("error returned stats")
			}
			return err
		},
		func() error {
			rows, err := monitor.List()
			if rows != nil {
				t.Error("error returned list")
			}
			return err
		},
	} {
		if err := call(); !errors.Is(err, want) {
			t.Fatalf("monitor error %v want %v", err, want)
		}
	}
}

func TestSyscallMonitorPrestartAndInvalidHandles(t *testing.T) {
	m, err := gosim.NewMachine(gosim.MachineConfig{Main: func() { _ = syscall.Getpid() }})
	testsupport.Check(t, err)
	monitor := m.SyscallsMonitor()
	if len(monitorCounts(t, monitor)) != 0 || len(monitorStats(t, monitor)) != 0 {
		t.Fatal("prestart counts")
	}
	if entries, err := monitor.List(); err != nil || len(entries) == 0 {
		t.Fatal(entries, err)
	}
	testsupport.Check(t, monitor.Start())
	testsupport.Check(t, monitor.Stop())
	testsupport.Check(t, monitor.Reset())
	testsupport.Check(t, monitor.Start())
	testsupport.Check(t, m.Start())
	testsupport.Wait(t, m)
	if monitorCounts(t, monitor)["getpid"] != 1 {
		t.Fatal("prestart counting missed workload")
	}
	// The existing globalHostname initializer calls os.Hostname on every boot.
	if monitorCounts(t, monitor)["uname"] == 0 {
		t.Fatal("prestart counting missed package initialization")
	}
	testsupport.Check(t, m.Destroy())
	checkInvalidMonitor(t, monitor, gosim.ErrMachineNotFound)
	checkInvalidMonitor(t, (gosim.Machine{}).SyscallsMonitor(), gosim.ErrWrongRun)
}

func TestSyscallMonitorRuntimeDescriptors(t *testing.T) {
	monitor := gosim.CurrentMachine().SyscallsMonitor()
	testsupport.Check(t, monitor.Start())
	if n, err := syscall.Read(0, make([]byte, 1)); err != nil || n != 0 {
		t.Fatal("stdin", n, err)
	}
	if _, err := syscall.Read(999, make([]byte, 1)); !errors.Is(err, syscall.EBADF) {
		t.Fatal(err)
	}
	testsupport.Check(t, monitor.Stop())
	want := map[string]gosim.SyscallStat{"read": {Count: 2, Group: gosim.SyscallGroupRT, ByDescriptor: map[gosim.DescriptorGroup]uint64{gosim.DescriptorGroupRT: 1, gosim.DescriptorGroupUnknown: 1}}}
	if got := monitorStats(t, monitor); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
}

func TestSyscallMonitorRestartAndIsolation(t *testing.T) {
	ready := make(chan struct{})
	m, err := gosim.NewMachine(gosim.MachineConfig{Main: func() {
		_ = syscall.Getpid()
		if n, err := syscall.Read(0, make([]byte, 1)); err != nil || n != 0 {
			t.Fatal("new epoch stdin", n, err)
		}
		testsupport.Check(t, syscall.Close(0))
		if _, err := syscall.Read(0, make([]byte, 1)); !errors.Is(err, syscall.EBADF) {
			t.Fatal(err)
		}
		ready <- struct{}{}
		select {}
	}})
	testsupport.Check(t, err)
	monitor := m.SyscallsMonitor()
	testsupport.Check(t, monitor.Start())
	testsupport.Check(t, m.Start())
	<-ready
	other := testsupport.Start(t, func() { _ = syscall.Getpid() })
	testsupport.Wait(t, other)
	if len(monitorCounts(t, other.SyscallsMonitor())) != 0 {
		t.Fatal("another machine inherited monitoring")
	}
	for epoch := uint64(1); epoch <= 2; epoch++ {
		stats := monitorStats(t, monitor)
		if stats["getpid"].Count != epoch || stats["read"].Count != 2*epoch || stats["read"].ByDescriptor[gosim.DescriptorGroupRT] != epoch || stats["read"].ByDescriptor[gosim.DescriptorGroupUnknown] != epoch || stats["close"].ByDescriptor[gosim.DescriptorGroupRT] != epoch {
			t.Fatal("epoch ownership/counts", epoch, stats)
		}
		testsupport.Check(t, m.Crash())
		if !reflect.DeepEqual(monitorStats(t, monitor), stats) {
			t.Fatal("crash changed statistics")
		}
		if epoch == 1 {
			testsupport.Check(t, m.Restart())
			<-ready
		}
	}
	testsupport.Check(t, monitor.Stop())
	before := monitorStats(t, monitor)
	testsupport.Check(t, m.Restart())
	<-ready
	testsupport.Check(t, m.Stop())
	if !reflect.DeepEqual(monitorStats(t, monitor), before) {
		t.Fatal("restart did not preserve pause")
	}
	testsupport.Check(t, m.Destroy())
	checkInvalidMonitor(t, monitor, gosim.ErrMachineNotFound)
}
