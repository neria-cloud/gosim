//go:build sim

package behavior_test

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"reflect"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
)

func TestSyscallMonitorMixedDescriptors(t *testing.T) {
	ready, release := make(chan struct{}), make(chan struct{})
	server := testsupport.StartConfigured(t, gosim.MachineConfig{Main: func() {
		listener, err := net.Listen("tcp", "10.91.0.1:9991")
		testsupport.Check(t, err)
		defer listener.Close()
		close(ready)
		conn, err := listener.Accept()
		testsupport.Check(t, err)
		defer conn.Close()
		if n, err := conn.Write([]byte("ns")); err != nil || n != 2 {
			t.Fatal(n, err)
		}
		<-release
	}}, netip.MustParseAddr("10.91.0.1"))
	<-ready
	conn, err := net.Dial("tcp", "10.91.0.1:9991")
	testsupport.Check(t, err)
	defer conn.Close()
	// Consume one byte before measuring. The second byte has arrived in the
	// same packet, so the measured raw read has no EAGAIN/poll retry ambiguity.
	data := make([]byte, 1)
	if n, err := conn.Read(data); err != nil || n != 1 || data[0] != 'n' {
		t.Fatal(n, err, data)
	}
	raw, err := conn.(*net.TCPConn).SyscallConn()
	testsupport.Check(t, err)
	var socketFD int
	testsupport.Check(t, raw.Control(func(fd uintptr) { socketFD = int(fd) }))
	testsupport.Check(t, os.WriteFile("mixed", []byte("f"), 0o600))
	file, err := os.Open("mixed")
	testsupport.Check(t, err)
	defer file.Close()
	fileFD := int(file.Fd())
	monitor := gosim.CurrentMachine().SyscallsMonitor()
	testsupport.Check(t, monitor.Start())
	if n, err := syscall.Read(fileFD, data); err != nil || n != 1 || data[0] != 'f' {
		t.Fatal(n, err, data)
	}
	if n, err := syscall.Read(socketFD, data); err != nil || n != 1 || data[0] != 's' {
		t.Fatal(n, err, data)
	}
	var stat syscall.Stat_t
	if err := syscall.Fstat(socketFD, &stat); !errors.Is(err, syscall.EBADF) {
		t.Fatal("socket Fstat", err)
	}
	testsupport.Check(t, monitor.Stop())
	counts, err := monitor.Counts()
	testsupport.Check(t, err)
	stats, err := monitor.Stats()
	testsupport.Check(t, err)
	wantCounts := map[string]uint64{"read": 2, "fstat": 1}
	wantStats := map[string]gosim.SyscallStat{
		"read":  {Count: 2, Group: gosim.SyscallGroupRT, ByDescriptor: map[gosim.DescriptorGroup]uint64{gosim.DescriptorGroupFS: 1, gosim.DescriptorGroupNet: 1}},
		"fstat": {Count: 1, Group: gosim.SyscallGroupFS, ByDescriptor: map[gosim.DescriptorGroup]uint64{gosim.DescriptorGroupNet: 1}},
	}
	if !reflect.DeepEqual(counts, wantCounts) || !reflect.DeepEqual(stats, wantStats) {
		t.Fatal(counts, stats)
	}
	for name, stat := range stats {
		var sum uint64
		for _, n := range stat.ByDescriptor {
			sum += n
		}
		if sum != stat.Count || counts[name] != sum {
			t.Fatal("inconsistent snapshot", name, stat)
		}
	}
	counts["read"] = 999
	stats["read"].ByDescriptor[gosim.DescriptorGroupNet] = 999
	delete(stats, "fstat")
	freshCounts, err := monitor.Counts()
	testsupport.Check(t, err)
	freshStats, err := monitor.Stats()
	testsupport.Check(t, err)
	if !reflect.DeepEqual(freshCounts, wantCounts) || !reflect.DeepEqual(freshStats, wantStats) {
		t.Fatal("mutated snapshot leaked")
	}
	close(release)
	testsupport.Wait(t, server)
}
