//go:build go1.25

// Package reflect2 provides translated type metadata for the pinned jsoniter
// dependency. Raw runtime type addresses and pointer-based value operations
// remain unsupported.
package reflect2

import (
	"reflect"
	"unsafe"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func unsupported(operation string) {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "github.com/modern-go/reflect2." + operation, Message: "raw-pointer reflection is not modeled"})
}

// These interfaces match reflect2 v1.0.1. A declaration permits type checking;
// each unsupported value operation below still aborts when executed.
type Type interface {
	Kind() reflect.Kind
	New() any
	UnsafeNew() unsafe.Pointer
	PackEFace(unsafe.Pointer) any
	Indirect(any) any
	UnsafeIndirect(unsafe.Pointer) any
	Type1() reflect.Type
	Implements(Type) bool
	String() string
	RType() uintptr
	LikePtr() bool
	IsNullable() bool
	IsNil(any) bool
	UnsafeIsNil(unsafe.Pointer) bool
	Set(any, any)
	UnsafeSet(unsafe.Pointer, unsafe.Pointer)
	AssignableTo(Type) bool
}

type ListType interface {
	Type
	Elem() Type
	SetIndex(any, int, any)
	UnsafeSetIndex(unsafe.Pointer, int, unsafe.Pointer)
	GetIndex(any, int) any
	UnsafeGetIndex(unsafe.Pointer, int) unsafe.Pointer
}

type ArrayType interface {
	ListType
	Len() int
}

type SliceType interface {
	ListType
	MakeSlice(int, int) any
	UnsafeMakeSlice(int, int) unsafe.Pointer
	Grow(any, int)
	UnsafeGrow(unsafe.Pointer, int)
	Append(any, any)
	UnsafeAppend(unsafe.Pointer, unsafe.Pointer)
	LengthOf(any) int
	UnsafeLengthOf(unsafe.Pointer) int
	SetNil(any)
	UnsafeSetNil(unsafe.Pointer)
	Cap(any) int
	UnsafeCap(unsafe.Pointer) int
}

type StructType interface {
	Type
	NumField() int
	Field(int) StructField
	FieldByName(string) StructField
	FieldByIndex([]int) StructField
	FieldByNameFunc(func(string) bool) StructField
}

type StructField interface {
	Offset() uintptr
	Name() string
	PkgPath() string
	Type() Type
	Tag() reflect.StructTag
	Index() []int
	Anonymous() bool
	Set(any, any)
	UnsafeSet(unsafe.Pointer, unsafe.Pointer)
	Get(any) any
	UnsafeGet(unsafe.Pointer) unsafe.Pointer
}

type MapType interface {
	Type
	Key() Type
	Elem() Type
	MakeMap(int) any
	UnsafeMakeMap(int) unsafe.Pointer
	SetIndex(any, any, any)
	UnsafeSetIndex(unsafe.Pointer, unsafe.Pointer, unsafe.Pointer)
	TryGetIndex(any, any) (any, bool)
	GetIndex(any, any) any
	UnsafeGetIndex(unsafe.Pointer, unsafe.Pointer) unsafe.Pointer
	Iterate(any) MapIterator
	UnsafeIterate(unsafe.Pointer) MapIterator
}

type MapIterator interface {
	HasNext() bool
	Next() (any, any)
	UnsafeNext() (unsafe.Pointer, unsafe.Pointer)
}

type PtrType interface {
	Type
	Elem() Type
}

type InterfaceType interface{ NumMethod() int }

type Config struct{ UseSafeImplementation bool }

type API interface {
	TypeOf(any) Type
	Type2(reflect.Type) Type
}

// The map belongs to translated configuration state. It is never a native
// global cache of simulated values or a cache keyed by runtime type addresses.
type frozenConfig struct {
	useSafeImplementation bool
	types                 map[reflect.Type]Type
}

var (
	ConfigUnsafe = Config{UseSafeImplementation: false}.Froze()
	ConfigSafe   = Config{UseSafeImplementation: true}.Froze()
)

func (cfg Config) Froze() *frozenConfig {
	return &frozenConfig{useSafeImplementation: cfg.UseSafeImplementation, types: make(map[reflect.Type]Type)}
}

func (cfg *frozenConfig) TypeOf(obj any) Type { return cfg.Type2(reflect.TypeOf(obj)) }

