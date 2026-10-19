package spec_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	p125 "github.com/neria-cloud/gosim/internal/compat/go125/profile"
	p126 "github.com/neria-cloud/gosim/internal/compat/go126/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestAdapterMetadataAndFingerprints(t *testing.T) {
	t.Parallel()
	for _, p := range []spec.Profile{p125.Descriptor(), p126.Descriptor()} {
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
				t.Fatal("stale adapter fingerprint; run go run ./internal/compat/genprofile")
			}
		})
	}
	p := p125.Descriptor()
	p.Hooks[0].Symbol = "mutation"
	p.Support.Family = "mutation"
	if p125.Descriptor().Hooks[0].Symbol == "mutation" || p125.Descriptor().Support.Family == "mutation" {
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
		p          spec.Profile
		origin     spec.Origin
		descriptor func(string) (spec.Profile, error)
	}{{p125.Descriptor(), p125.VerificationOrigin(), p125.DescriptorForArch}, {p126.Descriptor(), p126.VerificationOrigin(), p126.DescriptorForArch}} {
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
				p, err := candidate.descriptor(arch)
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
	hooks := p126.Descriptor().Hooks
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
	for _, path := range []string{"internal/compat/go125/host/coro.go", "internal/compat/go125/profile/policies.json", "internal/hostabi/coro_go125.go", "internal/compat/selected/go125.go", "internal/compat/spec/profile.go"} {
		write(path, "original")
	}
	digest := func() string {
		t.Helper()
		d, err := spec.SourceRevision(root, "go125")
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	original := digest()
	for _, path := range []string{"internal/compat/go125/host/coro.go", "internal/compat/go125/profile/policies.json", "internal/hostabi/coro_go125.go", "internal/compat/selected/go125.go", "internal/compat/spec/profile.go"} {
		write(path, "modified")
		if digest() == original {
			t.Fatalf("revision ignores %s", path)
		}
		write(path, "original")
	}
	for _, path := range []string{"internal/hostabi/coro_go126.go", "internal/compat/go125/host/coro_test.go", "internal/compat/go125/profile/revision.go", "internal/compat/go125/profile/tuples.json", "internal/compat/go125/profile/evidence.json"} {
		write(path, "not source for this identity")
	}
	if digest() != original {
		t.Fatal("other family, tests or evidence changed source identity")
	}
	if err := os.Rename(filepath.Join(root, "internal/compat/go125/host/coro.go"), filepath.Join(root, "internal/compat/go125/host/moved.go")); err != nil {
		t.Fatal(err)
	}
	if digest() == original {
		t.Fatal("adapter move did not change identity")
	}
}

func TestGo125ShimBindings(t *testing.T) {
	t.Parallel()
	shims := p125.ShimManifest()
	if err := spec.VerifyShimBindings("../../..", shims); err != nil {
		t.Fatal(err)
	}
	shims[0].Signature = "func()"
	if err := spec.VerifyShimBindings("../../..", shims); err == nil {
		t.Fatal("changed shim signature accepted")
	}
	if strings.HasPrefix(runtime.Version(), "go1.25.") {
		if err := spec.VerifyHooks(currentToolchain(), p125.ShimManifest()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGo126ShimBindings(t *testing.T) {
	t.Parallel()
	shims := p126.ShimManifest()
	if err := spec.VerifyShimBindings("../../..", shims); err != nil {
		t.Fatal(err)
	}
	shims[0].Signature = "func()"
	if err := spec.VerifyShimBindings("../../..", shims); err == nil {
		t.Fatal("changed shim signature accepted")
	}
	if strings.HasPrefix(runtime.Version(), "go1.26.") {
		if err := spec.VerifyHooks(currentToolchain(), p126.ShimManifest()); err != nil {
			t.Fatal(err)
		}
	}
}
