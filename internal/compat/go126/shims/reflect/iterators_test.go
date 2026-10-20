//go:build go1.26 && !go1.27

package reflect

import "testing"

type iteratorRecord struct {
	A int
	B string
}

func (r iteratorRecord) Example() int { return r.A }

func TestGo126Iterators(t *testing.T) {
	t.Parallel()
	value := ValueOf(&iteratorRecord{A: 42, B: "value"}).Elem()
	names := ""
	for field := range value.Type().Fields() {
		names += field.Name
	}
	if names != "AB" {
		t.Fatal(names)
	}
	seen := 0
	for field, item := range value.Fields() {
		seen++
		if field.Name == "A" {
			item.SetInt(99)
		}
		break
	}
	if seen != 1 || value.Field(0).Int() != 99 {
		t.Fatal("iterator mutation/early stop")
	}
	for method, fn := range value.Methods() {
		if method.Name != "Example" || fn.Call(nil)[0].Int() != 99 {
			t.Fatal("bound method iterator")
		}
		seen++
	}
	if seen != 2 {
		t.Fatal("missing method")
	}
	for method := range value.Type().Methods() {
		if method.Type.NumIn() != 1 {
			t.Fatal("unbound method lost receiver")
		}
	}
	typ := TypeOf(func(int, string) (bool, error) { return false, nil })
	inputs, outputs := 0, 0
	for arg := range typ.Ins() {
		if arg != typ.In(inputs) {
			t.Fatal("input mismatch")
		}
		inputs++
	}
	for arg := range typ.Outs() {
		if arg != typ.Out(outputs) {
			t.Fatal("output mismatch")
		}
		outputs++
	}
	if inputs != 2 || outputs != 2 {
		t.Fatal("signature iterator lengths")
	}
	// Eager kind validation must happen even if the iterator is never consumed.
	mustPanic(t, func() { TypeOf(1).Fields() })
	mustPanic(t, func() { TypeOf(1).Ins() })
	mustPanic(t, func() { TypeOf(1).Outs() })
	mustPanic(t, func() { ValueOf(1).Fields() })
}