func (cfg *frozenConfig) Type2(typ reflect.Type) Type {
	if typ == nil {
		return nil
	}
	if known, ok := cfg.types[typ]; ok {
		return known
	}
	base := typeBase{cfg: cfg, typ: typ}
	var wrapper Type
	switch typ.Kind() {
	case reflect.Ptr, reflect.Chan, reflect.Func:
		wrapper = &UnsafePtrType{typeBase: base}
	case reflect.Array:
		wrapper = &UnsafeArrayType{listBase: listBase{typeBase: base}}
	case reflect.Slice:
		wrapper = &UnsafeSliceType{listBase: listBase{typeBase: base}}
	case reflect.Struct:
		wrapper = &UnsafeStructType{typeBase: base}
	case reflect.Map:
		wrapper = &UnsafeMapType{typeBase: base}
	case reflect.Interface:
		if typ.NumMethod() == 0 {
			wrapper = &UnsafeEFaceType{typeBase: base}
		} else {
			wrapper = &UnsafeIFaceType{typeBase: base}
		}
	default:
		wrapper = &base
	}
	cfg.types[typ] = wrapper
	return wrapper
}

func TypeOf(obj any) Type         { return ConfigUnsafe.TypeOf(obj) }
func TypeOfPtr(obj any) PtrType   { return TypeOf(obj).(PtrType) }
func Type2(typ reflect.Type) Type { return ConfigUnsafe.Type2(typ) }
func PtrTo(typ Type) Type         { return Type2(reflect.PtrTo(typ.Type1())) }

func DefaultTypeOfKind(kind reflect.Kind) Type {
	switch kind {
	case reflect.Bool:
		return TypeOf(false)
	case reflect.Uint8:
		return TypeOf(uint8(0))
	case reflect.Int8:
		return TypeOf(int8(0))
	case reflect.Uint16:
		return TypeOf(uint16(0))
	case reflect.Int16:
		return TypeOf(int16(0))
	case reflect.Uint32:
		return TypeOf(uint32(0))
	case reflect.Int32:
		return TypeOf(int32(0))
	case reflect.Uint64:
		return TypeOf(uint64(0))
	case reflect.Int64:
		return TypeOf(int64(0))
	case reflect.Uint:
		return TypeOf(uint(0))
	case reflect.Int:
		return TypeOf(int(0))
	case reflect.Float32:
		return TypeOf(float32(0))
	case reflect.Float64:
		return TypeOf(float64(0))
	case reflect.Uintptr:
		return TypeOf(uintptr(0))
	case reflect.String:
		return TypeOf("")
	case reflect.UnsafePointer:
		return TypeOf(unsafe.Pointer(nil))
	default:
		return nil
	}
}

func IsNullable(kind reflect.Kind) bool {
	switch kind {
	case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Func, reflect.Slice, reflect.Interface:
		return true
	default:
		return false
	}
}

func IsNil(obj any) bool {
	if obj == nil {
		return true
	}
	v := reflect.ValueOf(obj)
	return IsNullable(v.Kind()) && v.IsNil()
}

// TODO(not planned): raw type-address and value-pointer extraction require a
// reviewed translated memory contract, including run and epoch ownership.
func PtrOf(any) unsafe.Pointer { unsupported("PtrOf"); panic("abort returned") }
func RTypeOf(any) uintptr      { unsupported("RTypeOf"); panic("abort returned") }

type typeBase struct {
	cfg *frozenConfig
	typ reflect.Type
}

func (t *typeBase) Kind() reflect.Kind           { return t.typ.Kind() }
func (t *typeBase) Type1() reflect.Type          { return t.typ }
func (t *typeBase) String() string               { return t.typ.String() }
func (t *typeBase) Implements(other Type) bool   { return t.typ.Implements(other.Type1()) }
func (t *typeBase) AssignableTo(other Type) bool { return t.typ.AssignableTo(other.Type1()) }
func (t *typeBase) IsNullable() bool             { return IsNullable(t.typ.Kind()) }
func (t *typeBase) IsNil(obj any) bool           { return IsNil(obj) }
func (t *typeBase) LikePtr() bool                { return likePtr(t.typ) }

func likePtr(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Func:
		return true
	case reflect.Struct:
		return typ.NumField() == 1 && likePtr(typ.Field(0).Type)
	case reflect.Array:
		return typ.Len() == 1 && likePtr(typ.Elem())
	default:
		return false
	}
}

