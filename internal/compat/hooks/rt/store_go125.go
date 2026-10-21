//go:build go1.25 && !go1.26

package rt

import "github.com/neria-cloud/gosim/rt/runtime"

//go:norace
func Sync_runtime_StoreReluintptr(ptr *uintptr, val uintptr) uintptr {
	gosimruntime.AtomicYield()
	*ptr = val
	return 0 // sync.Pool ignores this result; only the modeled store is observable.
}
