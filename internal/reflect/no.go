//go:build gosim_retired

package reflect

import "unsafe"

// link names used by the reflect2 package, used indirectly by etcd. expose them
// to let the package compile. make all of them panic to prevent usage.

func unsafe_New(rtype unsafe.Pointer) unsafe.Pointer {
	panic("no")
}

func typedmemmove(rtype unsafe.Pointer, dst, src unsafe.Pointer) {
	panic("no")
}

func unsafe_NewArray(rtype unsafe.Pointer, length int) unsafe.Pointer {
	panic("no")
}

type noSliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

func typedslicecopy(elemType unsafe.Pointer, dst, src noSliceHeader) int {
	panic("no")
}

func mapassign(rtype unsafe.Pointer, m unsafe.Pointer, key, val unsafe.Pointer) {
	panic("no")
}

func mapaccess(rtype unsafe.Pointer, m unsafe.Pointer, key unsafe.Pointer) (val unsafe.Pointer) {
	panic("no")
}

type noHiter struct {
	key   unsafe.Pointer // Must be in first position.  Write nil to indicate iteration end (see cmd/internal/gc/range.go).
	value unsafe.Pointer // Must be in second position (see cmd/internal/gc/range.go).
	// rest fields are ignored
}

func mapiterinit(rtype unsafe.Pointer, m unsafe.Pointer) *noHiter {
	panic("no")
}

func mapiternext(it *noHiter) {
	panic("no")
}

func ifaceE2I(rtype unsafe.Pointer, src interface{}, dst unsafe.Pointer) {
	panic("no")
}

func makemap(rtype unsafe.Pointer, cap int) (m unsafe.Pointer) {
	panic("no")
}