// TODO(not planned): all value construction and mutation below require
// translated value ownership and must fail fatally until implemented.
func (t *typeBase) New() any                  { unsupported("Type.New"); panic("abort returned") }
func (t *typeBase) UnsafeNew() unsafe.Pointer { unsupported("Type.UnsafeNew"); panic("abort returned") }
func (t *typeBase) PackEFace(unsafe.Pointer) any {
	unsupported("Type.PackEFace")
	panic("abort returned")
}
func (t *typeBase) Indirect(any) any { unsupported("Type.Indirect"); panic("abort returned") }
func (t *typeBase) UnsafeIndirect(unsafe.Pointer) any {
	unsupported("Type.UnsafeIndirect")
	panic("abort returned")
}
func (t *typeBase) RType() uintptr { unsupported("Type.RType"); panic("abort returned") }
func (t *typeBase) UnsafeIsNil(unsafe.Pointer) bool {
	unsupported("Type.UnsafeIsNil")
	panic("abort returned")
}
func (t *typeBase) Set(any, any)                             { unsupported("Type.Set") }
func (t *typeBase) UnsafeSet(unsafe.Pointer, unsafe.Pointer) { unsupported("Type.UnsafeSet") }

type listBase struct{ typeBase }

func (t *listBase) Elem() Type             { return t.cfg.Type2(t.typ.Elem()) }
func (t *listBase) SetIndex(any, int, any) { unsupported("ListType.SetIndex") }
func (t *listBase) UnsafeSetIndex(unsafe.Pointer, int, unsafe.Pointer) {
	unsupported("ListType.UnsafeSetIndex")
}
func (t *listBase) GetIndex(any, int) any { unsupported("ListType.GetIndex"); panic("abort returned") }
func (t *listBase) UnsafeGetIndex(unsafe.Pointer, int) unsafe.Pointer {
	unsupported("ListType.UnsafeGetIndex")
	panic("abort returned")
}

// The concrete names are part of jsoniter v1.1.11's source-level contract.
// They do not expose the upstream private reflect ABI in Gosim.
type (
	UnsafePtrType    struct{ typeBase }
	UnsafeArrayType  struct{ listBase }
	UnsafeSliceType  struct{ listBase }
	UnsafeStructType struct{ typeBase }
	UnsafeMapType    struct{ typeBase }
	UnsafeIFaceType  struct{ typeBase }
	UnsafeEFaceType  struct{ typeBase }
)

func (t *UnsafePtrType) Elem() Type       { return t.cfg.Type2(t.typ.Elem()) }
func (t *UnsafeArrayType) Len() int       { return t.typ.Len() }
func (t *UnsafeIFaceType) NumMethod() int { return t.typ.NumMethod() }
func (t *UnsafeEFaceType) NumMethod() int { return t.typ.NumMethod() }

func (t *UnsafeSliceType) MakeSlice(int, int) any {
	unsupported("SliceType.MakeSlice")
	panic("abort returned")
}

func (t *UnsafeSliceType) UnsafeMakeSlice(int, int) unsafe.Pointer {
	unsupported("SliceType.UnsafeMakeSlice")
	panic("abort returned")
}
func (t *UnsafeSliceType) Grow(any, int)                  { unsupported("SliceType.Grow") }
func (t *UnsafeSliceType) UnsafeGrow(unsafe.Pointer, int) { unsupported("SliceType.UnsafeGrow") }
func (t *UnsafeSliceType) Append(any, any)                { unsupported("SliceType.Append") }
func (t *UnsafeSliceType) UnsafeAppend(unsafe.Pointer, unsafe.Pointer) {
	unsupported("SliceType.UnsafeAppend")
}

func (t *UnsafeSliceType) LengthOf(any) int {
	unsupported("SliceType.LengthOf")
	panic("abort returned")
}

func (t *UnsafeSliceType) UnsafeLengthOf(unsafe.Pointer) int {
	unsupported("SliceType.UnsafeLengthOf")
	panic("abort returned")
}
func (t *UnsafeSliceType) SetNil(any)                  { unsupported("SliceType.SetNil") }
func (t *UnsafeSliceType) UnsafeSetNil(unsafe.Pointer) { unsupported("SliceType.UnsafeSetNil") }
func (t *UnsafeSliceType) Cap(any) int                 { unsupported("SliceType.Cap"); panic("abort returned") }
func (t *UnsafeSliceType) UnsafeCap(unsafe.Pointer) int {
	unsupported("SliceType.UnsafeCap")
	panic("abort returned")
}

