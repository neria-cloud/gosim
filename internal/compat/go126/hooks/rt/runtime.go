//go:build go1.26 && !go1.27

package rt

import bridge "github.com/neria-cloud/gosim/internal/compat/go126/shims/runtimebridge"

func Runtime_NumCPU() int                         { return bridge.Runtime_NumCPU() }
func Runtime_GOMAXPROCS(n int) int                { return bridge.Runtime_GOMAXPROCS(n) }
func Runtime_SetFinalizer(obj any, finalizer any) { bridge.Runtime_SetFinalizer(obj, finalizer) }
