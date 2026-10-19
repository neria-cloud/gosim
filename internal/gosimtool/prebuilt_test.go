package gosimtool

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestLookupArtifactUsesExactCurrentInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	id := artifactIdentity()
	sources := map[string]string{filepath.Join(root, "app.go"): buildspec.Hash([]byte("source"))}
	const pkg = "translated/example/app"
	input, err := PackageSourceDigest(pkg, sources)
	if err != nil {
		t.Fatal(err)
	}
	inputs := []PackageDigest{input}
	if _, err := LookupArtifact(root, id, pkg, sources); err == nil {
		t.Fatal("missing prebuilt accepted")
	}
	built, err := BuildArtifact(root, id, inputs, func(dir string) (Manifest, error) {
		return fixtureBuild(dir, id, inputs)
	})
	if err != nil {
		t.Fatal(err)
	}
	found, err := LookupArtifact(root, id, pkg, maps.Clone(sources))
	if err != nil || found.Directory != built.Directory {
		t.Fatal("matching prebuilt not found", found.Directory, err)
	}
	for name, mutate := range map[string]func(*buildspec.Identity, *string, map[string]string){
		"package": func(_ *buildspec.Identity, pkg *string, _ map[string]string) { *pkg += "/other" },
		"changed-source": func(_ *buildspec.Identity, _ *string, sources map[string]string) {
			sources[filepath.Join(root, "app.go")] = buildspec.Hash([]byte("changed"))
		},
		"added-source": func(_ *buildspec.Identity, _ *string, sources map[string]string) {
			sources[filepath.Join(root, "new.go")] = buildspec.Hash([]byte("new"))
		},
		"race": func(id *buildspec.Identity, _ *string, _ map[string]string) { id.Race = true },
		"features": func(id *buildspec.Identity, _ *string, _ map[string]string) {
			id.Features = []string{"rt", "fs"}
			id.ProviderRevisions = []buildspec.Revision{{Name: "fs", Digest: strings.Repeat("b", 64)}}
		},
		"certified": func(id *buildspec.Identity, _ *string, _ map[string]string) {
			id.Certification, id.EvidenceDigest = "certified", strings.Repeat("c", 64)
		},
	} {
		t.Run(name, func(t *testing.T) {
			otherID, otherPkg, otherSources := id, pkg, maps.Clone(sources)
			mutate(&otherID, &otherPkg, otherSources)
			if _, err := buildspec.NormalizeIdentity(otherID); err != nil {
				t.Fatal("invalid fixture identity", err)
			}
			if _, err := LookupArtifact(root, otherID, otherPkg, otherSources); err == nil {
				t.Fatal("lookup fell back to a different artifact")
			}
		})
	}
	if err := os.WriteFile(filepath.Join(built.Directory, built.Manifest.Binary.Path), []byte("corrupt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LookupArtifact(root, id, pkg, sources); err == nil {
		t.Fatal("corrupt binary accepted")
	}
}

func TestLookupArtifactRejectsLinkedNamespace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	id := artifactIdentity()
	sources := map[string]string{filepath.Join(root, "app.go"): buildspec.Hash([]byte("source"))}
	input, err := PackageSourceDigest("translated/app", sources)
	if err != nil {
		t.Fatal(err)
	}
	inputs := []PackageDigest{input}
	built, err := BuildArtifact(root, id, inputs, func(dir string) (Manifest, error) { return fixtureBuild(dir, id, inputs) })
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(built.Directory)
	moved := filepath.Join(t.TempDir(), "packages")
	if err := os.Rename(parent, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, parent); err != nil {
		t.Fatal(err)
	}
	if _, err := LookupArtifact(root, id, input.Path, sources); err == nil {
		t.Fatal("lookup followed a linked namespace")
	}
}

func TestPackageSourceDigestIsCanonicalAndValidated(t *testing.T) {
	t.Parallel()
	a, b := filepath.Join(t.TempDir(), "a.go"), filepath.Join(t.TempDir(), "b.go")
	digest := buildspec.Hash([]byte("source"))
	first, err := PackageSourceDigest("pkg", map[string]string{a: digest, b: digest})
	if err != nil {
		t.Fatal(err)
	}
	second, err := PackageSourceDigest("pkg", map[string]string{b: digest, a: digest})
	if err != nil || first != second {
		t.Fatal("noncanonical source ordering", err)
	}
	for _, sources := range []map[string]string{nil, {}, {"relative.go": digest}, {a: "invalid"}, {a + "/../a.go": digest}} {
		if _, err := PackageSourceDigest("pkg", sources); err == nil {
			t.Fatal("invalid source record accepted", sources)
		}
	}
	if _, err := PackageSourceDigest("", map[string]string{a: digest}); err == nil {
		t.Fatal("empty package accepted")
	}
}
