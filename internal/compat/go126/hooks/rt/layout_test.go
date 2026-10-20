//go:build go1.26 && !go1.27

package rt

import (
	"reflect"
	"testing"
	"time"
	"unsafe"
)

// The translated channel is a pointer-sized wrapper. Only this prefix is
// shared with time.Timer/Ticker; the remaining timer state belongs to Gosim.
func TestTimerPrefix(t *testing.T) {
	t.Parallel()
	for _, typ := range []reflect.Type{reflect.TypeOf(time.Timer{}), reflect.TypeOf(time.Ticker{})} {
		if typ.NumField() != 2 || typ.Field(0).Offset != unsafe.Offsetof(timeTimer{}.c) || typ.Field(0).Type.Size() != unsafe.Sizeof(timeTimer{}.c) || typ.Field(1).Offset != unsafe.Offsetof(timeTimer{}.init) || typ.Field(1).Type.Kind() != reflect.Bool {
			t.Fatalf("pinned timer prefix drift: %v", typ)
		}
	}
}

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
