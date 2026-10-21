package spec_test

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func profileDefinitions(t *testing.T) map[string]spec.FamilyDefinition {
	t.Helper()
	defs, err := spec.LoadDefinitions(os.DirFS("../profiles"))
	if err != nil {
		t.Fatal(err)
	}
	return defs
}

// These digests were compared once against git revision 550d5e3 after shared
// target/reason normalization and the nine explicitly added shim contracts.
// Tests only need committed definitions; they never read git or migration files.
func TestProfileDefinitionSemanticBaseline(t *testing.T) {
	t.Parallel()
	defs := profileDefinitions(t)
	expected := map[string]string{
		"1.25/amd64": "fa9beadde9ccdb776dd9838bda38e85466ad44480ceb8abc43a5f20e4a697881",
		"1.25/arm64": "5263ec93e16158ad16365f34556f7e7b1138c94d7ae2a8dab39a5a11e282bcee",
		"1.26/amd64": "21d966959587bb49c214cfb161d6c56f9f0d9cd6ce14edd432141bf3cc13b92d",
		"1.26/arm64": "e223ff97d2a663801013fafdbd0f7ff68f7332fe51b7c53ae7f9b1d172b7e148",
	}
	for _, family := range []string{"1.25", "1.26"} {
		r, err := spec.ResolveDefinitions(defs, family)
		if err != nil {
			t.Fatal(err)
		}
		for _, arch := range []string{"amd64", "arm64"} {
			p, err := r.Descriptor(arch, buildspec.Hash([]byte("fixture revision")))
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Validate(); err != nil {
				t.Fatal(err)
			}
			if err := r.PolicyManifest().Validate(p); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(semanticView(p, r.PolicyManifest(), r.ShimManifest(arch)))
			if err != nil {
				t.Fatal(err)
			}
			if got := buildspec.Hash(data); got != expected[family+"/"+arch] {
				t.Fatalf("%s/%s semantic metadata changed: %s", family, arch, got)
			}
			if err := spec.VerifyShimBindings("../../..", r, spec.SourceContext{OS: "linux", Arch: arch}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestUnchangedFamilyInheritsRealIntermediateDelta(t *testing.T) {
	defs := profileDefinitions(t)
	defs["1.27"] = spec.FamilyDefinition{Schema: 1, Family: "1.27", Parent: "1.26", Supported: false}
	before, err := spec.ResolveDefinitions(defs, "1.26")
	if err != nil {
		t.Fatal(err)
	}
	after, err := spec.ResolveDefinitions(defs, "1.27")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.Chain, []string{"1.25", "1.26", "1.27"}) || after.DefinitionDigest == before.DefinitionDigest {
		t.Fatal("lost ancestry identity")
	}
	if _, err := buildspec.LookupFamily(buildspec.GoVersion{Major: 1, Minor: 27}); err == nil {
		t.Fatal("fixture enabled production support")
	}
	after.Family, after.ProfileID, after.Supported = before.Family, before.ProfileID, before.Supported
	after.Chain, after.DefinitionDigest = before.Chain, before.DefinitionDigest
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unchanged child lost intermediate metadata")
	}
}

func TestCurrentCompilerResolvedContracts(t *testing.T) {
	t.Parallel()
	tc := currentToolchain()
	version, err := buildspec.ParseGoVersion(tc.Version)
	if err != nil {
		t.Fatal(err)
	}
	r, err := spec.ResolveDefinitions(profileDefinitions(t), version.Family())
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.Descriptor(tc.HostArch, buildspec.Hash([]byte("fixture revision")))
	if err != nil {
		t.Fatal(err)
	}
	// Other architectures and race contexts are source inspection, not execution.
	for _, arch := range []string{"amd64", "arm64"} {
		for _, race := range []bool{false, true} {
			if err := spec.VerifyFamilyContracts(t.Context(), buildspec.Resolution{Toolchain: tc}, p.ContractsByArch[arch], buildspec.BuildOptions{Race: race}); err != nil {
				t.Fatalf("%s race=%t: %v", arch, race, err)
			}
		}
	}
}

func semanticRecords[T any](rows []T) []string {
	out := []string{}
	for _, r := range rows {
		data, err := json.Marshal(r)
		if err != nil {
			panic(err)
		}
		out = append(out, string(data))
	}
	slices.Sort(out)
	return out
}

func semanticView(p spec.Profile, m spec.PolicyManifest, shims []spec.Hook) map[string]any {
	out := map[string]any{"id": p.ID, "hooks": semanticRecords(p.Hooks), "packages": semanticRecords(p.Packages), "runtime_packages": semanticRecords(p.RuntimePackages), "shims": semanticRecords(shims), "bridges": semanticRecords(m.Bridges), "native_imports": semanticRecords(m.NativeImports), "shared_globals": semanticRecords(m.SharedGlobals), "probes": semanticRecords(m.Probes), "modules": semanticRecords(m.Modules), "capabilities": semanticRecords(m.Capabilities)}
	for arch, c := range p.ContractsByArch {
		out[arch] = map[string]any{"schema": c.Schema, "family": c.Family, "architecture": c.Architecture, "hooks": semanticRecords(c.Hooks), "layouts": semanticRecords(c.Layouts), "constants": semanticRecords(c.Constants)}
	}
	return out
}
