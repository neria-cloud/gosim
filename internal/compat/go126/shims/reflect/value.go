//go:build go1.26 && !go1.27

// Copyright 2009 The Go Authors. All rights reserved.  Use of this source code
// is governed by a BSD-style license that can be found at
// https://go.googlesource.com/go/+/refs/heads/master/LICENSE.

// Based on https://go.googlesource.com/go/+/refs/heads/master/src/reflect/value.go.

package reflect

import (
	"iter"    //gosim:notranslate
	"reflect" //gosim:notranslate
	"unsafe"

	"github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type wrapKind byte

const (
	normal wrapKind = iota
	wrappedMap
	wrappedChan
)

type Value struct {
	inner reflect.Value
	kind  wrapKind
}

func wrapValue(inner reflect.Value) Value {
	if inner.IsValid() && isRuntimeChan(inner.Type()) {
		panic("gosim go126 reflect: reflected channel operations are unsupported")
	}
	if inner.Kind() == reflect.Map || inner.Kind() == reflect.Chan {
		panic("gosim go126 reflect: native map/channel crossed the simulation boundary")
	}

	if inner.Kind() == reflect.Struct && inner.NumField() == 1 {
		field := inner.Type().Field(0)
		if field.Name == "Impl" && field.Type.Implements(mapInterfaceType) {
			return Value{
				inner: inner,
				kind:  wrappedMap,
			}
		}
	}

	return Value{
		inner: inner,
	}
}

func wrapValues(x []reflect.Value) []Value {
	outer := make([]Value, len(x))
	for i, v := range x {
		outer[i] = wrapValue(v)
	}
	return outer
}

func unwrapValues(x []Value) []reflect.Value {
	inner := make([]reflect.Value, len(x))
	for i, v := range x {
		inner[i] = v.inner
	}
	return inner
}

func Append(s Value, x ...Value) Value {
	return wrapValue(reflect.Append(s.inner, unwrapValues(x)...))
}

// TypeAssert returns the value held by v as type T, reporting whether the
// assertion succeeded. It is equivalent to v.Interface().(T).
// Added in Go 1.25.
func TypeAssert[T any](v Value) (T, bool) {
	t, ok := v.inner.Interface().(T)
	return t, ok
}

func AppendSlice(s, t Value) Value {
	return wrapValue(reflect.AppendSlice(s.inner, t.inner))
}

func Copy(dst, src Value) int {
	return reflect.Copy(dst.inner, src.inner)
}

type SelectDir int

// NOTE: These values must match ../runtime/select.go:/selectDir.

const (
	_ SelectDir = iota
	SelectSend
	SelectRecv
	SelectDefault
)

type SelectCase struct {
	Dir  SelectDir
	Chan Value
	Send Value
}

func Select(cases []SelectCase) (chosen int, recv Value, recvOK bool) {
	panic("gosim go126 reflect: unsupported operation")
}

func MakeSlice(typ Type, len, cap int) Value {
	return wrapValue(reflect.MakeSlice(typ.(*typeImpl).inner, len, cap))
}

func MakeChan(typ Type, buffer int) Value {
	panic("gosim go126 reflect: unsupported operation")
}

func getDecriptor(typ Type) gosimruntime.ReflectMapType {
	implPtrTyp := typ.(*typeImpl).inner.Field(0).Type
	zeroImplPtr := reflect.Zero(implPtrTyp)
	mapInterface := zeroImplPtr.Interface().(gosimruntime.ReflectMap)
	return mapInterface.Type()
}

func MakeMap(typ Type) Value {
	if typ.Kind() != reflect.Map {
		reflect.MakeMap(typ.(*typeImpl).inner)
		panic("unreachable")
	}

	descriptor := getDecriptor(typ)
	return ValueOf(descriptor.Make())
}

func MakeMapWithSize(typ Type, n int) Value {
	if n < 0 {
		panic("reflect.MakeMapWithSize: negative size")
	}
	return MakeMap(typ)
}

func Indirect(v Value) Value {
	return wrapValue(reflect.Indirect(v.inner))
}

func ValueOf(i any) Value {
	return wrapValue(reflect.ValueOf(i))
}

func Zero(typ Type) Value {
	return wrapValue(reflect.Zero(typ.(*typeImpl).inner))
}

func New(typ Type) Value {
	return wrapValue(reflect.New(typ.(*typeImpl).inner))
}

func NewAt(typ Type, p unsafe.Pointer) Value {
	return wrapValue(reflect.NewAt(typ.(*typeImpl).inner, p))
}

func (v Value) Addr() Value {
	return wrapValue(v.inner.Addr())
}

func (v Value) Bool() bool {
	return v.inner.Bool()
}

func (v Value) Bytes() []byte {
	return v.inner.Bytes()
}

func (v Value) CanAddr() bool {
	return v.inner.CanAddr()
}

func (v Value) CanSet() bool {
	return v.inner.CanSet()
}

func (v Value) Call(in []Value) []Value {
	return wrapValues(v.inner.Call(unwrapValues(in)))
}

func (v Value) CallSlice(in []Value) []Value {
	return wrapValues(v.inner.CallSlice(unwrapValues(in)))
}

func (v Value) Cap() int {
	return v.inner.Cap()
}

func (v Value) Close() {
	v.inner.Close()
}

func (v Value) CanComplex() bool {
	return v.inner.CanComplex()
}

func (v Value) Complex() complex128 {
	return v.inner.Complex()
}

func (v Value) Elem() Value {
	return wrapValue(v.inner.Elem())
}

func (v Value) Field(i int) Value {
	if v.kind == wrappedMap {
		panic("reflect: struct field operation on map Value")
	}
	return wrapValue(v.inner.Field(i))
}

func (v Value) FieldByIndex(index []int) Value {
	if v.kind == wrappedMap {
		panic("reflect: struct field operation on map Value")
	}
	return wrapValue(v.inner.FieldByIndex(index))
}

func (v Value) FieldByIndexErr(index []int) (Value, error) {
	if v.kind == wrappedMap {
		panic("reflect: struct field operation on map Value")
	}
	inner, err := v.inner.FieldByIndexErr(index)
	return wrapValue(inner), err
}

func (v Value) FieldByName(name string) Value {
	if v.kind == wrappedMap {
		panic("reflect: struct field operation on map Value")
	}
	return wrapValue(v.inner.FieldByName(name))
}

func (v Value) FieldByNameFunc(match func(string) bool) Value {
	if v.kind == wrappedMap {
		panic("reflect: struct field operation on map Value")
	}
	return wrapValue(v.inner.FieldByNameFunc(match))
}

func (v Value) CanFloat() bool {
	return v.inner.CanFloat()
}

func (v Value) Float() float64 {
	return v.inner.Float()
}

func (v Value) Index(i int) Value {
	return wrapValue(v.inner.Index(i))
}

func (v Value) CanInt() bool {
	return v.inner.CanInt()
}

func (v Value) Int() int64 {
	return v.inner.Int()
}

func (v Value) CanInterface() bool {
	return v.inner.CanInterface()
}

func (v Value) Interface() (i any) {
	return v.inner.Interface()
}

func (v Value) InterfaceData() [2]uintptr {
	return v.inner.InterfaceData()
}

func (v Value) IsNil() bool {
	switch v.kind {
	case wrappedMap:
		return v.inner.Field(0).IsNil()
	default:
		return v.inner.IsNil()
	}
}

func (v Value) IsValid() bool {
	return v.inner.IsValid()
}

func (v Value) IsZero() bool {
	return v.inner.IsZero()
}

func (v Value) SetZero() {
	v.inner.SetZero()
}

func (v Value) Kind() Kind {
	switch v.kind {
	case normal:
		return v.inner.Kind()
	case wrappedMap:
		return Map
	default:
		panic("help")
	}
}

func (v Value) Len() int {
	switch v.kind {
	case normal:
		return v.inner.Len()
	case wrappedMap:
		return getMapInterface(v).Len()
	default:
		panic("help")
	}
}

func (v Value) MapIndex(key Value) Value {
	value, ok := getMapInterface(v).GetIndex(key.inner)
	if !ok {
		return Value{}
	}
	return wrapValue(value)
}

func (v Value) MapKeys() []Value {
	mapIface := getMapInterface(v)
	values := make([]Value, 0, mapIface.Len())
	iter := mapIface.Iter()
	for iter.Next() {
		values = append(values, wrapValue(iter.Key()))
	}
	return values
}

type MapIter struct {
	inner gosimruntime.ReflectMapIter
	valid bool
}

func (iter *MapIter) Key() Value {
	if !iter.valid {
		panic("reflect: MapIter.Key called without a current entry")
	}
	return wrapValue(iter.inner.Key())
}

func (v Value) SetIterKey(iter *MapIter) {
	v.Set(iter.Key())
}

func (iter *MapIter) Value() Value {
	if !iter.valid {
		panic("reflect: MapIter.Value called without a current entry")
	}
	return wrapValue(iter.inner.Value())
}

func (v Value) SetIterValue(iter *MapIter) {
	v.Set(iter.Value())
}

func (iter *MapIter) Next() bool {
	iter.valid = iter.inner.Next()
	return iter.valid
}

type zeroIter struct{}

func (z zeroIter) Value() reflect.Value {
	panic("gosim go126 reflect: unsupported operation")
}

func (z zeroIter) Key() reflect.Value {
	panic("gosim go126 reflect: unsupported operation")
}

func (z zeroIter) Next() bool {
	return false
}

func (iter *MapIter) Reset(v Value) {
	iter.valid = false
	if !v.IsValid() {
		iter.inner = zeroIter{}
		return
	}

	switch v.kind {
	case wrappedMap:
		inner := v.inner.Field(0).Interface().(gosimruntime.ReflectMap).Iter()
		iter.inner = inner
	default:
		var iter reflect.MapIter
		iter.Reset(v.inner) // will panic because this is not a map
		panic("unreachable")
	}
}

func getMapInterface(v Value) gosimruntime.ReflectMap {
	return v.inner.Field(0).Interface().(gosimruntime.ReflectMap)
}

func (v Value) MapRange() *MapIter {
	switch v.kind {
	case wrappedMap:
		inner := getMapInterface(v).Iter()
		return &MapIter{inner: inner}
	default:
		v.inner.MapRange() // will panic because this is not a map
		panic("unreachable")
	}
}

func (v Value) Method(i int) Value {
	return wrapValue(v.inner.Method(i))
}

func (v Value) NumMethod() int {
	return v.inner.NumMethod()
}

func (v Value) MethodByName(name string) Value {
	return wrapValue(v.inner.MethodByName(name))
}

func (v Value) NumField() int {
	if v.kind == wrappedMap {
		panic("reflect: struct field operation on map Value")
	}
	return v.inner.NumField()
}

func (v Value) OverflowComplex(x complex128) bool {
	return v.inner.OverflowComplex(x)
}

func (v Value) OverflowFloat(x float64) bool {
	return v.inner.OverflowFloat(x)
}

func (v Value) OverflowInt(x int64) bool {
	return v.inner.OverflowInt(x)
}

func (v Value) OverflowUint(x uint64) bool {
	return v.inner.OverflowUint(x)
}

// This prevents inlining Value.Pointer when -d=checkptr is enabled,
// which ensures cmd/compile can recognize unsafe.Pointer(v.Pointer())
// and make an exception.
//
//go:nocheckptr
func (v Value) Pointer() uintptr {
	switch v.kind {
	case wrappedMap:
		return uintptr(getMapInterface(v).UnsafePointer())
	default:
		return v.inner.Pointer()
	}
}

func (v Value) Recv() (x Value, ok bool) {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "reflect.Value.Recv", Message: "reflected channel scheduling is not modeled"})
	panic("abort returned")
}

