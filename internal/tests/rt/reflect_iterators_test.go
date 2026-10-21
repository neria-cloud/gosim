//go:build go1.26

package behavior_test

import (
	"reflect"
	"testing"
)

type reflectedRecord struct {
	Number int
	Items  map[string]int
}

func (r reflectedRecord) Count() int { return len(r.Items) }

type embeddedReflectType interface{ reflect.Type }

func TestReflectInheritedIterators(t *testing.T) {
	record := reflectedRecord{Number: 7, Items: map[string]int{"x": 9}}
	value := reflect.ValueOf(&record).Elem()
	// Exercise interface embedding, boxing and method dispatch after translation.
	var embedded embeddedReflectType = value.Type()
	var boxed any = embedded
	typ := boxed.(reflect.Type)
	names := ""
	for field := range typ.Fields() {
		names += field.Name
		if field.Name == "Items" && field.Type.Kind() != reflect.Map {
			t.Fatal("iterator leaked the simulated map's native representation")
		}
	}
	if names != "NumberItems" || typ != reflect.TypeOf(record) {
		t.Fatal("iterator changed type identity or fields", names)
	}
	seen := 0
	for _, field := range value.Fields() {
		field.SetInt(42)
		seen++
		break
	}
	if seen != 1 || record.Number != 42 {
		t.Fatal("value iterator did not preserve mutation/early stop")
	}
	for field, item := range value.Fields() {
		if field.Name == "Items" && item.MapIndex(reflect.ValueOf("x")).Int() != 9 {
			t.Fatal("field value escaped the simulated map")
		}
	}
	methods := 0
	for method := range typ.Methods() {
		if method.Name != "Count" || method.Type.NumIn() != 1 {
			t.Fatal("unbound method iterator lost receiver")
		}
		methods++
	}
	for method, bound := range value.Methods() {
		if method.Name != "Count" || bound.Call(nil)[0].Int() != 1 {
			t.Fatal("bound method iterator")
		}
		methods++
	}
	if methods != 2 {
		t.Fatal("missing methods", methods)
	}
	typ = reflect.TypeOf(func(int, map[string]int) (bool, error) { return false, nil })
	inputs, outputs := 0, 0
	for arg := range typ.Ins() {
		if arg != typ.In(inputs) {
			t.Fatal("input type identity")
		}
		inputs++
	}
	for arg := range typ.Outs() {
		if arg != typ.Out(outputs) {
			t.Fatal("output type identity")
		}
		outputs++
	}
	if inputs != 2 || outputs != 2 {
		t.Fatal("incomplete signature iterator")
	}
}
