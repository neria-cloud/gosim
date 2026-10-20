//go:build go1.25 && !go1.26

package rt

import bridge "github.com/neria-cloud/gosim/internal/compat/go125/shims/runtimebridge"

func Runtime_NumCPU() int                         { return bridge.Runtime_NumCPU() }
func Runtime_GOMAXPROCS(n int) int                { return bridge.Runtime_GOMAXPROCS(n) }
func Runtime_SetFinalizer(obj any, finalizer any) { bridge.Runtime_SetFinalizer(obj, finalizer) }
