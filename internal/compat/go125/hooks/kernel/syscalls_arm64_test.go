//go:build go1.25 && !go1.26 && arm64

package kernel_test

import (
	"math"
	"testing"
	"unsafe"

	fh "github.com/neria-cloud/gosim/internal/compat/go125/hooks/fs"
	kh "github.com/neria-cloud/gosim/internal/compat/go125/hooks/kernel"
	"github.com/neria-cloud/gosim/internal/contracts"
)

//go:linkname marshalArm64Stat github.com/neria-cloud/gosim/internal/compat/go125/hooks/fs.writeStat
func marshalArm64Stat(unsafe.Pointer, contracts.Stat)

func TestArm64LinuxLayoutOnHost(t *testing.T) {
	if unsafe.Sizeof(kh.Stat_t{}) != 128 || unsafe.Offsetof(kh.Stat_t{}.Mode) != 16 ||
		unsafe.Offsetof(kh.Stat_t{}.Nlink) != 20 || unsafe.Sizeof(kh.Stat_t{}.Nlink) != 4 {
		t.Fatal("host stat substituted for Linux arm64")
	}
	if kh.LinuxSYS_FSTATAT != 79 || kh.LinuxSYS_GETRANDOM != 278 {
		t.Fatal("wrong arm64 syscall numbers")
	}
	// This runs on Darwin too: the adapters use generated Linux records instead
	// of the native host's syscall.Stat_t. Full translation is wired in Task 21.
	must(t, runABI(t, contracts.FeatureSet(contracts.RT|contracts.FS), func() {
		fd, err := fh.Syscall_openat(-100, "linux-stat", 2|0x40, 0o600)
		must(t, err)
		if n, err := kh.Syscall_write(fd, []byte("linux")); n != 5 || err != nil {
			t.Fatal(n, err)
		}
		var out kh.Stat_t
		must(t, fh.Syscall_Fstat(fd, &out))
		if out.Size != 5 || out.Mode != 0o100600 || out.Nlink != 1 {
			t.Fatal(out)
		}
		marshalArm64Stat(unsafe.Pointer(&out), contracts.Stat{Inode: 7, Size: 9, Nlink: math.MaxUint32, AtimeNS: -1})
		if out.Nlink != math.MaxUint32 || out.Ino != 7 || out.Size != 9 || out.Atim.Sec != -1 ||
			out.Atim.Nsec != 999999999 {
			t.Fatal(out)
		}
		must(t, kh.Syscall_Close(fd))
	}))
	reached, deferred := false, false
	err := runABI(t, contracts.FeatureSet(contracts.RT|contracts.FS), func() {
		defer func() { _ = recover(); deferred = true }()
		var out kh.Stat_t
		marshalArm64Stat(unsafe.Pointer(&out), contracts.Stat{Nlink: math.MaxUint32 + 1})
		reached = true
	})
	if err == nil || reached || deferred {
		t.Fatal("nlink narrowing lost information", err, reached, deferred)
	}
}
