package abi_test

import (
	"slices"
	"testing"
	"unsafe"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestViewsBoundAndBorrowMemory(t *testing.T) {
	data := []byte{1, 2, 3, 4}
	var view abi.ByteView = abi.NewSliceView(&data[0], uintptr(len(data)))
	into := make([]byte, 8)
	if n := view.Read(into); n != 4 || !slices.Equal(into, []byte{1, 2, 3, 4, 0, 0, 0, 0}) {
		t.Fatalf("read exceeded bounds: n=%d, %v", n, into)
	}
	if n := view.Slice(1, 3).Write([]byte{8, 9, 10}); n != 2 || !slices.Equal(data, []byte{1, 8, 9, 4}) {
		t.Fatalf("write exceeded bounds: n=%d, %v", n, data)
	}
	if n := view.SliceFrom(3).Read(into[:1]); n != 1 || into[0] != 4 {
		t.Fatal("slice offset lost")
	}
	if view.Len() != 4 || view.SliceFrom(4).Len() != 0 {
		t.Fatal("unexpected bounds")
	}
	if empty := abi.NewSliceView[byte](nil, 0); empty.Len() != 0 || empty.Read(into) != 0 || empty.Write(data) != 0 {
		t.Fatal("empty view accessed memory")
	}
	x := uint64(3)
	v := abi.NewValueView(&x)
	if v.Get() != 3 || v.UnsafePointer() != unsafe.Pointer(&x) {
		t.Fatal("value view changed pointer identity")
	}
	v.Set(17)
	if x != 17 {
		t.Fatal("value view did not write borrowed memory")
	}
}

func TestViewSliceRejectsOutOfBounds(t *testing.T) {
	view := abi.ByteView{Ptr: []byte{1, 2}}
	for _, slice := range []func(){func() { view.Slice(-1, 1) }, func() { view.Slice(0, 3) }, func() { view.SliceFrom(3) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("out-of-bounds view accepted")
				}
			}()
			slice()
		}()
	}
}

func TestBoolWireRepresentation(t *testing.T) {
	if abi.BoolToUintptr(false) != 0 || abi.BoolToUintptr(true) != 1 || abi.BoolFromUintptr(0) || !abi.BoolFromUintptr(2) {
		t.Fatal("boolean wire representation changed")
	}
}
