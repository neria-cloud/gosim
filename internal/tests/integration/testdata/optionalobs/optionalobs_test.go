//go:build sim

package optionalobs

import (
	"context"
	"runtime"
	"runtime/trace"
	"testing"
)

// These are separately invoked expected-abort fixtures. A caller's recover
// must not convert active host observations into successful simulation work.
func TestActiveTraceGuard(t *testing.T) {
	func() {
		defer func() { _ = recover() }()
		trace.NewTask(context.Background(), "unsupported")
	}()
	t.Fatal("application recovered from active tracing")
}

func TestHostMetricsGuard(t *testing.T) {
	func() {
		defer func() { _ = recover() }()
		runtime.ReadMemStats(new(runtime.MemStats))
	}()
	t.Fatal("application recovered from host metrics collection")
}

func TestHostMutexProfileGuard(t *testing.T) {
	func() {
		defer func() { _ = recover() }()
		runtime.SetMutexProfileFraction(5)
	}()
	t.Fatal("application recovered from host mutex profiling")
}