func (v Value) Send(x Value) {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "reflect.Value.Send", Message: "reflected channel scheduling is not modeled"})
	panic("abort returned")
}

func (v Value) Set(x Value) {
	v.inner.Set(x.inner)
}

func (v Value) SetBool(x bool) {
	v.inner.SetBool(x)
}

func (v Value) SetBytes(x []byte) {
	v.inner.SetBytes(x)
}

func (v Value) SetComplex(x complex128) {
	v.inner.SetComplex(x)
}

func (v Value) SetFloat(x float64) {
	v.inner.SetFloat(x)
}

func (v Value) SetInt(x int64) {
	v.inner.SetInt(x)
}

func (v Value) SetLen(n int) {
	v.inner.SetLen(n)
}

func (v Value) SetCap(n int) {
	v.inner.SetCap(n)
}

func (v Value) SetMapIndex(key, elem Value) {
	getMapInterface(v).SetIndex(key.inner, elem.inner)
}

func (v Value) SetUint(x uint64) {
	v.inner.SetUint(x)
}

func (v Value) SetPointer(x unsafe.Pointer) {
	v.inner.SetPointer(x)
}

func (v Value) SetString(x string) {
	v.inner.SetString(x)
}

func (v Value) Slice(i, j int) Value {
	return wrapValue(v.inner.Slice(i, j))
}

