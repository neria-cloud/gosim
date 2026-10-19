//go:build go1.26 && !go1.27

package runtimebridge

import (
	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func Runtime_NumCPU() int { return 1 }

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
