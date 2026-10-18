package buildcfg

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func fixtureIdentity() buildspec.Identity {
	d := strings.Repeat("a", 64)
	return buildspec.Identity{
		Schema: 1, Toolchain: "go1.26.9", Profile: "go126", DistributionDigest: d, StdlibDigest: d,
		AdapterRevision: d, RuntimeRevision: d, ProviderRevisions: []buildspec.Revision{{Name: "fs", Digest: d}, {Name: "net", Digest: d}},
		HostOS: "darwin", HostArch: "arm64", SimulationOS: "linux", SimulationArch: "arm64", Features: []string{"rt", "fs", "net"},
		Tags: []string{"linkname", "sim"}, ModuleGraphDigest: d, TranslatorDigest: d, DebugPolicyDigest: d, Certification: "candidate",
	}
}

func TestCanonicalIdentityOwnsOrderedInputs(t *testing.T) {
	id := fixtureIdentity()
	want, err := Digest(id)
	if err != nil {
		t.Fatal(err)
	}
	id.Features = []string{"net", "fs", "rt"}
	id.Tags = []string{"sim", "linkname"}
	id.ProviderRevisions[0], id.ProviderRevisions[1] = id.ProviderRevisions[1], id.ProviderRevisions[0]
	beforeFeatures := append([]string(nil), id.Features...)
	got, err := Digest(id)
	if err != nil || got != want {
		t.Fatalf("order affected identity: %s %s %v", got, want, err)
	}
	if !reflect.DeepEqual(id.Features, beforeFeatures) || id.ProviderRevisions[0].Name != "net" || id.Tags[0] != "sim" {
		t.Fatal("canonicalization mutated caller")
	}
}

func TestEveryIdentityDimensionChangesDigest(t *testing.T) {
	base := fixtureIdentity()
	original, err := Digest(base)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < reflect.TypeOf(base).NumField(); i++ {
		field := reflect.TypeOf(base).Field(i)
		t.Run(field.Name, func(t *testing.T) {
			id := fixtureIdentity()
			value := reflect.ValueOf(&id).Elem().Field(i)
			switch value.Kind() {
			case reflect.String:
				value.SetString(strings.Repeat("b", 64))
			case reflect.Int:
				value.SetInt(2)
			case reflect.Bool:
				value.SetBool(true)
			case reflect.Slice:
				value.Set(reflect.MakeSlice(value.Type(), 0, 0))
			default:
				t.Fatalf("uncovered identity field %s", field.Name)
			}
			if digest, err := Digest(id); err == nil && digest == original {
				t.Fatal("changed field reused the build digest")
			}
		})
	}
	// Promotion changes the artifact identity, but not the source being certified.
	baseSource, err := buildspec.SourceDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	base.Certification, base.EvidenceDigest = "certified", strings.Repeat("b", 64)
	promoted, err := Digest(base)
	if err != nil || promoted == original {
		t.Fatalf("promotion reused candidate artifact: %v", err)
	}
	promotedSource, err := buildspec.SourceDigest(base)
	if err != nil || promotedSource != baseSource {
		t.Fatalf("circular source evidence: %v", err)
	}
}

func TestIdentityRejectsInvalidMetadata(t *testing.T) {
	for _, mutate := range []func(*buildspec.Identity){
		func(id *buildspec.Identity) { id.Features = []string{"rt", "rt"} },
		func(id *buildspec.Identity) { id.ProviderRevisions = nil },
		func(id *buildspec.Identity) { id.Tags = []string{"go1.27"} },
		func(id *buildspec.Identity) { id.Tags = []string{"sim", "sim"} },
		func(id *buildspec.Identity) { id.Certification = "certified" },
		func(id *buildspec.Identity) { id.EvidenceDigest = strings.Repeat("a", 64) },
		func(id *buildspec.Identity) { id.Profile = "go125" },
		func(id *buildspec.Identity) { id.SimulationArch = "amd64" },
	} {
		id := fixtureIdentity()
		mutate(&id)
		if _, err := Digest(id); err == nil {
			t.Fatalf("accepted invalid identity: %+v", id)
		}
	}
}

func TestSourceIdentityExcludesOnlyEvidenceAndDocs(t *testing.T) {
	root := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		filename := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{"rt/runtime", "internal/contracts", "internal/kernel", "internal/control", "internal/fs", "internal/net"} {
		write(dir+"/source.go", "package fixture\n")
	}
	write("go.mod", "module github.com/neria-cloud/gosim\n")
	write("internal/compat/go126/profile/tuples.json", `[]`)
	write("docs/architecture.md", "old docs")
	write(".gosim/output", "old artifact")
	write(".git", "gitdir: original/worktree")
	base := fixtureIdentity()
	build := func() buildspec.Identity {
		t.Helper()
		id, err := identityForSource(
			buildspec.Config{Schema: 1, Toolchain: base.Toolchain, Features: base.Features, SimulationOS: "linux"},
			buildspec.Toolchain{Version: base.Toolchain, HostOS: base.HostOS, HostArch: base.HostArch},
			spec.Profile{ID: base.Profile, Revision: base.AdapterRevision}, buildspec.BuildOptions{},
			Distribution{ArchiveDigest: base.DistributionDigest, SourceDigest: base.StdlibDigest}, root, base.ModuleGraphDigest)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	before := build()
	write("internal/compat/go126/profile/tuples.json", `[{"status":"certified"}]`)
	write("docs/architecture.md", "new docs")
	write(".gosim/output", "new artifact")
	write(".git", "gitdir: another/worktree")
	if after := build(); !reflect.DeepEqual(before, after) {
		t.Fatal("promotion, worktree location, docs or output changed executable fingerprint")
	}
	write("internal/fs/source.go", "package changed\n")
	if after := build(); after.ProviderRevisions[0].Digest == before.ProviderRevisions[0].Digest || after.TranslatorDigest == before.TranslatorDigest {
		t.Fatal("provider/source change failed to invalidate evidence")
	}
	write("internal/compat/go126/profile/hooks.json", `[{"symbol":"changed"}]`)
	if after := build(); after.TranslatorDigest == before.TranslatorDigest {
		t.Fatal("ABI policy not fingerprinted")
	}
}
