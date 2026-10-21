//go:build go1.25

package syncmap

import (
	"sync" //gosim:notranslate

	"github.com/neria-cloud/gosim/rt/runtime"
)

// Map models sync.Map using the deterministic translated map. Each operation
// yields before entering a non-yielding critical section. The private native
// mutex supplies host race synchronization; no callback or wait runs under it.
// This avoids exposing the host-address-dependent shape of Go's hash trie to
// the scheduler. Range takes a snapshot and invokes callbacks outside the lock.
type Map struct {
	mu     sync.Mutex
	values map[any]any
}

func (m *Map) lock() {
	gosimruntime.AtomicYield()
	m.mu.Lock()
}

// A nil or small translated map need not hash a key. Preserve sync.Map's panic
// for non-comparable dynamic keys even when no entry has yet been stored.
func checkKey(key any) { _ = key == key }

func (m *Map) Load(key any) (value any, ok bool) {
	checkKey(key)
	m.lock()
	defer m.mu.Unlock()
	value, ok = m.values[key]
	return
}

func (m *Map) Store(key, value any) { m.Swap(key, value) }

func (m *Map) Swap(key, value any) (previous any, loaded bool) {
	checkKey(key)
	m.lock()
	defer m.mu.Unlock()
	previous, loaded = m.values[key]
	if m.values == nil {
		m.values = make(map[any]any)
	}
	m.values[key] = value
	return
}

func (m *Map) LoadOrStore(key, value any) (actual any, loaded bool) {
	checkKey(key)
	m.lock()
	defer m.mu.Unlock()
	if actual, loaded = m.values[key]; !loaded {
		if m.values == nil {
			m.values = make(map[any]any)
		}
		m.values[key] = value
		actual = value
	}
	return
}

func (m *Map) LoadAndDelete(key any) (value any, loaded bool) {
	checkKey(key)
	m.lock()
	defer m.mu.Unlock()
	value, loaded = m.values[key]
	delete(m.values, key)
	return
}

func (m *Map) Delete(key any) { m.LoadAndDelete(key) }

func (m *Map) CompareAndSwap(key, old, new any) bool {
	checkKey(key)
	m.lock()
	defer m.mu.Unlock()
	if value, ok := m.values[key]; ok && value == old {
		m.values[key] = new
		return true
	}
	return false
}

func (m *Map) CompareAndDelete(key, old any) bool {
	checkKey(key)
	m.lock()
	defer m.mu.Unlock()
	if value, ok := m.values[key]; ok && value == old {
		delete(m.values, key)
		return true
	}
	return false
}

func (m *Map) Clear() {
	m.lock()
	defer m.mu.Unlock()
	clear(m.values)
}

func (m *Map) Range(f func(key, value any) bool) {
	for _, entry := range m.snapshot() {
		if !f(entry.key, entry.value) {
			return
		}
	}
}

type entry struct{ key, value any }

func (m *Map) snapshot() []entry {
	m.lock()
	defer m.mu.Unlock()
	entries := make([]entry, 0, len(m.values))
	for key, value := range m.values {
		entries = append(entries, entry{key, value})
	}
	return entries
}
