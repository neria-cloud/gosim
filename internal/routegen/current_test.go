package routegen

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/profiles"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

// Native test jobs exercise the compiler Go selected for that job. They never
// install or execute another family internally.
func currentGeneration(t *testing.T) (buildspec.Toolchain, spec.Profile, GenerateOptions) {
	t.Helper()
	r, err := buildcfg.ResolveRunningProcess(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	v, err := buildspec.ParseGoVersion(r.Toolchain.Version)
	if err != nil {
		t.Fatal(err)
	}
	family, err := buildspec.LookupFamily(v)
	if err != nil {
		t.Fatal(err)
	}
	p, err := profiles.Descriptor(family.Profile, r.Toolchain.HostArch)
	if err != nil {
		t.Fatal(err)
	}
	origin, err := spec.LoadOrigin(os.DirFS("../compat/profiles"), family.Profile)
	if err != nil {
		t.Fatal(err)
	}
	return r.Toolchain, p, GenerateOptions{RefreshOrigin: r.Toolchain.Version != origin.BaselineToolchain}
}

func otherFamily(current string) string {
	for _, f := range buildspec.SupportedFamilies() {
		if f.Profile != current {
			return f.Profile
		}
	}
	panic("fixture requires another registered family")
}

// Compare complete executable bytes; exact provenance stays in ignored records.
func assertGeneratedMatchesBaseline(t *testing.T, tc buildspec.Toolchain, name string, actual []byte) {
	t.Helper()
	if strings.HasSuffix(name, ".origin.json") {
		var origin spec.Origin
		if err := json.Unmarshal(actual, &origin); err != nil {
			t.Fatal(err)
		}
		if err := spec.VerifyBaselineOrigin(tc, origin); err != nil {
			t.Fatal(err)
		}
		return
	}
	if strings.HasPrefix(name, ".gosim/generation/") {
		var record GenerationRecord
		if err := json.Unmarshal(actual, &record); err != nil {
			t.Fatal(err)
		}
		if record.GeneratedWith != tc.Version || record.Stage != "typed-adapters" || !buildspec.ValidDigest(record.AdapterDigest) || !buildspec.ValidDigest(record.OriginDigest) {
			t.Fatal("invalid generation record", record)
		}
		return
	}
	want, err := os.ReadFile(filepath.Join("../..", name))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(actual, want) {
		t.Fatalf("generated declarations or route semantics changed in %s", name)
	}
}
