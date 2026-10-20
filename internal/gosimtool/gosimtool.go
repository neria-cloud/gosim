package gosimtool

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/mod/modfile"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/hostabi"
	"github.com/neria-cloud/gosim/internal/workerboot"
)

const (
	Module            = "github.com/neria-cloud/gosim"
	OutputDirectory   = ".gosim"
	moduleSearchLimit = 20
)

func FindGoMod() (string, *modfile.File, error) {
	baseDir, err := os.Getwd()
	if err != nil {
		return "", nil, err
	}

	for i := 0; i < moduleSearchLimit; i++ {
		p := path.Join(baseDir, "go.mod")
		moduleBytes, err := os.ReadFile(p)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return "", nil, err
			}

			baseDir = path.Dir(baseDir)
			if baseDir != "." {
				continue
			}
			return "", nil, errors.New("could not find go.mod file, hit root")
		}

		file, err := modfile.Parse(p, moduleBytes, nil)
		if err != nil {
			return "", nil, err
		}

		return p, file, nil
	}

	return "", nil, errors.New("could not find go.mod file, tried too many dirs")
}

func FindGoModDir() (string, error) {
	modFile, _, err := FindGoMod()
	if err != nil {
		return "", err
	}

	modDir := filepath.Dir(modFile)
	return modDir, nil
}

// MakeGoModForTest makes a go.mod and go.sum for tests that use the `gosim` CLI
// outside of the gosim module. It takes the go.mod from the gosim module strips
// it, and adds a `replace` directive pointing to the gosim module.
//
// Only dependencies that are listed in the original go.mod work.
func MakeGoModForTest(goSimModDir, testModDir string, requires []string) error {
	origModPath := filepath.Join(goSimModDir, "go.mod")
	origModBytes, err := os.ReadFile(origModPath)
	if err != nil {
		return err
	}

	origFile, err := modfile.Parse(origModPath, origModBytes, nil)
	if err != nil {
		return err
	}

	origSumBytes, err := os.ReadFile(filepath.Join(goSimModDir, "go.sum"))
	if err != nil {
		return err
	}

	origRequires := make(map[string]*modfile.Require)
	for _, mod := range origFile.Require {
		origRequires[mod.Mod.Path] = mod
	}

	newFile := &modfile.File{}
	newFile.AddModuleStmt("test")
	newFile.AddRequire(origFile.Module.Mod.Path, "v1.0.0")
	newFile.AddGoStmt(origFile.Go.Version)
	for _, pkg := range requires {
		require, ok := origRequires[pkg]
		if !ok {
			return fmt.Errorf("could not find pkg %s", pkg)
		}
		newFile.AddRequire(require.Mod.Path, require.Mod.Version)
	}
	newFile.AddReplace(origFile.Module.Mod.Path, "", goSimModDir, "")
	newModBytes, err := newFile.Format()
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(testModDir, "go.mod"), newModBytes, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(testModDir, "go.sum"), origSumBytes, 0o644); err != nil {
		return err
	}
	return nil
}

func isUncachedTestRun() bool {
	// TODO: fallback only allowed if -count=1 (which bans caching)
	flags := []string{"-test.count="}

	for _, arg := range os.Args {
		for _, flag := range flags {
			// if we have this flag and it looks non-empty, assume non-cached test run
			if strings.HasPrefix(arg, flag) && len(arg) > len(flag) {
				return true
			}
		}
	}

	return false
}

func ReplaceSpecialPackages(s string) string {
	changed := false
	parts := strings.Split(s, "/")
	for i, part := range parts {
		if part == "vendor" || part == "internal" {
			parts[i] = part + "_"
			changed = true
		}
	}
	if !changed {
		return s
	}
	return strings.Join(parts, "/")
}

func PreparedTestBinName(pkg string) string {
	return strings.ReplaceAll(pkg, "/", "-") + ".test"
}

func GetNewPackageName(pkg string) string {
	return "translated/" + ReplaceSpecialPackages(pkg)
}

