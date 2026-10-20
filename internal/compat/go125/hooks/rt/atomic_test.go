//go:build go1.25 && !go1.26

package rt

import (
	"testing"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

func runAtomicTest(t *testing.T, f func()) {
	t.Helper()
	e, err := gosimruntime.NewEngine(
		gosimruntime.EngineConfig{Seed: 42, TimeoutNS: 1e9, ReplayRevision: gosimruntime.ReplayRevision},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := e.Run(f); err != nil {
		t.Fatal(err)
	}
}

func TestAtomicInt32(t *testing.T) {
	runAtomicTest(t, func() {
		value := int32(12)
		if old := SyncAtomic_AndInt32(&value, 10); old != 12 || value != 8 {
			t.Fatalf("And old=%d value=%d", old, value)
		}
		if old := SyncAtomic_OrInt32(&value, 3); old != 8 || value != 11 {
			t.Fatalf("Or old=%d value=%d", old, value)
		}
		if old := SyncAtomic_SwapInt32(&value, 42); old != 11 || value != 42 {
			t.Fatalf("Swap old=%d value=%d", old, value)
		}
		if !SyncAtomic_CompareAndSwapInt32(&value, 42, 7) || SyncAtomic_CompareAndSwapInt32(&value, 42, 8) {
			t.Fatal("CAS state")
		}
		if updated := SyncAtomic_AddInt32(&value, 3); updated != 10 {
			t.Fatalf("Add result=%d", updated)
		}
		SyncAtomic_StoreInt32(&value, 9)
		if got := SyncAtomic_LoadInt32(&value); got != 9 {
			t.Fatalf("Load result=%d", got)
		}
	})
}

func TestAtomicInt64(t *testing.T) {
	runAtomicTest(t, func() {
		value := int64(12)
		if old := SyncAtomic_AndInt64(&value, 10); old != 12 || value != 8 {
			t.Fatalf("And old=%d value=%d", old, value)
		}
		if old := SyncAtomic_OrInt64(&value, 3); old != 8 || value != 11 {
			t.Fatalf("Or old=%d value=%d", old, value)
		}
		if old := SyncAtomic_SwapInt64(&value, 42); old != 11 || value != 42 {
			t.Fatalf("Swap old=%d value=%d", old, value)
		}
		if !SyncAtomic_CompareAndSwapInt64(&value, 42, 7) || SyncAtomic_CompareAndSwapInt64(&value, 42, 8) {
			t.Fatal("CAS state")
		}
		if updated := SyncAtomic_AddInt64(&value, 3); updated != 10 {
			t.Fatalf("Add result=%d", updated)
		}
		SyncAtomic_StoreInt64(&value, 9)
		if got := SyncAtomic_LoadInt64(&value); got != 9 {
			t.Fatalf("Load result=%d", got)
		}
	})
}

func TestAtomicUint32(t *testing.T) {
	runAtomicTest(t, func() {
		value := uint32(12)
		if old := SyncAtomic_AndUint32(&value, 10); old != 12 || value != 8 {
			t.Fatalf("And old=%d value=%d", old, value)
		}
		if old := SyncAtomic_OrUint32(&value, 3); old != 8 || value != 11 {
			t.Fatalf("Or old=%d value=%d", old, value)
		}
		if old := SyncAtomic_SwapUint32(&value, 42); old != 11 || value != 42 {
			t.Fatalf("Swap old=%d value=%d", old, value)
		}
		if !SyncAtomic_CompareAndSwapUint32(&value, 42, 7) || SyncAtomic_CompareAndSwapUint32(&value, 42, 8) {
			t.Fatal("CAS state")
		}
		if updated := SyncAtomic_AddUint32(&value, 3); updated != 10 {
			t.Fatalf("Add result=%d", updated)
		}
		SyncAtomic_StoreUint32(&value, 9)
		if got := SyncAtomic_LoadUint32(&value); got != 9 {
			t.Fatalf("Load result=%d", got)
		}
	})
}

func TestAtomicUint64(t *testing.T) {
	runAtomicTest(t, func() {
		value := uint64(12)
		if old := SyncAtomic_AndUint64(&value, 10); old != 12 || value != 8 {
			t.Fatalf("And old=%d value=%d", old, value)
		}
		if old := SyncAtomic_OrUint64(&value, 3); old != 8 || value != 11 {
			t.Fatalf("Or old=%d value=%d", old, value)
		}
		if old := SyncAtomic_SwapUint64(&value, 42); old != 11 || value != 42 {
			t.Fatalf("Swap old=%d value=%d", old, value)
		}
		if !SyncAtomic_CompareAndSwapUint64(&value, 42, 7) || SyncAtomic_CompareAndSwapUint64(&value, 42, 8) {
			t.Fatal("CAS state")
		}
		if new := SyncAtomic_AddUint64(&value, 3); new != 10 {
			t.Fatalf("Add result=%d", new)
		}
		SyncAtomic_StoreUint64(&value, 9)
		if got := SyncAtomic_LoadUint64(&value); got != 9 {
			t.Fatalf("Load result=%d", got)
		}
	})
}

func TestAtomicUintptr(t *testing.T) {
	runAtomicTest(t, func() {
		value := uintptr(12)
		if old := SyncAtomic_AndUintptr(&value, 10); old != 12 || value != 8 {
			t.Fatalf("And old=%d value=%d", old, value)
		}
		if old := SyncAtomic_OrUintptr(&value, 3); old != 8 || value != 11 {
			t.Fatalf("Or old=%d value=%d", old, value)
		}
		if old := SyncAtomic_SwapUintptr(&value, 42); old != 11 || value != 42 {
			t.Fatalf("Swap old=%d value=%d", old, value)
		}
		if !SyncAtomic_CompareAndSwapUintptr(&value, 42, 7) || SyncAtomic_CompareAndSwapUintptr(&value, 42, 8) {
			t.Fatal("CAS state")
		}
		if new := SyncAtomic_AddUintptr(&value, 3); new != 10 {
			t.Fatalf("Add result=%d", new)
		}
		SyncAtomic_StoreUintptr(&value, 9)
		if got := SyncAtomic_LoadUintptr(&value); got != 9 {
			t.Fatalf("Load result=%d", got)
		}
	})
}
