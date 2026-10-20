//go:build go1.26 && !go1.27 && (amd64 || arm64)

package kernel_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/control"
	fp "github.com/neria-cloud/gosim/internal/fs"
	"github.com/neria-cloud/gosim/internal/kernel"
	np "github.com/neria-cloud/gosim/internal/net"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func runABI(t *testing.T, features contracts.FeatureSet, main func()) error {
	t.Helper()
	e, err := simrt.NewEngine(simrt.EngineConfig{Seed: 17, TimeoutNS: 1e9, ReplayRevision: simrt.ReplayRevision})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	return e.Run(func() {
		var providers []contracts.Provider
		if features.Has(contracts.FS) {
			providers = append(providers, fp.NewProvider())
		}
		if features.Has(contracts.Net) {
			providers = append(providers, np.NewProvider())
		}
		k, err := kernel.New(
			kernel.Config{
				BuildDigest: strings.Repeat("b", 64),
				Features:    features,
				TimeoutNS:   1e9,
				Identity: abi.HostIdentity{
					Toolchain: "go1.26.9",
					Profile:   "go126",
					HostOS:    runtime.GOOS,
					HostArch:  runtime.GOARCH,
				},
			},
			e,
			providers,
		)
		if err != nil {
			t.Fatal(err)
		}
		control.Bind(k)
		defer func() {
			if err := k.Close(); err != nil {
				t.Error(err)
			}
			control.Bind(nil)
		}()
		if err := k.Run(main); err != nil {
			t.Error(err)
		}
	})
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
