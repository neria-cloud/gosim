//go:build go1.25 && !go1.26

package reflect

import (
	"testing"
	"unsafe"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

func TestReflectedMapBoundary(t *testing.T) {
	e, err := gosimruntime.NewEngine(
		gosimruntime.EngineConfig{Seed: 4, TimeoutNS: 1e9, ReplayRevision: gosimruntime.ReplayRevision},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := e.Run(func() {
		m := gosimruntime.NewMap[string, int]()
		m.Set("a", 1)
		m.Set("b", 2)
		value := ValueOf(m)
		mustPanic(t, func() { value.Field(0) })
		mustPanic(t, func() { value.Type().Field(0) })
		mustPanic(t, func() { MakeMapWithSize(value.Type(), -1) })
		if value.Kind() != Map || value.Len() != 2 || value.Comparable() || value.Type().Comparable() || !value.Type().CanSeq() || !value.Type().CanSeq2() {
			t.Fatal("wrong reflected map type")
		}
		if value.MapIndex(ValueOf("absent")).IsValid() {
			t.Fatal("missing key returned valid zero")
		}
		if got := value.MapIndex(ValueOf("a")).Int(); got != 1 {
			t.Fatalf("map lookup=%d", got)
		}
		keys, values := 0, int64(0)
		value.Seq2()(func(_, v Value) bool { keys++; values += v.Int(); return true })
		if keys != 2 || values != 3 {
			t.Fatalf("Seq2 keys=%d values=%d", keys, values)
		}
		keys = 0
		value.Seq()(func(_ Value) bool { keys++; return false })
		if keys != 1 {
			t.Fatal("Seq ignored early stop")
		}
		it := value.MapRange()
		mustPanic(t, func() { it.Key() })
		if !it.Next() {
			t.Fatal("empty iterator")
		}
		_ = it.Value()
		it.Reset(Value{})
		if it.Next() {
			t.Fatal("reset iterator retained entries")
		}
		mustPanic(t, func() { it.Value() })
		it.Reset(value)
		if !it.Next() {
			t.Fatal("iterator could not be reused")
		}
		value.SetMapIndex(ValueOf("a"), Value{})
		if _, ok := m.GetOk("a"); ok {
			t.Fatal("invalid Value did not delete map key")
		}
		clone := MakeMap(value.Type())
		clone.SetMapIndex(ValueOf("x"), ValueOf(9))
		if clone.Len() != 1 || value.Len() != 1 {
			t.Fatal("MakeMap did not create independent storage")
		}
		if !DeepEqual(m, m) || DeepEqual(m, clone.Interface()) {
			t.Fatal("DeepEqual lost map semantics")
		}
		value.Clear()
		if m.Len() != 0 {
			t.Fatal("Clear did not clear model")
		}
		nilValue := ValueOf(gosimruntime.NilMap[string, int]())
		if !nilValue.IsNil() || nilValue.Len() != 0 || nilValue.MapRange().Next() {
			t.Fatal("nil map reflection")
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func TestNativeContainersRejected(t *testing.T) {
	t.Parallel()
	for _, value := range []any{map[int]int{}, make(chan int), gosimruntime.Chan[int]{}} {
		mustPanic(t, func() { ValueOf(value) })
		mustPanic(t, func() { TypeOf(value) })
	}
}

func TestOrdinaryValuesStayWrapped(t *testing.T) {
	t.Parallel()
	type record struct {
		Number int
		Label  string
	}
	original := record{3, "three"}
	value := ValueOf(&original).Elem()
	value.FieldByName("Number").SetInt(9)
	if got, ok := TypeAssert[record](value); !ok || got.Number != 9 {
		t.Fatal("typed assertion or wrapped mutation failed")
	}
	callback := MakeFunc(
		TypeOf(func(int) int { return 0 }),
		func(args []Value) []Value { return []Value{ValueOf(int(args[0].Int()) + 1)} },
	)
	if got := callback.Call([]Value{ValueOf(41)})[0].Int(); got != 42 {
		t.Fatalf("MakeFunc=%d", got)
	}
	values := []int{4, 5}
	sliced := SliceAt(TypeFor[int](), unsafe.Pointer(&values[0]), 2)
	sliced.Index(1).SetInt(7)
	if values[1] != 7 {
		t.Fatal("SliceAt did not preserve view")
	}
	if len(VisibleFields(TypeOf(original))) != 2 {
		t.Fatal("visible fields")
	}
}

func mustPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("expected explicit rejection")
		}
	}()
	f()
}

func TestDeepEqualBytes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		a, b  []byte
		equal bool
	}{
		{nil, nil, true},
		{nil, []byte{}, false},
		{[]byte{}, []byte{}, true},
		{[]byte{1, 2}, []byte{1, 2}, true},
		{[]byte{1, 2}, []byte{1, 3}, false},
		{[]byte{1}, []byte{1, 2}, false},
	} {
		if got := DeepEqual(tc.a, tc.b); got != tc.equal {
			t.Fatalf("DeepEqual(%v,%v)=%v", tc.a, tc.b, got)
		}
	}
}
