// Package profile is inspectable metadata; it does not import executable adapters.
package profile

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

//go:embed hooks.json tuples.json policies.json origin.json
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

func Verify(tc buildspec.Toolchain) error {
	if err := spec.VerifyOrigin(tc, VerificationOrigin()); err != nil {
		return err
	}
	return spec.VerifyHooks(tc, Descriptor().Hooks)
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
