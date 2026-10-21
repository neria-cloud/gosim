//go:build go1.25

package runtimebridge

import (
	"runtime" //gosim:notranslate

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func Runtime_NumCPU() int { return 1 }

func Runtime_SetBlockProfileRate(rate int) {
	// Nonpositive rates disable profiling, which is already disabled here.
	if rate <= 0 {
		return
	}
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "runtime.SetBlockProfileRate", Message: "native profiling is not modeled"})
}

func Runtime_ReadMemStats(stats *runtime.MemStats) {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "runtime.ReadMemStats", Message: "native GC statistics are not modeled"})
}

func Runtime_GOMAXPROCS(n int) int {
	if n > 1 {
		gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "runtime.GOMAXPROCS", Message: "the deterministic scheduler supports one simulated processor"})
	}
	return 1
}

func Runtime_SetFinalizer(obj any, finalizer any) {
	// Named stub in the profile: no native GC callbacks. Machine lifecycle owns
	// descriptors and resources; translated application objects do not escape.
}

// Runtime_GOROOT reports no installed toolchain in the simulated filesystem.
// In particular time.LoadLocation must never search the host GOROOT for data.
func Runtime_GOROOT() string { return "" }

// RuntimeCleanup carries no native GC handle. The stdlib can declare its fields,
// but registering or operating on a cleanup is explicitly unsupported.
type RuntimeCleanup struct{}

func Runtime_AddCleanup[T, S any](_ *T, _ func(S), _ S) RuntimeCleanup {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "runtime.AddCleanup", Message: "host GC cleanups are not modeled"})
	panic("abort returned")
}

func (RuntimeCleanup) Stop() {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "runtime.Cleanup.Stop", Message: "host GC cleanups are not modeled"})
}

// Legacy runtime random linknames are redirected for all translated callers.
func Runtime_fastrand() uint32          { return uint32(gosimruntime.Fastrand64()) }
func Runtime_fastrandn(n uint32) uint32 { return uint32(uint64(Runtime_fastrand()) * uint64(n) >> 32) }
func Runtime_fastrand64() uint64        { return gosimruntime.Fastrand64() }

// Runtime_getAuxv exposes no host ELF vector or process addresses.
// x/sys Auxv reports ENOENT for this explicitly unavailable process service.
func Runtime_getAuxv() []uintptr { return nil }

func Runtime_NumGoroutine() int {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "runtime.NumGoroutine", Message: "runtime goroutine metrics are not modeled"})
	panic("abort returned")
}

func Runtime_ThreadCreateProfile(p any) (int, bool) {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "runtime.ThreadCreateProfile", Message: "host thread profiling is not modeled"})
	panic("abort returned")
}
