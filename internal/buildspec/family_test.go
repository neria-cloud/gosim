package buildspec_test

import (
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/testutil"
)

func TestStableFamiliesAcceptPatchesWithoutRegistration(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"go1.25.0", "go1.25.1", "go1.25.5", "go1.25.999", "go1.26.0", "go1.26.1", "go1.26.9", "go1.26.999"} {
		if err := buildspec.ValidateVersion(raw); err != nil {
			t.Error(raw, err)
		}
	}
	for _, raw := range []string{"go1.24.9", testutil.UnregisteredGo().Version, "go1.25", "go1.25.01", "go1.26rc1", "devel go1.26", "go1.26.1-custom", "go1.26.999999999999999999999999999999"} {
		if err := buildspec.ValidateVersion(raw); err == nil {
			t.Error("accepted", raw)
		}
	}
	families := buildspec.SupportedFamilies()
	families[0].Family = "broken"
	if family, err := buildspec.LookupFamily(buildspec.GoVersion{Major: 1, Minor: 25, Patch: 1}); err != nil || family.Profile != "go125" {
		t.Fatal(family, err)
	}
}
