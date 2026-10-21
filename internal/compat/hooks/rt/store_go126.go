//go:build go1.26

package rt

import "github.com/neria-cloud/gosim/rt/runtime"

//go:norace
func Sync_runtime_StoreReluintptr(ptr *uintptr, val uintptr) {
	gosimruntime.AtomicYield()
	*ptr = val
}
