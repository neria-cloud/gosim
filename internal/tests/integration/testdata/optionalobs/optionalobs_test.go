//go:build sim

package optionalobs

import (
	"context"
	"runtime"
	"runtime/trace"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestDisabledObservabilityInitialization(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("runtime tracing became active")
	}
	provider := sdktrace.NewTracerProvider()
	ctx, span := provider.Tracer("etcd-probe").Start(context.Background(), "disabled-runtime-trace")
	span.End()
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if prometheus.DefaultGatherer == nil || prometheus.NewGoCollector() == nil {
		t.Fatal("Prometheus collector registration did not initialize")
	}
}

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
