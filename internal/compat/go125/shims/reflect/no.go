//go:build go1.25 && !go1.26

package reflect

import "unsafe"

const (
	reflectPrivateABIMessage  = "gosim go125 reflect: unsupported private reflect2 ABI"
	reflectMapTypeMessage     = "reflect: struct field operation on map Type"
	reflectMapValueMessage    = "reflect: struct field operation on map Value"
	reflectUnsupportedMessage = "gosim go125 reflect: unsupported operation"
)

// link names used by the reflect2 package, used indirectly by etcd. expose them
// to let the package compile. make all of them panic to prevent usage.

func unsafe_New(rtype unsafe.Pointer) unsafe.Pointer {
	panic(reflectPrivateABIMessage)
}

func typedmemmove(rtype unsafe.Pointer, dst, src unsafe.Pointer) {
	panic(reflectPrivateABIMessage)
}

func unsafe_NewArray(rtype unsafe.Pointer, length int) unsafe.Pointer {
	panic(reflectPrivateABIMessage)
}

type noSliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

func typedslicecopy(elemType unsafe.Pointer, dst, src noSliceHeader) int {
	panic(reflectPrivateABIMessage)
}

func mapassign(rtype unsafe.Pointer, m unsafe.Pointer, key, val unsafe.Pointer) {
	panic(reflectPrivateABIMessage)
}

func mapaccess(rtype unsafe.Pointer, m unsafe.Pointer, key unsafe.Pointer) (val unsafe.Pointer) {
	panic(reflectPrivateABIMessage)
}

type noHiter struct {
	key   unsafe.Pointer // Must be in first position.  Write nil to indicate iteration end (see cmd/internal/gc/range.go).
	value unsafe.Pointer // Must be in second position (see cmd/internal/gc/range.go).
	// rest fields are ignored
}

func mapiterinit(rtype unsafe.Pointer, m unsafe.Pointer) *noHiter {
	panic(reflectPrivateABIMessage)
}

func mapiternext(it *noHiter) {
	panic(reflectPrivateABIMessage)
}

func ifaceE2I(rtype unsafe.Pointer, src interface{}, dst unsafe.Pointer) {
	panic(reflectPrivateABIMessage)
}

func makemap(rtype unsafe.Pointer, cap int) (m unsafe.Pointer) {
	panic(reflectPrivateABIMessage)
}
