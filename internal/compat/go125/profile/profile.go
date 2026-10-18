// Package profile is inspectable metadata; it does not import executable adapters.
package profile

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

//go:embed hooks.json tuples.json policies.json origin.json shims.json
var metadata embed.FS

// Descriptor returns independent records so callers cannot mutate later selections.
// TODO(migration): Complete this profile before selecting it in Task 21.
func Descriptor() spec.Profile {
	p := spec.Profile{
		ID: "go125", Revision: Revision,
		RuntimePackages: []string{"github.com/neria-cloud/gosim/rt/runtime", "github.com/neria-cloud/gosim/rt/runtime/abi", "github.com/neria-cloud/gosim/internal/hostabi"},
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

// ShimManifest is separate from callable hooks: the translator substitutes
// these packages as complete type universes, rather than individual functions.
func ShimManifest() []spec.Hook {
	var shims []spec.Hook
	decode("shims.json", &shims)
	return shims
}

func Verify(tc buildspec.Toolchain) error {
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
		panic(fmt.Sprintf("invalid embedded go125 profile %s: %v", name, err))
	}
}
