// Package profile is inspectable metadata; it does not import executable adapters.
package profile

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

//go:embed hooks.json tuples.json policies.json origin.json bridges.json native_imports.json capabilities.json shims.json
var metadata embed.FS

// Descriptor returns independent records so callers cannot mutate later selections.
// TODO(migration): Complete this profile before selecting it in Task 21.
func Descriptor() spec.Profile {
	p := spec.Profile{
		ID: "go126", Revision: Revision,
		RuntimePackages: []string{
			"github.com/neria-cloud/gosim/rt/runtime", "github.com/neria-cloud/gosim/rt/runtime/abi", "github.com/neria-cloud/gosim/internal/compat/go126/hooks/rt",
			"github.com/neria-cloud/gosim/internal/compat/go126/shims/reflect",
			"github.com/neria-cloud/gosim/internal/compat/go126/shims/testing",
			"github.com/neria-cloud/gosim/internal/compat/go126/shims/maps",
			"github.com/neria-cloud/gosim/internal/compat/go126/shims/synctest",
			"github.com/neria-cloud/gosim/internal/compat/go126/shims/cryptorand",
		},
	}
	decode("hooks.json", &p.Hooks)
	decode("tuples.json", &p.Tuples)
	decode("policies.json", &p.Packages)
	return p
}

func VerificationOrigin() spec.Origin {
	var origin spec.Origin
	decode("origin.json", &origin)
	return origin
}

func Verify(tc buildspec.Toolchain) error {
	if err := Descriptor().Validate(); err != nil {
		return err
	}
	if err := PolicyManifest().Validate(Descriptor()); err != nil {
		return err
	}
	if err := spec.VerifyOrigin(tc, VerificationOrigin()); err != nil {
		return err
	}
	if err := spec.VerifyHooks(tc, Descriptor().Hooks); err != nil {
		return err
	}
	return spec.VerifyHooks(tc, ShimManifest())
}

func decode(name string, result any) {
	data, err := metadata.ReadFile(name)
	if err == nil {
		err = json.Unmarshal(data, result)
	}
	if err != nil {
		panic(fmt.Sprintf("invalid embedded go126 profile %s: %v", name, err))
	}
}

// PolicyManifest currently owns runtime boundaries; Task 17 adds the selected
// shim and initialization audit before Task 21 may activate this profile.
func PolicyManifest() spec.PolicyManifest {
	var m spec.PolicyManifest
	decode("bridges.json", &m.Bridges)
	decode("native_imports.json", &m.NativeImports)
	decode("capabilities.json", &m.Capabilities)
	return m
}

func ShimManifest() []spec.Hook { var shims []spec.Hook; decode("shims.json", &shims); return shims }
