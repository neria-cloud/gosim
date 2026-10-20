//go:build go1.25 && !go1.26

package rt

import (
	"unsafe"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func Sync_runtime_Semacquire(addr *uint32) {
	gosimruntime.Semacquire(addr, false)
}

func Sync_runtime_Semrelease(addr *uint32, _ bool, skipframes int) {
	gosimruntime.Semrelease(addr)
}

func Sync_runtime_SemacquireRWMutexR(addr *uint32, lifo bool, _ int) {
	gosimruntime.Semacquire(addr, lifo)
}

func Sync_runtime_SemacquireRWMutex(addr *uint32, lifo bool, _ int) {
	gosimruntime.Semacquire(addr, lifo)
}

func Sync_runtime_registerPoolCleanup(_ func()) {
	// noop
}

func Sync_runtime_procPin() int {
	return gosimruntime.ProcPin()
}

func Sync_runtime_procUnpin() {
	gosimruntime.ProcUnpin()
}

//go:norace
func Sync_runtime_LoadAcquintptr(ptr *uintptr) uintptr {
	// XXX
	gosimruntime.AtomicYield()
	return *ptr
}

//go:norace
func Sync_runtime_StoreReluintptr(ptr *uintptr, val uintptr) uintptr {
	gosimruntime.AtomicYield()
	*ptr = val
	return 0 // sync.Pool ignores this result; only the modeled store is observable.
}

type NotifyList struct {
	inner gosimruntime.NotifyList
}

func Sync_runtime_notifyListAdd(l *NotifyList) uint32 {
	return l.inner.Add()
}

func Sync_runtime_notifyListWait(l *NotifyList, t uint32) {
	l.inner.Wait(t)
}

func Sync_runtime_notifyListNotifyAll(l *NotifyList) {
	l.inner.NotifyAll()
}

func Sync_runtime_notifyListNotifyOne(l *NotifyList) {
	l.inner.NotifyOne()
}

// Ensure that sync and runtime agree on size of notifyList.
func Sync_runtime_notifyListCheck(size uintptr) {
	if size != unsafe.Sizeof(NotifyList{}) {
		panic("gosim: translated notifyList layout mismatch")
	}
}

func Sync_runtime_randn(n uint32) uint32 {
	return gosimruntime.Fastrandn(n)
}

func Sync_throw(str string) {
	gosimruntime.Abort(abi.Diagnostic{Kind: "RuntimeFatal", Feature: "rt", Operation: "Sync_throw", Message: str})
}

func Sync_fatal(str string) {
	gosimruntime.Abort(abi.Diagnostic{Kind: "RuntimeFatal", Feature: "rt", Operation: "Sync_fatal", Message: str})
}

func Sync_runtime_SemacquireWaitGroup(addr *uint32, _ bool) {
	gosimruntime.Semacquire(addr, false)
}

func InternalSync_runtime_SemacquireMutex(addr *uint32, lifo bool, _ int) {
	gosimruntime.Semacquire(addr, lifo)
}

func InternalSync_runtime_Semrelease(addr *uint32, _ bool, skipframes int) {
	gosimruntime.Semrelease(addr)
}

func InternalSync_runtime_canSpin(_ int) bool {
	return false
}

func InternalSync_runtime_doSpin() {
	panic("no")
}

func InternalSync_runtime_nanotime() int64 {
	return gosimruntime.Nanotime()
}

func InternalSync_runtime_rand() uint64 {
	return gosimruntime.Fastrand64()
}

func InternalSync_throw(str string) {
	gosimruntime.Abort(
		abi.Diagnostic{Kind: "RuntimeFatal", Feature: "rt", Operation: "InternalSync_throw", Message: str},
	)
}

func InternalSync_fatal(str string) {
	gosimruntime.Abort(
		abi.Diagnostic{Kind: "RuntimeFatal", Feature: "rt", Operation: "InternalSync_fatal", Message: str},
	)
}
