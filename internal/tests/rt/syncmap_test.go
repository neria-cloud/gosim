//go:build sim

package behavior_test

import (
	"log/slog"
	"sync"
	"testing"
)

// The repeated-run metatest also checks checksums and Range order. Pointer
// addresses change between runs and must not affect either.
func TestSyncMapPointerKeys(t *testing.T) {
	var m sync.Map
	keys := make([]*int, 128)
	for i := range keys {
		keys[i] = new(int)
		*keys[i] = i
		m.Store(keys[i], i)
	}
	for i, key := range keys {
		if got, ok := m.Load(key); !ok || got != i {
			t.Fatalf("Load(%d) = %v, %v", i, got, ok)
		}
		if got, loaded := m.LoadOrStore(key, -1); !loaded || got != i {
			t.Fatalf("LoadOrStore(%d) = %v, %v", i, got, loaded)
		}
	}
	seen := make(map[*int]bool)
	m.Range(func(key, value any) bool {
		p := key.(*int)
		if seen[p] || *p != value.(int) {
			t.Fatal("Range duplicated a key or lost its value")
		}
		seen[p] = true
		slog.Info("sync map entry", "index", value)
		// Range must allow reentrant mutation and must not visit a key twice.
		m.Delete(key)
		m.Store(key, value)
		return true
	})
	if len(seen) != len(keys) {
		t.Fatalf("Range visited %d keys", len(seen))
	}
	m.Clear()
	m.Range(func(key, value any) bool {
		t.Fatal("Clear left an entry")
		return false
	})
	if previous, loaded := m.Swap(nil, nil); loaded || previous != nil {
		t.Fatal("Swap on empty map")
	}
	if value, loaded := m.LoadAndDelete(nil); !loaded || value != nil {
		t.Fatal("nil key/value not preserved")
	}
	if m.CompareAndSwap(nil, nil, 1) || m.CompareAndDelete(nil, nil) {
		t.Fatal("comparison succeeded on absent entry")
	}
}

func TestSyncMapConcurrentCompareAndSwap(t *testing.T) {
	var m sync.Map
	var wg sync.WaitGroup
	m.Store("counter", 0)
	for range 4 {
		wg.Go(func() {
			for range 20 {
				for {
					old, _ := m.Load("counter")
					if m.CompareAndSwap("counter", old, old.(int)+1) {
						break
					}
				}
			}
		})
	}
	wg.Wait()
	if got, _ := m.Load("counter"); got != 80 {
		t.Fatalf("lost an update: %v", got)
	}
	if m.CompareAndDelete("counter", 79) || !m.CompareAndDelete("counter", 80) {
		t.Fatal("CompareAndDelete did not compare values")
	}
}

func TestSyncMapPanicsAndRangeStop(t *testing.T) {
	var m sync.Map
	mustPanic := func(f func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Error("expected panic")
			}
		}()
		f()
	}
	mustPanic(func() { m.Load([]int{1}) })
	mustPanic(func() { m.Store([]int{1}, 1) })
	m.Store("slice", []int{1})
	mustPanic(func() { m.CompareAndSwap("slice", []int{1}, 2) })
	mustPanic(func() { m.CompareAndDelete("slice", []int{1}) })
	// A comparison panic must release the lock.
	m.Delete("slice")
	for i := range 10 {
		m.Store(i, i)
	}
	calls := 0
	m.Range(func(key, value any) bool { calls++; return false })
	if calls != 1 {
		t.Fatalf("Range ignored false: %d calls", calls)
	}
}
