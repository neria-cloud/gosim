package gosimtool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func artifactIdentity() buildspec.Identity {
	d := strings.Repeat("a", 64)
	return buildspec.Identity{
		Schema:            2,
		Toolchain:         "go1.26.9",
		Profile:           "go126",
		ToolchainDigest:   d,
		StdlibDigest:      d,
		ContractDigest:    d,
		AdapterRevision:   d,
		RuntimeRevision:   d,
		HostOS:            "darwin",
		HostArch:          "arm64",
		SimulationOS:      "linux",
		SimulationArch:    "arm64",
		Features:          []string{"rt"},
		Tags:              []string{"sim", "linkname"},
		ModuleGraphDigest: d,
		TranslatorDigest:  d,
		DebugPolicyDigest: d,
	}
}

func artifactPackages(name string) []PackageDigest {
	return []PackageDigest{{Path: name, Digest: buildspec.Hash([]byte(name))}}
}

func fixtureBuild(directory string, id buildspec.Identity, packages []PackageDigest) (Manifest, error) {
	source := []byte("package sample\n")
	binary := []byte("executable fixture")
	if err := MaterializeFiles(directory, map[string][]byte{"src/sample.go": source}); err != nil {
		return Manifest{}, err
	}
	if err := os.WriteFile(filepath.Join(directory, "sample.test"), binary, 0o755); err != nil {
		return Manifest{}, err
	}
	return Manifest{
		Schema:         2,
		Identity:       id,
		PackageDigests: packages,
		Binary:         ArtifactFile{Path: "sample.test", SHA256: buildspec.Hash(binary)},
		GeneratedFiles: []ArtifactFile{{Path: "src/sample.go", SHA256: buildspec.Hash(source)}},
	}, nil
}

