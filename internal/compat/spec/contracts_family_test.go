package spec_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/profiles"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestCurrentCompilerFamilyContracts(t *testing.T) {
	t.Parallel()
	tc := currentToolchain()
	v, err := buildspec.ParseGoVersion(tc.Version)
	if err != nil {
		t.Fatal(err)
	}
	p, err := profiles.Descriptor("go1"+strconv.Itoa(v.Minor), tc.HostArch)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := p.Contracts(tc.HostArch)
	if err != nil {
		t.Fatal(err)
	}
	// Both source selections are inspected in this native unit test. No race
	// executable or simulation is built or run.
	for _, race := range []bool{false, true} {
		if err := spec.VerifyFamilyContracts(context.Background(), buildspec.Resolution{Toolchain: tc}, manifest, buildspec.BuildOptions{Race: race}); err != nil {
			t.Fatalf("source race context %t: %v", race, err)
		}
	}
	manifest.Layouts[0].Fields[0].Type = "mutated"
	manifest.Hooks[0].Signature = "mutated"
	other, err := p.Contracts(tc.HostArch)
	if err != nil {
		t.Fatal(err)
	}
	if other.Layouts[0].Fields[0].Type == "mutated" || other.Hooks[0].Signature == "mutated" {
		t.Fatal("contracts retained caller-owned slices")
	}
	if _, err := p.Contracts("unsupported"); err == nil {
		t.Fatal("inferred missing architecture")
	}
}
