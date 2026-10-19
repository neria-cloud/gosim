//go:build go1.26 && !go1.27 && linux && (amd64 || arm64)

package kernel_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"

	fh "github.com/neria-cloud/gosim/internal/compat/go126/hooks/fs"
	kh "github.com/neria-cloud/gosim/internal/compat/go126/hooks/kernel"
	nh "github.com/neria-cloud/gosim/internal/compat/go126/hooks/net"
	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/control"
	"github.com/neria-cloud/gosim/internal/kernel"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

// Exercise the real upstream pointer types across the same ABI linknames used
// by translated source, not only calls using our generated record types.
//
//go:linkname linkedFstat github.com/neria-cloud/gosim/internal/compat/go126/hooks/fs.Syscall_Fstat
func linkedFstat(int, *syscall.Stat_t) error

//go:linkname linkedXStat github.com/neria-cloud/gosim/internal/compat/go126/hooks/fs.GolangOrgXSysUnix_Fstat
func linkedXStat(int, *unix.Stat_t) error

//go:linkname linkedLock github.com/neria-cloud/gosim/internal/compat/go126/hooks/fs.Syscall_FcntlFlock
func linkedLock(uintptr, int, *syscall.Flock_t) error

//go:linkname linkedXLock github.com/neria-cloud/gosim/internal/compat/go126/hooks/fs.GolangOrgXSysUnix_FcntlFlock
func linkedXLock(uintptr, int, *unix.Flock_t) error

func TestLinuxLayouts(t *testing.T) {
	t.Parallel()
	for name, pair := range map[string][2]uintptr{
		"stat-size": {unsafe.Sizeof(kh.Stat_t{}), unsafe.Sizeof(syscall.Stat_t{})}, "xstat-size": {unsafe.Sizeof(kh.XStat_t{}), unsafe.Sizeof(unix.Stat_t{})},
		"stat-mode": {unsafe.Offsetof(kh.Stat_t{}.Mode), unsafe.Offsetof(syscall.Stat_t{}.Mode)}, "xstat-atim": {unsafe.Offsetof(kh.XStat_t{}.Atim), unsafe.Offsetof(unix.Stat_t{}.Atim)},
		"flock-size": {unsafe.Sizeof(kh.Flock_t{}), unsafe.Sizeof(syscall.Flock_t{})}, "xflock-size": {unsafe.Sizeof(kh.XFlock_t{}), unsafe.Sizeof(unix.Flock_t{})},
		"flock-start": {unsafe.Offsetof(kh.Flock_t{}.Start), unsafe.Offsetof(unix.Flock_t{}.Start)}, "flock-pid": {unsafe.Offsetof(kh.Flock_t{}.Pid), unsafe.Offsetof(unix.Flock_t{}.Pid)},
		"dirent-name": {unsafe.Offsetof(kh.Dirent{}.Name), 19}, "dirent-reclen": {unsafe.Offsetof(kh.Dirent{}.Reclen), 16},
		"sockaddr-size": {unsafe.Sizeof(kh.RawSockaddrAny{}), unsafe.Sizeof(syscall.RawSockaddrAny{})}, "ipv4-size": {unsafe.Sizeof(kh.RawSockaddrInet4{}), 16},
		"uname-size": {unsafe.Sizeof(kh.Utsname{}), unsafe.Sizeof(unix.Utsname{})},
	} {
		if pair[0] != pair[1] {
			t.Fatal(name, pair)
		}
	}
	if kh.LinuxSYS_FCNTL != syscall.SYS_FCNTL || kh.LinuxSYS_GETRANDOM != unix.SYS_GETRANDOM {
		t.Fatal("syscall numbers")
	}
}