func TestArtifactCancellationDoesNotPublish(t *testing.T) {
	t.Parallel()
	root, id, packages := t.TempDir(), artifactIdentity(), artifactPackages("canceled")
	directory, err := ArtifactDirectory(root, id, packages)
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := buildspec.LockCache(t.Context(), directory+".lock")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
	defer cancel()
	_, err = BuildArtifact(ctx, root, id, packages, func(string) (Manifest, error) {
		t.Fatal("canceled lock waiter entered builder")
		return Manifest{}, nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	_, err = BuildArtifact(ctx, root, id, packages, func(stage string) (Manifest, error) {
		manifest, err := fixtureBuild(stage, id, packages)
		stop()
		return manifest, err
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled build published", err)
	}
	stages, err := filepath.Glob(filepath.Join(filepath.Dir(directory), ".build-*"))
	if err != nil || len(stages) != 0 {
		t.Fatal("canceled build leaked staging", stages, err)
	}
}

func TestArtifactCanonicalIdentityAndPackageSeparation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	id := artifactIdentity()
	packages := append(artifactPackages("z"), artifactPackages("a")...)
	calls := 0
	build := func(dir string) (Manifest, error) { calls++; return fixtureBuild(dir, id, packages) }
	a, err := BuildArtifact(t.Context(), root, id, packages, build)
	if err != nil {
		t.Fatal(err)
	}
	m, err := VerifyArtifact(a.Directory, id, packages)
	if err != nil {
		t.Fatal(err)
	}
	beforeTags := append([]string(nil), id.Tags...)
	beforePackages := append([]PackageDigest(nil), packages...)
	canonical, err := CanonicalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	m.PackageDigests[0], m.PackageDigests[1] = m.PackageDigests[1], m.PackageDigests[0]
	reordered, err := CanonicalManifest(m)
	if err != nil || !bytes.Equal(canonical, reordered) {
		t.Fatal("noncanonical manifest ordering", err)
	}
	if !reflect.DeepEqual(id.Tags, beforeTags) || !reflect.DeepEqual(packages, beforePackages) {
		t.Fatal("canonicalization mutated input")
	}
	again, err := BuildArtifact(
		t.Context(),
		root,
		id,
		packages,
		func(string) (Manifest, error) { t.Fatal("rebuilt complete artifact"); return Manifest{}, nil },
	)
	if err != nil || again.Directory != a.Directory || calls != 1 {
		t.Fatal("artifact reuse", err, calls)
	}
	other := artifactPackages("other")
	b, err := BuildArtifact(
		t.Context(),
		root,
		id,
		other,
		func(dir string) (Manifest, error) { return fixtureBuild(dir, id, other) },
	)
	if err != nil || b.Directory == a.Directory {
		t.Fatal("package artifacts collided", err)
	}
	sameContent := append([]PackageDigest(nil), packages...)
	sameContent[0].Path = "different-package-name"
	sameContentDir, err := ArtifactDirectory(root, id, sameContent)
	if err != nil || sameContentDir == a.Directory {
		t.Fatal("same-content packages collided", err)
	}
	otherPatch := id
	otherPatch.Toolchain = "go1.26.1"
	otherDir, err := ArtifactDirectory(root, otherPatch, packages)
	if err != nil || otherDir == a.Directory {
		t.Fatal("patch identity collision", err)
	}
	if _, err := VerifyArtifact(a.Directory, otherPatch, packages); err == nil {
		t.Fatal("accepted artifact compiled by another patch")
	}
	if _, err := VerifyArtifact(a.Directory, id, other); err == nil {
		t.Fatal("wrong source package set accepted")
	}
	if !strings.Contains(filepath.ToSlash(a.Directory), "/.gosim/v2/") ||
		!strings.Contains(filepath.ToSlash(a.Directory), "/packages/") {
		t.Fatal("wrong artifact namespace", a.Directory)
	}
}

func TestArtifactContentChecksAndFailedReplacement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	id := artifactIdentity()
	packages := artifactPackages("sample")
	builder := func(dir string) (Manifest, error) { return fixtureBuild(dir, id, packages) }
	a, err := BuildArtifact(t.Context(), root, id, packages, builder)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(a.Directory, "src/sample.go")
	before, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(source, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyArtifact(a.Directory, id, packages); err == nil {
		t.Fatal("source-content mutation with unchanged mtime accepted")
	}
	failed := errors.New("compiler failure")
	if _, err := BuildArtifact(t.Context(), root, id, packages, func(dir string) (Manifest, error) {
		os.WriteFile(filepath.Join(dir, "partial"), []byte("partial"), 0o600)
		return Manifest{}, failed
	}); !errors.Is(err, failed) {
		t.Fatal("failed builder was not reported", err)
	}
	data, err := os.ReadFile(source)
	if err != nil || string(data) != "corrupt" {
		t.Fatal("failed build overwrote existing artifact", err)
	}
	if _, err := BuildArtifact(t.Context(), root, id, packages, builder); err != nil {
		t.Fatal("corrupt artifact could not be repaired", err)
	}
	if _, err := VerifyArtifact(a.Directory, id, packages); err != nil {
		t.Fatal(err)
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(a.Directory), ".*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("leaked staging or replaced output", leftovers, err)
	}
	binary := filepath.Join(a.Directory, "sample.test")
	if err := os.Chmod(binary, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyArtifact(a.Directory, id, packages); err == nil {
		t.Fatal("nonexecutable binary accepted")
	}
	if err := os.Chmod(binary, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("wrong executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyArtifact(a.Directory, id, packages); err == nil {
		t.Fatal("binary hash not checked")
	}
}

func TestArtifactBuilderMustMatchManifest(t *testing.T) {
	t.Parallel()
	id := artifactIdentity()
	packages := artifactPackages("sample")
	for name, mutate := range map[string]func(string, *Manifest){
		"schema":    func(_ string, m *Manifest) { m.Schema = 1 },
		"identity":  func(_ string, m *Manifest) { m.Identity.RuntimeRevision = strings.Repeat("b", 64) },
		"package":   func(_ string, m *Manifest) { m.PackageDigests = artifactPackages("wrong") },
		"binary":    func(_ string, m *Manifest) { m.Binary.SHA256 = strings.Repeat("b", 64) },
		"missing":   func(_ string, m *Manifest) { m.GeneratedFiles[0].Path = "missing" },
		"traversal": func(_ string, m *Manifest) { m.Binary.Path = "../outside" },
		"extra": func(dir string, _ *Manifest) {
			os.WriteFile(filepath.Join(dir, "extra.go"), []byte("package extra"), 0o600)
		},
		"manifest": func(dir string, _ *Manifest) {
			os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("untrusted"), 0o600)
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			directory, err := ArtifactDirectory(root, id, packages)
			if err != nil {
				t.Fatal(err)
			}
			_, err = BuildArtifact(t.Context(), root, id, packages, func(dir string) (Manifest, error) {
				m, err := fixtureBuild(dir, id, packages)
				if err != nil {
					return m, err
				}
				mutate(dir, &m)
				return m, nil
			})
			if err == nil {
				t.Fatal("invalid artifact published")
			}
			if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed build left published output", err)
			}
		})
	}
}

func TestArtifactPathTraversalAndSymlinks(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", ".", "..", "../outside", "/outside", "a/../outside", "a//b", "./a", "C:/outside", "a\\b", "a\x00b", "manifest.json", "manifest.json/a"} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			dir := t.TempDir()
			if err := MaterializeFiles(dir, map[string][]byte{"valid.go": []byte("valid"), name: []byte("invalid")}); err == nil {
				t.Fatal("unsafe output accepted", name)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatal("wrote before validating paths", err)
			}
		})
	}
	dir := t.TempDir()
	if err := MaterializeFiles(dir, map[string][]byte{"a": nil, "a/b": nil}); err == nil {
		t.Fatal("file/directory conflict accepted")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := MaterializeFiles(dir, map[string][]byte{"link/escape": []byte("bad")}); err == nil {
		t.Fatal("followed output parent symlink")
	}
	if err := MaterializeFiles(filepath.Join(dir, "link"), map[string][]byte{"escape": []byte("bad")}); err == nil {
		t.Fatal("followed staging root symlink")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("escaped staging", err)
	}
	id := artifactIdentity()
	packages := artifactPackages("sample")
	root := t.TempDir()
	a, err := BuildArtifact(
		t.Context(),
		root,
		id,
		packages,
		func(dir string) (Manifest, error) { return fixtureBuild(dir, id, packages) },
	)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outside, "source")
	if err := os.WriteFile(target, []byte("package sample\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(a.Directory, "src/sample.go")
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, source); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyArtifact(a.Directory, id, packages); err == nil {
		t.Fatal("manifest accepted same-content symlink")
	}
	if _, err := BuildArtifact(t.Context(), root, id, packages, func(dir string) (Manifest, error) { return fixtureBuild(dir, id, packages) }); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "package sample\n" {
		t.Fatal("repair followed a symlink", err)
	}
}