// invokeTestWorker fixes the compiler to the running native test process. The
// caller's selected source is bootstrapped through the same cache as the CLI.
func invokeTestWorker(t *testing.T, patch, command string, args ...string) []byte {
	t.Helper()
	ctx, cancel := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	r, err := buildcfg.ResolveRunningProcess(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Toolchain.Version != patch {
		t.Fatalf("test compiler is %s, artifact requests %s; re-run go test", r.Toolchain.Version, patch)
	}
	if err := buildcfg.ValidateWorker(r.Toolchain); err != nil {
		t.Fatal(err)
	}
	source, err := workerboot.ResolveSource(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := buildcfg.Fingerprint(r.Toolchain)
	if err != nil {
		t.Fatal(err)
	}
	var worker workerboot.Worker
	if override := os.Getenv("GOSIMTOOL"); override != "" {
		worker, err = workerboot.OpenOverride(ctx, override, r, source, inputs)
	} else {
		worker, err = workerboot.Ensure(ctx, r, source, inputs, "")
	}
	if err != nil {
		t.Fatal(err)
	}
	var output, diagnostics bytes.Buffer
	code, err := workerboot.Run(
		ctx,
		worker,
		r,
		append([]string{command}, args...),
		workerboot.Streams{Out: &output, Err: &diagnostics},
	)
	if err != nil || code != 0 {
		t.Fatalf("gosim worker %s: exit status %d: %v\n%s", command, code, err, diagnostics.String())
	}
	return output.Bytes()
}

// ResolveTestIdentity resolves config in the host process so the outer Go test
// cache observes it, then asks the exact worker for the complete source identity.
func ResolveTestIdentity(t *testing.T, configPath string) buildspec.Identity {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	resolvedConfigPath, err := workerboot.ConfigPath(cwd, configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := buildcfg.Load(resolvedConfigPath, buildcfg.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := buildcfg.ResolveRunningProcess(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ExplicitToolchain != "" && cfg.ExplicitToolchain != resolution.Toolchain.Version {
		t.Fatal("configured compiler differs from running Go test; migrate config to schema 2")
	}
	var args []string
	if configPath != "" {
		args = []string{"-config", configPath}
	}
	if hostabi.RaceEnabled {
		args = append(args, "-race")
	}
	var id buildspec.Identity
	if err := json.Unmarshal(invokeTestWorker(t, resolution.Toolchain.Version, "resolve-build", args...), &id); err != nil {
		t.Fatal(err)
	}
	return id
}

// GetPathForPrecompiledTestBinary reloads the current dependency closure and
// verifies exactly the supplied identity, package digest and artifact bytes.
func GetPathForPrecompiledTestBinary(t *testing.T, pkg string, identity buildspec.Identity) string {
	t.Helper()
	digest, err := buildspec.Digest(identity)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-go", identity.Toolchain, "-features", strings.Join(identity.Features, ",")}
	if identity.Race {
		args = append(args, "-race")
	}
	args = append(args, pkg)
	if isUncachedTestRun() {
		t.Log("looks like an uncached test run... running gosim build-tests", pkg)
		invokeTestWorker(t, identity.Toolchain, "build-tests", args...)
	}
	var lookup struct {
		Binary   string
		Sources  map[string]string
		Identity buildspec.Identity
	}
	if err := json.Unmarshal(invokeTestWorker(t, identity.Toolchain, "lookup-test", args...), &lookup); err != nil {
		t.Fatal(err)
	}
	actual, err := buildspec.Digest(lookup.Identity)
	if err != nil || actual != digest {
		t.Fatalf("prebuilt identity mismatch: %v", err)
	}
	// Worker reads alone are invisible to Go's outer test-result cache.
	for source, want := range lookup.Sources {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if buildspec.Hash(data) != want {
			t.Fatalf("source changed during artifact lookup: %s", source)
		}
		if _, err := os.ReadDir(filepath.Dir(source)); err != nil {
			t.Fatal(err)
		}
	}
	if !filepath.IsAbs(lookup.Binary) || filepath.Clean(lookup.Binary) != lookup.Binary {
		t.Fatalf("invalid prebuilt worker response %q", lookup.Binary)
	}
	t.Logf("Using pre-built gosim test binary %q", lookup.Binary)
	return lookup.Binary
}

// ManifestForBinary verifies the complete artifact once more before launch.
func ManifestForBinary(binary string, identity buildspec.Identity) (Manifest, error) {
	dir := filepath.Dir(binary)
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Manifest{}, err
	}
	m, err := decodeManifest(data)
	if err != nil {
		return Manifest{}, err
	}
	m, err = VerifyArtifact(dir, identity, m.PackageDigests)
	if err != nil {
		return Manifest{}, err
	}
	if filepath.Join(dir, m.Binary.Path) != binary {
		return Manifest{}, errors.New("manifest binary path mismatch")
	}
	return m, nil
}
