//go:build go1.25 && !go1.26

package shims_test

import (
	"strings"
	"testing"
	"time"

	exec "github.com/neria-cloud/gosim/internal/compat/go125/shims/exec"
	pprof "github.com/neria-cloud/gosim/internal/compat/go125/shims/pprof"
	rb "github.com/neria-cloud/gosim/internal/compat/shims/runtimebridge"
	signal "github.com/neria-cloud/gosim/internal/compat/go125/shims/signal"
	tls "github.com/neria-cloud/gosim/internal/compat/go125/shims/tls"
	x509 "github.com/neria-cloud/gosim/internal/compat/go125/shims/x509"
	engine "github.com/neria-cloud/gosim/rt/runtime"
)

func TestOptionalOperationsAbort(t *testing.T) {
	t.Parallel()
	for name, operation := range map[string]func(){
		"tls":                  func() { tls.Client(nil, nil) },
		"certificate":          func() { _, _ = x509.ParseCertificate(nil) },
		"process":              func() { exec.Command("unused") },
		"signals":              func() { signal.Notify(nil) },
		"profiling":            func() { _ = pprof.StartCPUProfile(nil) },
		"enable-block-profile": func() { rb.Runtime_SetBlockProfileRate(1) },
	} {
		t.Run(name, func(t *testing.T) {
			// A process can own only one active simulation engine.
			e, err := engine.NewEngine(engine.EngineConfig{Seed: 1, TimeoutNS: int64(time.Second), ReplayRevision: engine.ReplayRevision})
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			completed := false
			err = e.Run(func() { func() { defer func() { _ = recover() }(); operation() }(); completed = true })
			if err == nil || !strings.Contains(err.Error(), "UnsupportedOperation") || completed {
				t.Fatalf("guard failed: completed=%v err=%v", completed, err)
			}
		})
	}
}