func TestArtifactConcurrentBuilds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	id := artifactIdentity()
	packages := artifactPackages("same")
	var calls atomic.Int32
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := BuildArtifact(t.Context(), root, id, packages, func(dir string) (Manifest, error) {
				calls.Add(1)
				time.Sleep(20 * time.Millisecond)
				return fixtureBuild(dir, id, packages)
			})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("same artifact built concurrently", calls.Load())
	}
	// Different packages share an identity, but neither may serialize the other.
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	results = make(chan error, 2)
	for _, name := range []string{"one", "two"} {
		p := artifactPackages(name)
		go func() {
			_, err := BuildArtifact(
				t.Context(),
				root,
				id,
				p,
				func(dir string) (Manifest, error) { entered <- struct{}{}; <-release; return fixtureBuild(dir, id, p) },
			)
			results <- err
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("different package builds share a lock")
		}
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}

func TestArtifactProcessHelper(t *testing.T) {
	root := os.Getenv("GOSIM_ARTIFACT_PROCESS_TEST")
	if root == "" {
		t.Skip("subprocess helper")
	}
	id := artifactIdentity()
	packages := artifactPackages("process")
	_, err := BuildArtifact(t.Context(), root, id, packages, func(dir string) (Manifest, error) {
		f, err := os.OpenFile(filepath.Join(root, "builders"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return Manifest{}, err
		}
		_, writeErr := f.WriteString("builder\n")
		if err := errors.Join(writeErr, f.Close()); err != nil {
			return Manifest{}, err
		}
		time.Sleep(50 * time.Millisecond)
		return fixtureBuild(dir, id, packages)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestArtifactLockAcrossProcesses(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var commands []*exec.Cmd
	var outputs []*bytes.Buffer
	for i := 0; i < 2; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestArtifactProcessHelper$")
		cmd.Env = append(os.Environ(), "GOSIM_ARTIFACT_PROCESS_TEST="+root)
		out := new(bytes.Buffer)
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, cmd)
		outputs = append(outputs, out)
	}
	for i, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("builder child: %v: %s", err, outputs[i].String())
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "builders"))
	if err != nil || string(data) != "builder\n" {
		t.Fatal("process lock did not serialize/reuse artifact", string(data), err)
	}
}

func TestArtifactManifestRejectsUnknownAndTrailingData(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	id := artifactIdentity()
	packages := artifactPackages("schema")
	a, err := BuildArtifact(
		t.Context(),
		root,
		id,
		packages,
		func(dir string) (Manifest, error) { return fixtureBuild(dir, id, packages) },
	)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(a.Directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	unknown := append([]byte(`{"unknown":true,`), data[1:]...)
	for _, bad := range [][]byte{unknown, append(bytes.Clone(data), []byte(`{}`)...), []byte(`{`)} {
		if _, err := decodeManifest(bad); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
}

func TestArtifactNamespaceAndLockRejectSymlinks(t *testing.T) {
	t.Parallel()
	id := artifactIdentity()
	packages := artifactPackages("symlink")
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, OutputDirectory)); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildArtifact(t.Context(), root, id, packages, nil); err == nil {
		t.Fatal("artifact namespace symlink accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("artifact escaped project", err)
	}
	root = t.TempDir()
	directory, err := ArtifactDirectory(root, id, packages)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(directory), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outside, "lock-target")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, directory+".lock"); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildArtifact(t.Context(), root, id, packages, nil); err == nil {
		t.Fatal("lock symlink accepted")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "keep" {
		t.Fatal("lock followed symlink", err)
	}
}
