//go:build go1.26

package rt

import "testing"

func TestStoreRelHasNoResult(t *testing.T) {
	var store func(*uintptr, uintptr) = Sync_runtime_StoreReluintptr
	runHooks(t, func() {
		var value uintptr
		store(&value, 42)
		if Sync_runtime_LoadAcquintptr(&value) != 42 {
			t.Fatal("store/load mismatch")
		}
	})
}