func (v Value) Slice3(i, j, k int) Value {
	return wrapValue(v.inner.Slice3(i, j, k))
}

func (v Value) String() string {
	return v.inner.String()
}

func (v Value) TryRecv() (x Value, ok bool) {
	panic("gosim go126 reflect: unsupported operation")
}

func (v Value) TrySend(x Value) bool {
	panic("gosim go126 reflect: unsupported operation")
}

func (v Value) Type() Type {
	return wrapType(v.inner.Type())
}

func (v Value) CanUint() bool {
	return v.inner.CanUint()
}

func (v Value) Uint() uint64 {
	return v.inner.Uint()
}

// This prevents inlining Value.UnsafeAddr when -d=checkptr is enabled,
// which ensures cmd/compile can recognize unsafe.Pointer(v.UnsafeAddr())
// and make an exception.
//
//go:nocheckptr
func (v Value) UnsafeAddr() uintptr {
	switch v.kind {
	case wrappedMap:
		return uintptr(getMapInterface(v).UnsafePointer())
	default:
		return v.inner.UnsafeAddr()
	}
}

func (v Value) UnsafePointer() unsafe.Pointer {
	switch v.kind {
	case wrappedMap:
		return getMapInterface(v).UnsafePointer()
	default:
		return v.inner.UnsafePointer()
	}
}