func (t *UnsafeStructType) NumField() int           { return t.typ.NumField() }
func (t *UnsafeStructType) Field(i int) StructField { return t.field(t.typ.Field(i)) }
func (t *UnsafeStructType) FieldByIndex(index []int) StructField {
	return t.field(t.typ.FieldByIndex(index))
}

func (t *UnsafeStructType) FieldByName(name string) StructField {
	field, ok := t.typ.FieldByName(name)
	if !ok {
		return nil
	}
	return t.field(field)
}

func (t *UnsafeStructType) FieldByNameFunc(match func(string) bool) StructField {
	field, ok := t.typ.FieldByNameFunc(match)
	if !ok {
		return nil
	}
	return t.field(field)
}

func (t *UnsafeStructType) field(field reflect.StructField) StructField {
	return &structField{cfg: t.cfg, field: field}
}

type structField struct {
	cfg   *frozenConfig
	field reflect.StructField
}

func (f *structField) Offset() uintptr                          { return f.field.Offset }
func (f *structField) Name() string                             { return f.field.Name }
func (f *structField) PkgPath() string                          { return f.field.PkgPath }
func (f *structField) Type() Type                               { return f.cfg.Type2(f.field.Type) }
func (f *structField) Tag() reflect.StructTag                   { return f.field.Tag }
func (f *structField) Index() []int                             { return append([]int(nil), f.field.Index...) }
func (f *structField) Anonymous() bool                          { return f.field.Anonymous }
func (f *structField) Set(any, any)                             { unsupported("StructField.Set") }
func (f *structField) UnsafeSet(unsafe.Pointer, unsafe.Pointer) { unsupported("StructField.UnsafeSet") }
func (f *structField) Get(any) any                              { unsupported("StructField.Get"); panic("abort returned") }
func (f *structField) UnsafeGet(unsafe.Pointer) unsafe.Pointer {
	unsupported("StructField.UnsafeGet")
	panic("abort returned")
}

func (t *UnsafeMapType) Key() Type       { return t.cfg.Type2(t.typ.Key()) }
func (t *UnsafeMapType) Elem() Type      { return t.cfg.Type2(t.typ.Elem()) }
func (t *UnsafeMapType) MakeMap(int) any { unsupported("MapType.MakeMap"); panic("abort returned") }
func (t *UnsafeMapType) UnsafeMakeMap(int) unsafe.Pointer {
	unsupported("MapType.UnsafeMakeMap")
	panic("abort returned")
}
func (t *UnsafeMapType) SetIndex(any, any, any) { unsupported("MapType.SetIndex") }
func (t *UnsafeMapType) UnsafeSetIndex(unsafe.Pointer, unsafe.Pointer, unsafe.Pointer) {
	unsupported("MapType.UnsafeSetIndex")
}

func (t *UnsafeMapType) TryGetIndex(any, any) (any, bool) {
	unsupported("MapType.TryGetIndex")
	panic("abort returned")
}

func (t *UnsafeMapType) GetIndex(any, any) any {
	unsupported("MapType.GetIndex")
	panic("abort returned")
}

func (t *UnsafeMapType) UnsafeGetIndex(unsafe.Pointer, unsafe.Pointer) unsafe.Pointer {
	unsupported("MapType.UnsafeGetIndex")
	panic("abort returned")
}

func (t *UnsafeMapType) Iterate(any) MapIterator {
	unsupported("MapType.Iterate")
	panic("abort returned")
}

func (t *UnsafeMapType) UnsafeIterate(unsafe.Pointer) MapIterator {
	unsupported("MapType.UnsafeIterate")
	panic("abort returned")
}

var (
	_ API           = (*frozenConfig)(nil)
	_ PtrType       = (*UnsafePtrType)(nil)
	_ ArrayType     = (*UnsafeArrayType)(nil)
	_ SliceType     = (*UnsafeSliceType)(nil)
	_ StructType    = (*UnsafeStructType)(nil)
	_ MapType       = (*UnsafeMapType)(nil)
	_ InterfaceType = (*UnsafeIFaceType)(nil)
)
