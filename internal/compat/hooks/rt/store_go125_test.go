//go:build go1.25 && !go1.26

package rt

import "testing"

func TestStoreRelBaselineResult(t *testing.T) {
	var store func(*uintptr, uintptr) uintptr = Sync_runtime_StoreReluintptr
	runHooks(t, func() {
		var value uintptr
		if result := store(&value, 42); result != 0 || Sync_runtime_LoadAcquintptr(&value) != 42 {
			t.Fatal("store/load or baseline result changed", result, value)
		}
	})
}
