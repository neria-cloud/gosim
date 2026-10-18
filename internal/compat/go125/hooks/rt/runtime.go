//go:build go1.25 && !go1.26

package rt

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
