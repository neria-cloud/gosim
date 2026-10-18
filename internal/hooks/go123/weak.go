package go123

import "unsafe"

// Go 1.25: weak package hooks for weak pointer support.
// In the simulation we treat weak pointers as strong pointers — they never
// return nil from Value(). This is safe for tests that don't rely on
// GC-driven finalization of weak references.

func Weak_runtime_registerWeakPointer(ptr unsafe.Pointer) unsafe.Pointer {
	return ptr
}

func Weak_runtime_makeStrongFromWeak(ptr unsafe.Pointer) unsafe.Pointer {
	return ptr
}
