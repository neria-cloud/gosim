//go:build go1.25 && !go1.26

package reflect

import "unsafe"

// link names used by the reflect2 package, used indirectly by etcd. expose them
// to let the package compile. make all of them panic to prevent usage.

func unsafe_New(_ unsafe.Pointer) unsafe.Pointer {
	panic("gosim go125 reflect: unsupported private reflect2 ABI")
}

func typedmemmove(_ unsafe.Pointer, dst, src unsafe.Pointer) {
	panic("gosim go125 reflect: unsupported private reflect2 ABI")
}

func unsafe_NewArray(_ unsafe.Pointer, length int) unsafe.Pointer {
	panic("gosim go125 reflect: unsupported private reflect2 ABI")
}

type noSliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

func typedslicecopy(_ unsafe.Pointer, dst, src noSliceHeader) int {
	panic("gosim go125 reflect: unsupported private reflect2 ABI")
}

func mapassign(_ unsafe.Pointer, m unsafe.Pointer, key, val unsafe.Pointer) {
	panic("gosim go125 reflect: unsupported private reflect2 ABI")
}

func mapaccess(_ unsafe.Pointer, m unsafe.Pointer, key unsafe.Pointer) (val unsafe.Pointer) {
	panic("gosim go125 reflect: unsupported private reflect2 ABI")
}

type noHiter struct {
	key   unsafe.Pointer // Must be in first position.  Write nil to indicate iteration end (see cmd/internal/gc/range.go).
	value unsafe.Pointer // Must be in second position (see cmd/internal/gc/range.go).
	// rest fields are ignored
}

func mapiterinit(_ unsafe.Pointer, m unsafe.Pointer) *noHiter {
	panic("gosim go125 reflect: unsupported private reflect2 ABI")
}

func mapiternext(_ *noHiter) {
	panic("gosim go125 reflect: unsupported private reflect2 ABI")
}

func ifaceE2I(_ unsafe.Pointer, src interface{}, dst unsafe.Pointer) {
	panic("gosim go125 reflect: unsupported private reflect2 ABI")
}

func makemap(_ unsafe.Pointer, cap int) (m unsafe.Pointer) {
	panic("gosim go125 reflect: unsupported private reflect2 ABI")
}
