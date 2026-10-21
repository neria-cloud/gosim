package spec_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/neria-cloud/gosim/internal/testutil"

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

func TestUnchangedFamilyInheritsRealIntermediateDelta(t *testing.T) {
	defs := profileDefinitions(t)
	future := testutil.UnregisteredGo()
	parent := fmt.Sprintf("1.%d", future.Minor-1)
	defs[future.Family] = spec.FamilyDefinition{Schema: 1, Family: future.Family, Parent: parent, Supported: false}
	before, err := spec.ResolveDefinitions(defs, parent)
	if err != nil {
		t.Fatal(err)
	}
	after, err := spec.ResolveDefinitions(defs, future.Family)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.Chain, append(append([]string(nil), before.Chain...), future.Family)) || after.DefinitionDigest == before.DefinitionDigest {
		t.Fatal("lost ancestry identity")
	}
	if _, err := buildspec.LookupFamily(buildspec.GoVersion{Major: 1, Minor: future.Minor}); err == nil {
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

func TestPreviouslyOmittedShimContracts(t *testing.T) {
	for _, family := range []string{"1.25", "1.26"} {
		r, err := spec.ResolveDefinitions(profileDefinitions(t), family)
		if err != nil {
			t.Fatal(err)
		}
		p, err := r.Descriptor("arm64", buildspec.Hash([]byte("fixture")))
		if err != nil {
			t.Fatal(err)
		}
		for arch, manifest := range p.ContractsByArch {
			hooks := map[string]bool{}
			for _, h := range manifest.Hooks {
				hooks[h.Package+"."+h.Symbol] = true
			}
			for _, name := range []string{"crypto/tls.CipherSuites", "crypto/tls.InsecureCipherSuites", "runtime/metrics.All", "runtime/metrics.Read", "runtime/metrics.Value.Kind", "runtime/metrics.Value.Uint64", "runtime/metrics.Value.Float64", "runtime/metrics.Value.Float64Histogram", "runtime/debug.ReadGCStats"} {
				if !hooks[name] {
					t.Errorf("%s/%s lost %s", family, arch, name)
				}
			}
		}
	}
}
