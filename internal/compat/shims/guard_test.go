//go:build go1.25

package shims_test

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	debug "github.com/neria-cloud/gosim/internal/compat/shims/debug"
	ecdsa "github.com/neria-cloud/gosim/internal/compat/shims/ecdsa"
	elliptic "github.com/neria-cloud/gosim/internal/compat/shims/elliptic"
	exec "github.com/neria-cloud/gosim/internal/compat/shims/exec"
	pprof "github.com/neria-cloud/gosim/internal/compat/shims/pprof"
	reflect2 "github.com/neria-cloud/gosim/internal/compat/shims/reflect2"
	rsa "github.com/neria-cloud/gosim/internal/compat/shims/rsa"
	rb "github.com/neria-cloud/gosim/internal/compat/shims/runtimebridge"
	signal "github.com/neria-cloud/gosim/internal/compat/shims/signal"
	tls "github.com/neria-cloud/gosim/internal/compat/shims/tls"
	trace "github.com/neria-cloud/gosim/internal/compat/shims/trace"
	x509 "github.com/neria-cloud/gosim/internal/compat/shims/x509"
	engine "github.com/neria-cloud/gosim/rt/runtime"
)

func TestOptionalOperationsAbort(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, want string
		operation  func()
	}{
		{"ecdsa-keygen", "crypto/ecdsa.GenerateKey", func() { _, _ = ecdsa.GenerateKey(nil, nil) }},
		{"elliptic-curve", "crypto/elliptic.P521", func() { _ = elliptic.P521() }},
		{"rsa-verify", "crypto/rsa.VerifyPKCS1v15", func() { _ = rsa.VerifyPKCS1v15(&rsa.PublicKey{N: big.NewInt(3), E: 3}, 0, nil, nil) }},
		{"tls", "crypto/tls", func() { tls.Client(nil, nil) }},
		{"certificate", "crypto/x509", func() { _, _ = x509.ParseCertificate(nil) }},
		{"process", "os/exec", func() { exec.Command("unused") }},
		{"signals", "os/signal", func() { signal.Notify(nil) }},
		{"profiling", "runtime/pprof", func() { _ = pprof.StartCPUProfile(nil) }},
		{"enable-block-profile", "runtime.SetBlockProfileRate", func() { rb.Runtime_SetBlockProfileRate(1) }},
		{"reflect-pointer", "reflect2.PtrOf", func() { reflect2.PtrOf(new(int)) }},
		{"reflect-value", "reflect2.Type.UnsafeNew", func() { reflect2.TypeOf(0).UnsafeNew() }},
		{"reflect-map-iterator", "reflect2.MapType.UnsafeIterate", func() { reflect2.TypeOf(map[int]int{}).(reflect2.MapType).UnsafeIterate(nil) }},
		{"trace-task", "runtime/trace.NewTask", func() { _, _ = trace.NewTask(context.Background(), "guard") }},
		{"trace-start", "runtime/trace.Start", func() { _ = trace.Start(nil) }},
		{"host-metrics", "runtime.ReadMemStats", func() { rb.Runtime_ReadMemStats(nil) }},
		{"mutex-profile", "runtime.SetMutexProfileFraction", func() { _ = rb.Runtime_SetMutexProfileFraction(5) }},
		{"debug-stack", "runtime/debug.Stack", func() { _ = debug.Stack() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A process can own only one active simulation engine.
			e, err := engine.NewEngine(engine.EngineConfig{Seed: 1, TimeoutNS: int64(time.Second), ReplayRevision: engine.ReplayRevision})
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			completed := false
			err = e.Run(func() { func() { defer func() { _ = recover() }(); tc.operation() }(); completed = true })
			if err == nil || !strings.Contains(err.Error(), "UnsupportedOperation") || !strings.Contains(err.Error(), tc.want) || completed {
				t.Fatalf("guard failed: completed=%v err=%v", completed, err)
			}
		})
	}
}
