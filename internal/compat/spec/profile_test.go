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
		Schema: 1, Toolchain: "go1.26.9", Profile: "go126", DistributionDigest: d, StdlibDigest: d,
		AdapterRevision: d, RuntimeRevision: d, HostOS: "linux", HostArch: "arm64", SimulationOS: "linux", SimulationArch: "arm64",
		Features: []string{"rt"}, Tags: []string{"linkname", "sim"}, ModuleGraphDigest: d, TranslatorDigest: d, DebugPolicyDigest: d, Certification: "candidate",
	}
	p := Profile{
		ID: "go126", Revision: d,
		Hooks:           []Hook{{Package: "runtime", Symbol: "Gosched", Signature: "func()", Target: module + "/internal/compat/go126/hooks/rt.Gosched", Owner: "rt"}},
		Packages:        []PackagePolicy{{Path: "runtime", Mode: "bridge", Reason: "explicit runtime hooks"}},
		RuntimePackages: []string{module + "/internal/kernel"},
	}
	for _, features := range [][]string{{"rt"}, {"rt", "fs"}, {"rt", "net"}, {"rt", "fs", "net"}} {
		p.Tuples = append(p.Tuples, buildspec.Tuple{
			Toolchain: id.Toolchain, HostOS: id.HostOS, HostArch: id.HostArch, SimulationOS: id.SimulationOS, SimulationArch: id.SimulationArch,
			Features: features, Status: "pending",
		})
	}
	return p, id
}

func TestSelectFeatureProvidersAndOwnInputs(t *testing.T) {
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

func TestCertificationRequiresExactSourceEvidence(t *testing.T) {
	p, id := fixture()
	id.Certification, id.EvidenceDigest = "certified", strings.Repeat("b", 64)
	if _, err := p.Select(id); err == nil {
		t.Fatal("certified selection accepted pending tuple")
	}
	source, err := buildspec.SourceDigest(id)
	if err != nil {
		t.Fatal(err)
	}
	p.Tuples[0].Status, p.Tuples[0].SourceDigest, p.Tuples[0].ResultDigest = "certified", source, id.EvidenceDigest
	if _, err := p.Select(id); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*buildspec.Identity){
		func(id *buildspec.Identity) { id.RuntimeRevision = strings.Repeat("c", 64) },
		func(id *buildspec.Identity) { id.EvidenceDigest = strings.Repeat("c", 64) },
		func(id *buildspec.Identity) { id.Race = true },
		func(id *buildspec.Identity) { id.Toolchain = "go1.26.8" },
		func(id *buildspec.Identity) { id.AdapterRevision = strings.Repeat("c", 64) },
		func(id *buildspec.Identity) { id.DebugPolicyDigest = strings.Repeat("c", 64) },
	} {
		copy := id
		change(&copy)
		if _, err := p.Select(copy); err == nil {
			t.Fatalf("accepted stale/mismatched certification: %+v", copy)
		}
	}
	id.Certification, id.EvidenceDigest = "candidate", ""
	if _, err := p.Select(id); err == nil {
		t.Fatal("candidate path accepted tuple not pending")
	}
}

func TestProfileRejectsIncompleteOrAmbiguousMetadata(t *testing.T) {
	for _, change := range []func(*Profile){
		func(p *Profile) { p.Hooks = nil },
		func(p *Profile) { p.Tuples = append(p.Tuples, p.Tuples[0]) },
		func(p *Profile) { p.Tuples[0].Toolchain = "go1.25.14" },
		func(p *Profile) { p.Tuples[0].Status = "certified" },
		func(p *Profile) { p.Tuples[0].ResultDigest = strings.Repeat("a", 64) },
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
