//go:build go1.25

package reflect

import (
	native "reflect"
	"testing"
)

func TestSelectedTypeMethodSurface(t *testing.T) {
	// Native reflection on the interface checks the flattened embedded method
	// set against the actual compiler's standard-library interface.
	got, want := native.TypeFor[Type](), native.TypeFor[native.Type]()
	names := func(typ native.Type) []string {
		var result []string
		for i := range typ.NumMethod() {
			method := typ.Method(i)
			if method.IsExported() {
				result = append(result, method.Name)
			}
		}
		return result
	}
	if !native.DeepEqual(names(got), names(want)) {
		t.Fatalf("shim methods %v; compiler methods %v", names(got), names(want))
	}
}