type StringHeader = reflect.StringHeader

type SliceHeader = reflect.SliceHeader

func (v Value) Grow(n int) {
	v.inner.Grow(n)
}

func (v Value) Clear() {
	if v.kind == wrappedMap {
		for _, key := range v.MapKeys() {
			v.SetMapIndex(key, Value{})
		}
		return
	}
	v.inner.Clear()
}

func (v Value) Convert(t Type) Value {
	return wrapValue(v.inner.Convert(t.(*typeImpl).inner))
}

func (v Value) CanConvert(t Type) bool {
	return v.inner.CanConvert(t.(*typeImpl).inner)
}

func (v Value) Comparable() bool {
	if v.kind == wrappedMap {
		return false
	}
	return v.inner.Comparable()
}

func (v Value) Equal(u Value) bool {
	if v.kind == wrappedMap || u.kind == wrappedMap {
		panic("reflect.Value.Equal: maps are not comparable")
	}
	return v.inner.Equal(u.inner)
}

func (v Value) Seq() iter.Seq[Value] {
	if v.kind == wrappedMap {
		return func(yield func(Value) bool) {
			it := v.MapRange()
			for it.Next() {
				if !yield(it.Key()) {
					return
				}
			}
		}
	}

	return func(yield func(Value) bool) {
		v.inner.Seq()(func(rv reflect.Value) bool {
			return yield(wrapValue(rv))
		})
	}
}

func (v Value) Seq2() iter.Seq2[Value, Value] {
	if v.kind == wrappedMap {
		return func(yield func(Value, Value) bool) {
			it := v.MapRange()
			for it.Next() {
				if !yield(it.Key(), it.Value()) {
					return
				}
			}
		}
	}

	return func(yield func(Value, Value) bool) {
		v.inner.Seq2()(func(k, val reflect.Value) bool {
			return yield(wrapValue(k), wrapValue(val))
		})
	}
}

func SliceAt(typ Type, p unsafe.Pointer, n int) Value {
	return wrapValue(reflect.SliceAt(typ.(*typeImpl).inner, p, n))
}
