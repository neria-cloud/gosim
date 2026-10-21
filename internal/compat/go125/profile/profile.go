// Package profile is inspectable metadata; it does not import executable adapters.
package profile

import (
	"embed"
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

//go:embed contracts_*.json hooks_*.json hooks.json policies.json origin.json shims.json bridges.json probes.json modules.json capabilities.json native_imports.json routes_*.json shared_globals.json
var metadata embed.FS

// Descriptor returns independent records so callers cannot mutate later selections.
func Descriptor() spec.Profile {
	p, err := DescriptorForArch(runtime.GOARCH)
	if err != nil {
		panic(err)
	}
	return p
}

// DescriptorForArch selects the Linux source ABI independently of the host OS.
// The Hook schema remains identical; an absent architecture is never inferred.
func DescriptorForArch(arch string) (spec.Profile, error) {
	if arch != "amd64" && arch != "arm64" {
		return spec.Profile{}, fmt.Errorf("unsupported simulation architecture %q", arch)
	}
	p := spec.Profile{
		ID: "go125", Revision: Revision,
		Support: support(),
		RuntimePackages: []string{
			"github.com/neria-cloud/gosim/rt/runtime", "github.com/neria-cloud/gosim/rt/runtime/abi", "github.com/neria-cloud/gosim/internal/compat/hooks/rt",
			"github.com/neria-cloud/gosim/internal/compat/hooks/kernel",
			"github.com/neria-cloud/gosim/internal/compat/hooks/fs",
			"github.com/neria-cloud/gosim/internal/compat/hooks/net",
			"github.com/neria-cloud/gosim/internal/compat/shims/reflect",
			"github.com/neria-cloud/gosim/internal/compat/shims/testing",
			"github.com/neria-cloud/gosim/internal/compat/go125/shims/synctest",
			"github.com/neria-cloud/gosim/internal/compat/go125/shims/maps",
			"github.com/neria-cloud/gosim/internal/compat/go125/shims/cryptorand",
			"github.com/neria-cloud/gosim/internal/compat/shims/runtimebridge",
		},
	}
	decode("hooks.json", &p.Hooks)
	var archHooks []spec.Hook
	decode("hooks_"+arch+".json", &archHooks)
	p.Hooks = append(p.Hooks, archHooks...)
	decode("policies.json", &p.Packages)
	p.ContractsByArch = make(map[string]spec.ContractManifest)
	for _, contractArch := range []string{"amd64", "arm64"} {
		var manifest spec.ContractManifest
		decode("contracts_"+contractArch+".json", &manifest)
		p.ContractsByArch[contractArch] = manifest
	}
	return p, nil
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
	p, err := DescriptorForArch(tc.HostArch)
	if err != nil {
		return err
	}
	if err := p.Validate(); err != nil {
		return err
	}
	if err := PolicyManifest().Validate(p); err != nil {
		return err
	}
	if err := spec.VerifyBaselineOrigin(tc, VerificationOrigin()); err != nil {
		return err
	}
	if err := spec.VerifyHooks(tc, p.Hooks); err != nil {
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

// PolicyManifest contains application boundaries, not host compiler/runner
// permissions. Selected adapters implement the classified initialization probes.
func PolicyManifest() spec.PolicyManifest {
	var manifest spec.PolicyManifest
	decode("shared_globals.json", &manifest.SharedGlobals)
	decode("bridges.json", &manifest.Bridges)
	decode("probes.json", &manifest.Probes)
	decode("modules.json", &manifest.Modules)
	decode("capabilities.json", &manifest.Capabilities)
	decode("native_imports.json", &manifest.NativeImports)
	return manifest
}

func VerifyModules(tc buildspec.Toolchain, directory string) error {
	return spec.VerifyModules(tc, directory, PolicyManifest().Modules)
}

func support() buildspec.FamilySupport {
	support, err := buildspec.LookupProfile("go125")
	if err != nil {
		panic(err)
	}
	return support
}
