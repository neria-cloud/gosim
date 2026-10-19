//go:build gosim_retired && linux

package go125

import (
	"syscall"
	"unsafe"

	"github.com/neria-cloud/gosim/internal/simulation"
)

//go:nocheckptr
//go:uintptrescapes
func GolangOrgXSysUnix_RawSyscall6(trap, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno) {
	return simulation.RawSyscall6(trap, a1, a2, a3, a4, a5, a6)
}

func GolangOrgXSysUnix_SyscallNoError(trap, a1, a2, a3 uintptr) (r1, r2 uintptr) {
	panic("gosim not implemented")
}

func GolangOrgXSysUnix_RawSyscallNoError(trap, a1, a2, a3 uintptr) (r1, r2 uintptr) {
	panic("gosim not implemented")
}

func GolangOrgXSysUnix_Syscall(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno) {
	return simulation.RawSyscall6(trap, a1, a2, a3, 0, 0, 0)
}

func GolangOrgXSysUnix_Syscall6(trap, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno) {
	return simulation.RawSyscall6(trap, a1, a2, a3, a4, a5, a6)
}

func GolangOrgXSysUnix_RawSyscall(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno) {
	panic("gosim not implemented")
}

func GolangOrgXSysUnix_gettimeofday(timeval unsafe.Pointer) syscall.Errno {
	panic("gosim not implemented")
}
