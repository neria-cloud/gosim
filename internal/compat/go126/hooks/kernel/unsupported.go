//go:build go1.26 && !go1.27

package kernel

import (
	"sync"
	"syscall"
	"unsafe"
)

// TODO(not planned): full Linux processes, signals, native threads and application cgo.
// These declarations are needed by translated stdlib bodies, but none may enter
// host process, thread, cgo, VDSO or scheduler state. Abort cannot be recovered by
// an application and occurs before inspecting any borrowed pointer.
func Syscall_runtime_BeforeFork()       { Reject("syscall.runtime_BeforeFork") }
func Syscall_runtime_AfterFork()        { Reject("syscall.runtime_AfterFork") }
func Syscall_runtime_AfterForkInChild() { Reject("syscall.runtime_AfterForkInChild") }
func Syscall_runtime_BeforeExec()       { Reject("syscall.runtime_BeforeExec") }
func Syscall_runtime_AfterExec()        { Reject("syscall.runtime_AfterExec") }
func Syscall_hasWaitingReaders(rw *sync.RWMutex) bool {
	Reject("syscall.hasWaitingReaders")
	return false
}
func Syscall_Exit(code int) { Reject("syscall.Exit") }
func Syscall_runtime_doAllThreadsSyscall(trap, a1, a2, a3, a4, a5, a6 uintptr) (uintptr, uintptr, uintptr) {
	Reject("syscall.runtime_doAllThreadsSyscall")
	return 0, 0, 0
}

func Syscall_cgocaller(fn unsafe.Pointer, args ...uintptr) uintptr {
	Reject("syscall.cgocaller")
	return 0
}
func Syscall_runtime_entersyscall() { Reject("syscall.runtime_entersyscall") }
func Syscall_runtime_exitsyscall()  { Reject("syscall.runtime_exitsyscall") }

// Timeval is intentionally opaque: these rejected amd64-only ABI entries never
// dereference it, including on a Darwin host compiling Linux-selected source.
func Syscall_gettimeofday(tv unsafe.Pointer) syscall.Errno { Reject("syscall.gettimeofday"); return 0 }

func GolangOrgXSysUnix_gettimeofday(tv unsafe.Pointer) syscall.Errno {
	Reject("golang.org/x/sys/unix.gettimeofday")
	return 0
}

func Os_ignoreSIGSYS()  { Reject("os.ignoreSIGSYS") }
func Os_restoreSIGSYS() { Reject("os.restoreSIGSYS") }
