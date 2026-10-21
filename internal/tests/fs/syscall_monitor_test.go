//go:build sim

package behavior_test

import (
	"errors"
	"reflect"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
)

func TestSyscallMonitorFilesystem(t *testing.T) {
	monitor := gosim.CurrentMachine().SyscallsMonitor()
	testsupport.Check(t, monitor.Start())
	fd, err := unix.Openat(unix.AT_FDCWD, "monitored", unix.O_CREAT|unix.O_RDWR, 0o600)
	testsupport.Check(t, err)
	if n, err := unix.Write(fd, []byte("data")); err != nil || n != 4 {
		t.Fatal(n, err)
	}
	_, err = unix.Seek(fd, 0, 0)
	testsupport.Check(t, err)
	if _, err := unix.Seek(fd, 0, 99); !errors.Is(err, unix.EINVAL) {
		t.Fatal(err)
	}
	data := make([]byte, 4)
	if n, err := unix.Read(fd, data); err != nil || n != 4 || string(data) != "data" {
		t.Fatal(n, err, string(data))
	}
	testsupport.Check(t, unix.Fsync(fd))
	testsupport.Check(t, unix.Fdatasync(fd))
	var stat unix.Stat_t
	testsupport.Check(t, unix.Fstat(fd, &stat))
	if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0); !errors.Is(err, unix.ENOSYS) {
		t.Fatal(err)
	}
	lock := unix.Flock_t{Type: unix.F_WRLCK}
	testsupport.Check(t, unix.FcntlFlock(uintptr(fd), unix.F_SETLK, &lock))
	testsupport.Check(t, unix.Close(fd))
	if err := unix.Close(fd); !errors.Is(err, unix.EBADF) {
		t.Fatal(err)
	}
	if _, err := unix.Read(fd, data); !errors.Is(err, unix.EBADF) {
		t.Fatal(err)
	}
	testsupport.Check(t, monitor.Stop())
	counts, err := monitor.Counts()
	testsupport.Check(t, err)
	want := map[string]uint64{"openat": 1, "write": 1, "lseek": 2, "read": 2, "fsync": 1, "fdatasync": 1, "fstat": 1, "fcntl": 2, "close": 2}
	if !reflect.DeepEqual(counts, want) {
		t.Fatal("counts", counts)
	}
	stats, err := monitor.Stats()
	testsupport.Check(t, err)
	if len(stats) != len(want) {
		t.Fatal(stats)
	}
	for name, count := range want {
		group := gosim.SyscallGroupFS
		if name == "read" || name == "write" || name == "close" || name == "fcntl" {
			group = gosim.SyscallGroupRT
		}
		buckets := map[gosim.DescriptorGroup]uint64{gosim.DescriptorGroupFS: count}
		if name == "openat" {
			buckets = map[gosim.DescriptorGroup]uint64{gosim.DescriptorGroupNone: 1}
		}
		if name == "read" || name == "close" {
			buckets = map[gosim.DescriptorGroup]uint64{gosim.DescriptorGroupFS: 1, gosim.DescriptorGroupUnknown: 1}
		}
		expected := gosim.SyscallStat{Count: count, Group: group, ByDescriptor: buckets}
		if !reflect.DeepEqual(stats[name], expected) {
			t.Fatalf("%s: %+v want %+v", name, stats[name], expected)
		}
	}
}
