package spec_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/profiles"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestAdapterMetadataAndFingerprints(t *testing.T) {
	t.Parallel()
	for _, p := range []spec.Profile{fixtureProfile(t, "go125"), fixtureProfile(t, "go126")} {
		t.Run(p.ID, func(t *testing.T) {
			if err := p.Validate(); err != nil {
				t.Fatal(err)
			}
			support, err := buildspec.LookupProfile(p.ID)
			if err != nil || support.Family != p.Support.Family {
				t.Fatal("invalid family support", err)
			}
			digest, err := spec.SourceRevision("../../..", p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if digest != p.Revision {
				t.Fatal("stale adapter fingerprint; run go run ./cmd/gosim-gen profiles")
			}
		})
	}
	p := fixtureProfile(t, "go125")
	p.Hooks[0].Symbol = "mutation"
	p.Support.Family = "mutation"
	if fixtureProfile(t, "go125").Hooks[0].Symbol == "mutation" || fixtureProfile(t, "go125").Support.Family == "mutation" {
		t.Fatal("descriptor state leaked")
	}
}

func currentToolchain() buildspec.Toolchain {
	return buildspec.Toolchain{Executable: filepath.Join(runtime.GOROOT(), "bin/go"), GOROOT: runtime.GOROOT(), Version: runtime.Version(), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
}

func TestPinnedAdapterSourceVerification(t *testing.T) {
	t.Parallel()
	tc := currentToolchain()
	for _, candidate := range []struct {
		p      spec.Profile
		origin spec.Origin
	}{{fixtureProfile(t, "go125"), fixtureOrigin(t, "go125")}, {fixtureProfile(t, "go126"), fixtureOrigin(t, "go126")}} {
		t.Run(candidate.p.ID, func(t *testing.T) {
			origin := candidate.origin
			if origin.BaselineToolchain != tc.Version {
				if err := spec.VerifyBaselineOrigin(tc, origin); err == nil {
					t.Fatal("wrong patch/family accepted")
				}
				return
			}
			if err := spec.VerifyBaselineOrigin(tc, origin); err != nil {
				t.Fatal(err)
			}
			for _, arch := range []string{"amd64", "arm64"} {
				sourceTC := tc
				sourceTC.HostArch = arch
				p, err := profiles.Descriptor(candidate.p.ID, arch)
				if err != nil {
					t.Fatal(err)
				}
				if err := spec.VerifyHooks(sourceTC, p.Hooks); err != nil {
					t.Fatal(err)
				}
			}
			// A correct file hash cannot conceal a wrong private-symbol expectation.
			for i := range origin.Files {
				if len(origin.Files[i].Hooks) > 0 {
					origin.Files[i].Hooks[0].Symbol = "missingPrivateSymbol"
					break
				}
			}
			if err := spec.VerifyBaselineOrigin(tc, origin); err == nil || !strings.Contains(err.Error(), "declaration drift") {
				t.Fatalf("wrong private symbol: %v", err)
			}
			origin = candidate.origin
			// Copy only inspected files; no executable is taken from this altered root.
			clone := t.TempDir()
			version, err := os.ReadFile(filepath.Join(tc.GOROOT, "VERSION"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(clone, "VERSION"), version, 0o644); err != nil {
				t.Fatal(err)
			}
			for _, file := range origin.Files {
				data, err := os.ReadFile(filepath.Join(tc.GOROOT, file.Path))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(clone, file.Path)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(clone, origin.Files[0].Path)
			if err := os.WriteFile(path, []byte("package corrupted\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			altered := tc
			altered.GOROOT = clone
			if err := spec.VerifyBaselineOrigin(altered, origin); err == nil || !strings.Contains(err.Error(), "source modified") {
				t.Fatalf("modified source: %v", err)
			}
		})
	}
	// This declaration exists on both development and pinned families. VerifyHooks
	// must reject a changed signature even before strict patch selection is wired.
	hooks := fixtureProfile(t, "go126").Hooks
	hooks[0].Signature = "func(int)"
	if err := spec.VerifyHooks(tc, hooks); err == nil || !strings.Contains(err.Error(), "ABI drift") {
		t.Fatalf("signature drift: %v", err)
	}
}

func TestSourceRevisionIncludesCodePolicyAndPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(path, data string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"internal/compat/host/coro.go", "internal/compat/hooks/rt/source.go", "internal/hostabi/native.go", "internal/compat/selected/go125.go", "internal/compat/spec/profile.go", "internal/buildspec/modules.go"} {
		write(path, "original")
	}
	write("internal/compat/profiles/go125.json", `{"schema":1,"family":"1.25","supported":true,"delta":{}}`)
	digest := func() string {
		t.Helper()
		d, err := spec.SourceRevision(root, "go125")
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	original := digest()
	for _, path := range []string{"internal/compat/host/coro.go", "internal/compat/hooks/rt/source.go", "internal/hostabi/native.go", "internal/compat/selected/go125.go", "internal/compat/spec/profile.go", "internal/buildspec/modules.go"} {
		write(path, "modified")
		if digest() == original {
			t.Fatalf("revision ignores %s", path)
		}
		write(path, "original")
	}
	for _, path := range []string{"internal/compat/host/coro_test.go", "internal/compat/profiles/revisions_gen.go", "internal/compat/profiles/evidence.json"} {
		write(path, "not source for this identity")
	}
	if digest() != original {
		t.Fatal("tests or evidence changed source identity")
	}
	if err := os.Rename(filepath.Join(root, "internal/compat/host/coro.go"), filepath.Join(root, "internal/compat/host/moved.go")); err != nil {
		t.Fatal(err)
	}
	if digest() == original {
		t.Fatal("adapter move did not change identity")
	}
}

func TestResolvedFamilyShimBindings(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"1.25", "1.26"} {
		r, err := profiles.Resolve(family)
		if err != nil {
			t.Fatal(err)
		}
		context := spec.SourceContext{OS: "linux", Arch: runtime.GOARCH}
		if err := spec.VerifyShimBindings("../../..", r, context); err != nil {
			t.Fatal(err)
		}
		shims := r.ShimManifest(runtime.GOARCH)
		if len(shims) == 0 {
			t.Fatal("empty shim bindings")
		}
		if strings.HasPrefix(runtime.Version(), "go"+family+".") {
			if err := spec.VerifyHooks(currentToolchain(), shims); err != nil {
				t.Fatal(err)
			}
		}
		for i := range r.Declarations {
			if r.Declarations[i].Hook == shims[0] {
				r.Declarations[i].Hook.Signature = "func()"
			}
		}
		if err := spec.VerifyShimBindings("../../..", r, context); err == nil {
			t.Fatal("changed shim signature accepted")
		}
	}
}

func fixtureOrigin(t *testing.T, profileID string) spec.Origin {
	t.Helper()
	origin, err := spec.LoadOrigin(os.DirFS("../profiles"), profileID)
	if err != nil {
		t.Fatal(err)
	}
	return origin
}

func fixtureProfile(t *testing.T, id string) spec.Profile {
	t.Helper()
	p, err := profiles.Descriptor(id, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func fixturePolicy(t *testing.T, id string) spec.PolicyManifest {
	t.Helper()
	p, err := profiles.Policy(id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
