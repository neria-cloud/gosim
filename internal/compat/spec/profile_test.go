package spec

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func fixture() (Profile, buildspec.Identity) {
	d := strings.Repeat("a", 64)
	id := buildspec.Identity{
		Schema: 2, Toolchain: "go1.26.9", Profile: "go126", ToolchainDigest: d, StdlibDigest: d, ContractDigest: d,
		AdapterRevision: d, RuntimeRevision: d, HostOS: "linux", HostArch: "arm64", SimulationOS: "linux", SimulationArch: "arm64",
		Features: []string{"rt"}, Tags: []string{"linkname", "sim"}, ModuleGraphDigest: d, TranslatorDigest: d, DebugPolicyDigest: d,
	}
	p := Profile{
		ID: "go126", Revision: d, Support: buildspec.FamilySupport{Family: "1.26", Profile: "go126"},
		Hooks:           []Hook{{Package: "runtime", Symbol: "Gosched", Signature: "func()", Target: module + "/internal/compat/hooks/rt.Gosched", Owner: "rt"}},
		Packages:        []PackagePolicy{{Path: "runtime", Mode: "bridge", Reason: "explicit runtime hooks"}},
		RuntimePackages: []string{module + "/internal/kernel"},
	}

	p.ContractsByArch = map[string]ContractManifest{}
	for _, arch := range []string{"amd64", "arm64"} {
		p.ContractsByArch[arch] = ContractManifest{
			Schema: 1, Family: p.Support.Family, Architecture: arch, Hooks: slices.Clone(p.Hooks),
			Layouts:   []LayoutContract{{Package: "test", Type: "Record", Size: 8, Align: 8, Fields: []FieldContract{{Name: "Value", Type: "int64", Offset: 0}}}},
			Constants: []ConstantContract{{Package: "test", Symbol: "Number", Value: "1"}},
		}
	}
	id.ContractDigest, _ = p.ContractDigest()
	return p, id
}

func TestSelectFeatureProvidersAndOwnInputs(t *testing.T) {
	t.Parallel()
	for _, features := range [][]string{{"rt"}, {"rt", "fs"}, {"rt", "net"}, {"rt", "fs", "net"}} {
		p, id := fixture()
		id.Features = slices.Clone(features)
		var want []string
		for _, feature := range features[1:] {
			id.ProviderRevisions = append(id.ProviderRevisions, buildspec.Revision{Name: feature, Digest: id.RuntimeRevision})
			want = append(want, module+"/internal/"+feature)
		}
		selection, err := p.Select(id)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(selection.ProviderImports, want) || !buildspec.ValidDigest(selection.HookManifestDigest) {
			t.Fatalf("wrong selection %+v", selection)
		}
		selection.Identity.Features[0] = "mutated"
		selection.RuntimePackages[0] = "mutated"
		if id.Features[0] != "rt" || p.RuntimePackages[0] == "mutated" {
			t.Fatal("selection retains caller-owned slices")
		}
	}
}

func TestFamilySelectionNeedsNoPatchEvidence(t *testing.T) {
	t.Parallel()
	p, id := fixture()
	for _, patch := range []string{"go1.26.0", "go1.26.1", "go1.26.9", "go1.26.999"} {
		id.Toolchain = patch
		if _, err := p.Select(id); err != nil {
			t.Fatal(patch, err)
		}
	}
	original := id.ContractDigest
	id.ContractDigest = strings.Repeat("c", 64)
	if _, err := p.Select(id); err == nil {
		t.Fatal("accepted wrong contract digest")
	}
	id.ContractDigest = original
	id.Toolchain = "go1.25.1"
	if _, err := p.Select(id); err == nil {
		t.Fatal("accepted wrong family")
	}
	id.Toolchain = "go1.26.1"
	id.AdapterRevision = strings.Repeat("b", 64)
	if _, err := p.Select(id); err == nil {
		t.Fatal("accepted wrong adapter revision")
	}
}

func TestProfileRejectsIncompleteOrAmbiguousMetadata(t *testing.T) {
	t.Parallel()
	for _, change := range []func(*Profile){
		func(p *Profile) { p.Hooks = nil },
		func(p *Profile) { p.Support.Profile = "go125" },
		func(p *Profile) { p.Packages[0].Mode = "skip" },
		func(p *Profile) { p.Packages[0].Reason = "" },
		func(p *Profile) { p.Hooks = append(p.Hooks, p.Hooks[0]) },
		func(p *Profile) { p.Hooks[0].Owner = "other" },
		func(p *Profile) { p.Hooks[0].Target = module + "/internal/compat/go125/hooks/rt.Other" },
		func(p *Profile) { p.RuntimePackages = append(p.RuntimePackages, module+"/internal/fs/model") },
	} {
		p, _ := fixture()
		change(&p)
		if err := p.Validate(); err == nil {
			t.Fatalf("accepted invalid profile: %+v", p)
		}
	}
}

func TestVerifyHooksAgainstActiveToolchain(t *testing.T) {
	t.Parallel()
	tc := buildspec.Toolchain{
		Executable: filepath.Join(runtime.GOROOT(), "bin", "go"), GOROOT: runtime.GOROOT(), Version: runtime.Version(), HostOS: runtime.GOOS, HostArch: runtime.GOARCH,
	}
	hook := Hook{Package: "runtime", Symbol: "Gosched", Signature: "func()"}
	if err := VerifyHooks(tc, []Hook{hook}); err != nil {
		t.Fatal(err)
	}
	hook.Signature = "func(int)"
	if err := VerifyHooks(tc, []Hook{hook}); err == nil || !strings.Contains(err.Error(), "ABI drift") {
		t.Fatalf("signature mismatch not detected: %v", err)
	}
	hook.Symbol = "GosimMissingHookFixture"
	if err := VerifyHooks(tc, []Hook{hook}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing hook not detected: %v", err)
	}
}