func TestLinuxFileRoutes(t *testing.T) {
	must(t, runABI(t, contracts.FeatureSet(contracts.RT|contracts.FS), func() {
		fd, err := fh.Syscall_openat(-100, "file", 2|0x40|0x200, 0o600)
		must(t, err)
		if n, err := kh.Syscall_write(fd, []byte("abcdef")); n != 6 || err != nil {
			t.Fatal(n, err)
		}
		if n, err := fh.GolangOrgXSysUnix_pwrite(fd, []byte("Q"), 1); n != 1 || err != nil {
			t.Fatal(n, err)
		}
		data := make([]byte, 6)
		if n, err := fh.Syscall_pread(fd, data, 0); n != 6 || err != nil || string(data) != "aQcdef" {
			t.Fatal(n, err, string(data))
		}
		if _, err := fh.Syscall_Seek(fd, 0, 0); err != nil {
			t.Fatal(err)
		}
		clear(data)
		if n, err := kh.GolangOrgXSysUnix_read(fd, data); n != 6 || err != nil || string(data) != "aQcdef" {
			t.Fatal(n, err, string(data))
		}
		var stat syscall.Stat_t
		must(t, linkedFstat(fd, &stat))
		if stat.Size != 6 || stat.Mode != 0o100000 || stat.Uid != 0 || stat.Atim.Sec != 0 || stat.Nlink != 0 {
			t.Fatal(stat)
		}
		var xstat unix.Stat_t
		must(t, linkedXStat(fd, &xstat))
		if xstat.Size != stat.Size || xstat.Ino != stat.Ino || xstat.Mode != stat.Mode {
			t.Fatal(xstat)
		}
		lock := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0}
		must(t, linkedLock(uintptr(fd), syscall.F_SETLK, &lock))
		xlock := unix.Flock_t{Type: unix.F_WRLCK, Whence: 0}
		must(t, linkedXLock(uintptr(fd), unix.F_GETLK, &xlock))
		if xlock.Type != unix.F_WRLCK {
			t.Fatal(xlock)
		}
		if _, err := kh.GolangOrgXSysUnix_FcntlInt(uintptr(fd), syscall.F_GETLK, 0); err != syscall.EINVAL {
			t.Fatal("integer pointer command", err)
		}
		must(t, fh.Syscall_Flock(fd, syscall.LOCK_EX))
		must(t, fh.Syscall_Madvise(data, 0))
		mapped, err := fh.Syscall_mmap(0, 6, 1, 1, fd, 0)
		must(t, err)
		if mapped == 0 {
			t.Fatal("empty map")
		}
		must(t, fh.Syscall_munmap(mapped, 6))
		must(t, fh.Syscall_Fallocate(fd, 0, 0, 10))
		must(t, fh.Syscall_Ftruncate(fd, 8))
		must(t, fh.Syscall_Fdatasync(fd))
		must(t, fh.Syscall_Fsync(fd))
		must(t, fh.Syscall_Renameat(-100, "file", -100, "renamed"))
		var raw kh.Stat_t
		must(t, fh.Syscall_fstatat(-100, "renamed", &raw, 0))
		if raw.Size != 8 {
			t.Fatal(raw)
		}
		if err := fh.Syscall_Fstat(fd, nil); err != syscall.EFAULT {
			t.Fatal(err)
		}
		if _, err := fh.Syscall_openat(-100, "bad\x00name", 0, 0); err != syscall.EINVAL {
			t.Fatal(err)
		}
		must(t, fh.Syscall_Mkdirat(-100, "sub", 0o700))
		must(t, fh.Syscall_Chdir("sub"))
		cwd := make([]byte, 256)
		if n, err := fh.Syscall_Getcwd(cwd); err != nil || n < 2 || cwd[n-1] != 0 {
			t.Fatal(n, err)
		}
		must(t, fh.Syscall_Chdir(".."))
		dir, err := fh.Syscall_openat(-100, ".", 0, 0)
		must(t, err)
		entries := make([]byte, 512)
		n, err := fh.Syscall_Getdents(dir, entries)
		must(t, err)
		if n < 20 || binary.LittleEndian.Uint16(entries[16:18]) < 20 || !bytes.Contains(entries[:n], []byte("renamed")) {
			t.Fatal(entries[:n])
		}
		must(t, kh.Syscall_Close(dir))
		must(t, kh.Syscall_Close(fd))
		must(t, fh.Syscall_unlinkat(-100, "renamed", 0))
	}))
}

