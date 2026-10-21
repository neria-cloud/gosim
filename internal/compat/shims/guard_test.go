//go:build go1.25

package shims_test

import (
	"math/big"
	"strings"
	"testing"
	"time"

	ecdsa "github.com/neria-cloud/gosim/internal/compat/shims/ecdsa"
	elliptic "github.com/neria-cloud/gosim/internal/compat/shims/elliptic"
	exec "github.com/neria-cloud/gosim/internal/compat/shims/exec"
	pprof "github.com/neria-cloud/gosim/internal/compat/shims/pprof"
	rsa "github.com/neria-cloud/gosim/internal/compat/shims/rsa"
	rb "github.com/neria-cloud/gosim/internal/compat/shims/runtimebridge"
	signal "github.com/neria-cloud/gosim/internal/compat/shims/signal"
	tls "github.com/neria-cloud/gosim/internal/compat/shims/tls"
	x509 "github.com/neria-cloud/gosim/internal/compat/shims/x509"
	engine "github.com/neria-cloud/gosim/rt/runtime"
)

func TestOptionalOperationsAbort(t *testing.T) {
	t.Parallel()
	for name, operation := range map[string]func(){
		"ecdsa-keygen":         func() { _, _ = ecdsa.GenerateKey(nil, nil) },
		"elliptic-curve":       func() { _ = elliptic.P521() },
		"rsa-verify":           func() { _ = rsa.VerifyPKCS1v15(&rsa.PublicKey{N: big.NewInt(3), E: 3}, 0, nil, nil) },
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
