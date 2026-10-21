//go:build sim

package optionalreflect

import (
	"testing"

	"github.com/modern-go/reflect2"
)

// This test is an expected-abort fixture and must be run separately.
func TestUnsafeReflectGuard(t *testing.T) {
	func() {
		defer func() { _ = recover() }()
		reflect2.PtrOf(new(int))
	}()
	t.Fatal("application recovered from a raw-pointer reflection operation")
}