func TestLinuxProcessAndSocketRoutes(t *testing.T) {
	must(t, runABI(t, contracts.FeatureSet(contracts.RT|contracts.Net), func() {
		limit := kh.Rlimit{Cur: 123, Max: 456}
		if err := kh.Syscall_Getrlimit(7, &limit); err != syscall.ENOSYS || limit.Cur != 123 || limit.Max != 456 {
			t.Fatal(limit, err)
		}
		if kh.Syscall_Getpid() != 42 {
			t.Fatal("pid")
		}
		var uname kh.Utsname
		must(t, kh.Syscall_Uname(&uname))
		if uname.Sysname[0] != 0 || uname.Release[0] != 0 {
			t.Fatal("invented uname", uname)
		}
		random := make([]byte, 32)
		if n, err := kh.InternalSyscallUnix_GetRandom(random, 0); n != 32 || err != nil || bytes.Equal(random, make([]byte, 32)) {
			t.Fatal(n, err)
		}
		if n, ok := kh.InternalSyscallUnix_vgetrandom(random, 1); !ok || n != -int(syscall.EINVAL) {
			t.Fatal(n, ok)
		}
		if value, errno := kh.InternalSyscallUnix_fcntl(2, 3, 0); value != 1 || errno != 0 {
			t.Fatal(value, errno)
		}
		if value, errno := kh.InternalSyscallUnix_fcntl(99, 3, 0); value != -1 || errno != int32(syscall.EBADF) {
			t.Fatal(value, errno)
		}
		fd, err := nh.Syscall_socket(2, 1|0x800|0x80000, 6)
		must(t, err)
		address := kh.RawSockaddrInet4{Family: 2, Port: 0x901f} // 8080 in network byte order
		must(t, nh.Syscall_bind(fd, unsafe.Pointer(&address), 16))
		must(t, nh.Syscall_Listen(fd, 16))
		var result kh.RawSockaddrAny
		length := uint32(112)
		must(t, nh.Syscall_getsockname(fd, &result, &length))
		if length != 16 || (*kh.RawSockaddrInet4)(unsafe.Pointer(&result)).Port != address.Port {
			t.Fatal(result, length)
		}
		if err := nh.Syscall_bind(fd, nil, 0); err != syscall.EFAULT {
			t.Fatal(err)
		}
		ipv6 := kh.RawSockaddrInet4{Family: 10}
		if err := nh.Syscall_connect(fd, unsafe.Pointer(&ipv6), 16); err != syscall.EAFNOSUPPORT {
			t.Fatal(err)
		}
		if err := nh.Syscall_connect(fd, unsafe.Pointer(&address), 2); err != syscall.EINVAL {
			t.Fatal(err)
		}
		length = 2
		if err := nh.Syscall_getsockname(fd, &result, &length); err != syscall.EINVAL || length != 2 {
			t.Fatal(err, length)
		}
		var option int32
		length = 4
		if err := nh.Syscall_getsockopt(fd, 1, 4, unsafe.Pointer(&option), &length); err != syscall.ENOTCONN {
			t.Fatal(err)
		}
		client, err := nh.Syscall_socket(2, 1|0x800|0x80000, 6)
		must(t, err)
		local, err := kh.Call(kernel.NetAddressRequest{Machine: control.CurrentRef()}, kernel.DecodeNetAddressResult)
		must(t, err)
		address.Addr = local.Address
		if err := nh.Syscall_connect(client, unsafe.Pointer(&address), 16); err != syscall.EINPROGRESS {
			t.Fatal(err)
		}
		simrt.Sleep(100000000)
		length = 112
		accepted, err := nh.Syscall_accept4(fd, &result, &length, 0x800|0x80000)
		must(t, err)
		if length != 16 {
			t.Fatal(length)
		}
		length = 112
		must(t, nh.Syscall_getpeername(client, &result, &length))
		length = 4
		must(t, nh.Syscall_getsockopt(client, 1, 4, unsafe.Pointer(&option), &length))
		if option != 0 || length != 4 {
			t.Fatal(option, length)
		}
		must(t, kh.Syscall_Close(accepted))
		must(t, kh.Syscall_Close(client))
		if err := nh.Syscall_getsockopt(fd, 1, 4, nil, nil); err != syscall.EFAULT {
			t.Fatal(err)
		}
		option = 1
		must(t, nh.Syscall_setsockopt(fd, 1, 2, unsafe.Pointer(&option), 4))
		must(t, kh.Syscall_Close(fd))
	}))
}

func TestLinuxDisabledAndRawBoundaries(t *testing.T) {
	for name, operation := range map[string]func(){
		"fs-nil": func() { _ = fh.Syscall_Fstat(-1, nil) }, "net-nil": func() { _ = nh.Syscall_bind(-1, nil, 0) },
		"raw-known": func() { _, _, _ = kh.Syscall_RawSyscall6(kh.LinuxSYS_READ, 0, 0, 0, 0, 0, 0) }, "raw-unknown": func() { _, _, _ = kh.GolangOrgXSysUnix_Syscall(99999, 0, 0, 0) },
		"rlimit": func() { _ = kh.Syscall_Getrlimit(1, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			reached, deferred := false, false
			err := runABI(t, contracts.FeatureSet(contracts.RT), func() { defer func() { _ = recover(); deferred = true }(); operation(); reached = true })
			if err == nil || reached || deferred {
				t.Fatal(err, reached, deferred)
			}
		})
	}
}

// Borrowed memory remains visible to the race detector even though the kernel
// reads/writes it with norace views. The child intentionally races application
// stores with a real write adapter; the parent requires the race report.
func TestLinuxBorrowedMemoryRace(t *testing.T) {
	if !simrt.RaceEnabled {
		t.Skip("race-only oracle")
	}
	if os.Getenv("GOSIM_ADAPTER_RACE_CHILD") == "1" {
		must(t, runABI(t, contracts.FeatureSet(contracts.RT|contracts.FS), func() {
			fd, err := fh.Syscall_openat(-100, "race", 2|0x40, 0o600)
			must(t, err)
			data := make([]byte, 4)
			simrt.Go(func() {
				for range 5 {
					data[0]++
					simrt.Yield()
				}
			})
			for range 5 {
				_, _ = kh.Syscall_write(fd, data)
				simrt.Yield()
			}
		}))
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestLinuxBorrowedMemoryRace$")
	cmd.Env = append(os.Environ(), "GOSIM_ADAPTER_RACE_CHILD=1", "GORACE=halt_on_error=1")
	out, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte("WARNING: DATA RACE")) || !bytes.Contains(out, []byte("runtime.racereadrange")) || !bytes.Contains(out, []byte("TestLinuxBorrowedMemoryRace.func1.1")) {
		t.Fatalf("missing application-memory race: %v\n%s", err, out)
	}
}
