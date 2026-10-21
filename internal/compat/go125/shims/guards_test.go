//go:build go1.25 && !go1.26

package shims_test

import (
	"errors"
	"testing"

	"github.com/neria-cloud/gosim/internal/compat/go125/shims/debug"
	"github.com/neria-cloud/gosim/internal/compat/go125/shims/metrics"
	"github.com/neria-cloud/gosim/internal/compat/go125/shims/reflect"
	"github.com/neria-cloud/gosim/internal/compat/go125/shims/tls"
	"github.com/neria-cloud/gosim/internal/compat/shims/runtimebridge"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

func TestOptionalHostCollectorsAbort(t *testing.T) {
	if len(metrics.All()) != 0 {
		t.Fatal("host metrics advertised")
	}
	if runtimebridge.Runtime_getAuxv() != nil {
		t.Fatal("host auxiliary vector exposed")
	}
	for _, tc := range []struct {
		operation string
		call      func()
	}{
		{"runtime/metrics.Read", func() { metrics.Read([]metrics.Sample{{Name: "/gc/heap/live:bytes"}}) }},
		{"runtime.NumGoroutine", func() { runtimebridge.Runtime_NumGoroutine() }},
		{"runtime.ThreadCreateProfile", func() { runtimebridge.Runtime_ThreadCreateProfile(nil) }},
		{"runtime/debug.ReadGCStats", func() { debug.ReadGCStats(new(debug.GCStats)) }},
		{"crypto/tls", func() { tls.CipherSuites() }},
		{"crypto/tls", func() { tls.InsecureCipherSuites() }},
		{"reflect.MapOf", func() { reflect.MapOf(nil, nil) }},
	} {
		t.Run(tc.operation, func(t *testing.T) {
			e, err := simrt.NewEngine(simrt.EngineConfig{Seed: 1, TimeoutNS: 1e9, ReplayRevision: simrt.ReplayRevision})
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			after, recovered := false, false
			err = e.Run(func() {
				machine, err := e.PrepareMachine(1, 1, "guard")
				if err != nil {
					t.Fatal(err)
				}
				e.PublishMachine(machine, func() {
					defer func() { _ = recover(); recovered = true }()
					tc.call()
					after = true
				})
				simrt.NewChan[struct{}](0).Recv()
			})
			var aborted *simrt.AbortError
			if !errors.As(err, &aborted) || aborted.Diagnostic.Operation != tc.operation || after || recovered {
				t.Fatalf("err=%v after=%v recovered=%v", err, after, recovered)
			}
		})
	}
}
