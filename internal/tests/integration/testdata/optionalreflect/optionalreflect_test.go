//go:build sim

package optionalreflect

import (
	"reflect"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"

	"github.com/neria-cloud/gosim"
)

func TestReflect2Initialization(t *testing.T) {
	if jsoniter.ConfigDefault == nil || jsoniter.ConfigCompatibleWithStandardLibrary == nil {
		t.Fatal("jsoniter configuration did not initialize")
	}
	ptr := reflect2.TypeOfPtr((*string)(nil))
	if ptr.Kind() != reflect.Ptr || ptr.Elem().Kind() != reflect.String {
		t.Fatal("pointer element metadata is incorrect")
	}
	if ptr.Elem() != reflect2.TypeOf("") || ptr.Elem() != reflect2.Type2(reflect.TypeOf("")) {
		t.Fatal("type wrappers are not canonical")
	}
	if reflect2.TypeOf(nil) != nil || reflect2.Type2(nil) != nil || !reflect2.IsNil((*string)(nil)) {
		t.Fatal("nil type or value metadata is incorrect")
	}
	if _, ok := reflect2.TypeOf("").(reflect2.MapType); ok {
		t.Fatal("scalar wrapper incorrectly implements MapType")
	}
	if _, ok := reflect2.TypeOf("").(reflect2.PtrType); ok {
		t.Fatal("scalar wrapper incorrectly implements PtrType")
	}
	if (reflect2.Config{UseSafeImplementation: true}).Froze().TypeOf("") == reflect2.ConfigUnsafe.TypeOf("") {
		t.Fatal("independent configurations share mutable wrapper state")
	}
}

func TestReflect2PerMachineState(t *testing.T) {
	results := make(chan reflect2.Type, 2)
	for i := 0; i < 2; i++ {
		m, err := gosim.NewMachine(gosim.MachineConfig{Label: "reflect2", Main: func() {
			results <- reflect2.TypeOfPtr((*string)(nil)).Elem()
		}})
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Start(); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	if left, right := <-results, <-results; left == right {
		t.Fatal("machine-local reflect2 wrappers were shared")
	}
}

// This test is an expected-abort fixture and must be run separately.
func TestUnsafeReflectGuard(t *testing.T) {
	func() {
		defer func() { _ = recover() }()
		reflect2.PtrOf(new(int))
	}()
	t.Fatal("application recovered from a raw-pointer reflection operation")
}
