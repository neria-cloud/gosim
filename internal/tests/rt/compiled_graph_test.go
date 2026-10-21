//go:build sim

package behavior_test

import (
	"reflect"
	"runtime"
	"testing"
)

// Building this ordinary fixture checks the real translated graph, including
// shared reflect/testing hooks and the native runtime's host/profile closure.
func TestCompiledCompatibilityGraph(t *testing.T) {
	values := map[string]int{"answer": 42}
	typ := reflect.TypeOf(values)
	if typ.Kind() != reflect.Map || typ.Elem().Kind() != reflect.Int {
		t.Fatal("shared reflect surface")
	}
	done := make(chan int, 1)
	go func() { runtime.Gosched(); done <- values["answer"] }()
	if got := <-done; got != 42 {
		t.Fatal(got)
	}
}
