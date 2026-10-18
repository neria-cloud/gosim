//go:build linux

package go125

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/neria-cloud/gosim/internal/simulation"
	"github.com/neria-cloud/gosim/rt/runtime"
)

func Syscall_runtime_BeforeFork() {
	panic("gosim not implemented")
}

func Syscall_runtime_AfterFork() {
	panic("gosim not implemented")
}

func Syscall_runtime_AfterForkInChild() {
	panic("gosim not implemented")
}

func Syscall_runtime_BeforeExec() {
	panic("gosim not implemented")
}

func Syscall_runtime_AfterExec() {
	panic("gosim not implemented")
}

func Syscall_hasWaitingReaders(rw *sync.RWMutex) bool {
	panic("gosim not implemented")
}

func Syscall_Getpagesize() int {
	// TODO: support bigger pages optionally? flip per architecture?
	return 4096
}

func Syscall_Exit(code int) {
	panic("gosim not implemented")
}

func Syscall_runtimeClearenv(env map[string]int) {
	panic("gosim not implemented")
}

func Syscall_runtimeSetenv(k, v string) {
	panic("gosim not implemented")
}

func Syscall_runtimeUnsetenv(k string) {
	panic("gosim not implemented")
}

func Syscall_rawSyscallNoError(trap, a1, a2, a3 uintptr) (r1, r2 uintptr) {
	panic("gosim not implemented")
}

func Syscall_rawVforkSyscall(trap, a1, a2 uintptr) (r1 uintptr, err syscall.Errno) {
	panic("gosim not implemented")
}

func Syscall_runtime_doAllThreadsSyscall(trap, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2, err uintptr) {
	panic("gosim not implemented")
}

func Syscall_cgocaller(unsafe.Pointer, ...uintptr) uintptr {
	panic("gosim not implemented")
}

func Syscall_runtime_envs() []string {
	return gosimruntime.Envs()
}

func Syscall_runtime_entersyscall() {
}

func Syscall_runtime_exitsyscall() {
}

//go:nocheckptr
//go:uintptrescapes
func Syscall_RawSyscall6(trap, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno) {
	return simulation.RawSyscall6(trap, a1, a2, a3, a4, a5, a6)
}

func Syscall_gettimeofday(timeval unsafe.Pointer) syscall.Errno {
	panic("gosim not implemented")
}

func Syscall_FcntlFlock(fd uintptr, cmd int, lk *syscall.Flock_t) error {
	return simulation.SyscallSysFcntlFlock(fd, cmd, lk)
}
