package gosimtool

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"golang.org/x/mod/modfile"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/race"
)

const (
	Module          = "github.com/neria-cloud/gosim"
	OutputDirectory = ".gosim"
)

func FindGoMod() (string, *modfile.File, error) {
	baseDir, err := os.Getwd()
	if err != nil {
		return "", nil, err
	}

	for i := 0; i < 20; i++ {
		p := path.Join(baseDir, "go.mod")
		bytes, err := os.ReadFile(p)
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

		file, err := modfile.Parse(p, bytes, nil)
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

func PreparedTestInfoName(pkg string) string {
	return strings.ReplaceAll(pkg, "/", "-") + ".info"
}

func GetNewPackageName(pkg string) string {
	return "translated/" + ReplaceSpecialPackages(pkg)
}

var (
	buildOnceMu sync.Mutex
	buildOnce   = make(map[string]*sync.Once)
)

// TODO(migration): Task 23 adds the explicit expected identity parameter and
// removes package-only build ownership. The worker already reloads current
// sources and verifies the complete identity/package/content artifact.
func GetPathForPrecompiledTestBinary(t *testing.T, pkg string) string {
	t.Helper()
	invoke := func(command string) string {
		var child *exec.Cmd
		args := []string{command}
		if race.Enabled {
			args = append(args, "-race")
		}
		args = append(args, pkg)
		if worker := os.Getenv("GOSIMTOOL"); worker != "" {
			child = exec.Command(worker, args...)
		} else {
			args = append([]string{"run", "-ldflags=-checklinkname=0", "-tags=linkname", Module + "/cmd/gosim"}, args...)
			child = exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), args...)
		}
		child.Env = append(os.Environ(), "GOTOOLCHAIN=local", "PATH="+filepath.Join(runtime.GOROOT(), "bin")+string(filepath.ListSeparator)+os.Getenv("PATH"))
		output, err := child.Output()
		if err != nil {
			var exit *exec.ExitError
			var detail string
			if errors.As(err, &exit) {
				detail = string(exit.Stderr)
			}
			t.Fatalf("Could not find pre-built gosim test binary for %s: %v\n%s", pkg, err, detail)
		}
		return strings.TrimSpace(string(output))
	}
	if isUncachedTestRun() {
		buildOnceMu.Lock()
		once := buildOnce[pkg]
		if once == nil {
			once = new(sync.Once)
			buildOnce[pkg] = once
		}
		buildOnceMu.Unlock()
		once.Do(func() {
			t.Log("looks like an uncached test run... running gosim build-tests", pkg)
			invoke("build-tests")
		})
	}
	var lookup struct {
		Binary  string
		Sources map[string]string
	}
	if err := json.Unmarshal([]byte(invoke("lookup-test")), &lookup); err != nil {
		t.Fatal(err)
	}
	// Observe the freshly discovered inputs in this host test process as well.
	// Go's outer test-result cache cannot see filesystem reads in the worker.
	for source, digest := range lookup.Sources {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if buildspec.Hash(data) != digest {
			t.Fatalf("source changed during artifact lookup: %s", source)
		}
		if _, err := os.ReadDir(filepath.Dir(source)); err != nil {
			t.Fatal(err)
		}
	}
	path := lookup.Binary
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		t.Fatalf("invalid prebuilt worker response %q", path)
	}
	t.Logf("Using pre-built gosim test binary %q", path)
	return path
}
