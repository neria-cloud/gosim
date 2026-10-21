package routegen

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestCurrentCompilerFirstOriginBootstrap(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	resolution, err := buildcfg.ResolveRunningProcess(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	tc := resolution.Toolchain
	v, err := buildspec.ParseGoVersion(tc.Version)
	if err != nil {
		t.Fatal(err)
	}
	support, err := buildspec.LookupFamily(v)
	if err != nil {
		t.Fatal(err)
	}
	root := copyGenerationSource(t)
	binary := filepath.Join(t.TempDir(), "gosim-gen")
	build := buildspec.OutputCommand(ctx, tc, root, "build", "-ldflags=-checklinkname=0", "-tags=linkname", "-o", binary, "./cmd/gosim-gen")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	run := func(wantError string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = root
		cmd.Env = build.Env
		output, err := cmd.CombinedOutput()
		if wantError == "" {
			if err != nil {
				t.Fatalf("%v: %v\n%s", args, err, output)
			}
		} else if err == nil || !strings.Contains(string(output), wantError) {
			t.Fatalf("%v: wanted %q, got %v\n%s", args, wantError, err, output)
		}
	}
	dir := filepath.Join(root, "internal/compat/profiles")
	other := filepath.Join(dir, otherFamily(support.Profile)+".origin.json")
	protected, err := os.ReadFile(other)
	if err != nil {
		t.Fatal(err)
	}
	originPath := filepath.Join(dir, support.Profile+".origin.json")
	if err := os.Remove(originPath); err != nil {
		t.Fatal(err)
	}
	run("", "profiles", "-root=.")
	run("origin.json", "profiles", "-root=.", "-check")
	args := []string{"-profile=" + support.Profile, "-toolchain=" + tc.Version, "-arches=" + tc.HostArch, "-output=."}
	run("origin.json", append(slices.Clone(args), "-check")...)
	// Refresh bypasses only an absent file, never malformed existing provenance.
	if err := os.WriteFile(originPath, []byte("malformed"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("invalid character", append(slices.Clone(args), "-refresh-origin")...)
	if err := os.Remove(originPath); err != nil {
		t.Fatal(err)
	}
	definitionPath := filepath.Join(dir, support.Profile+".json")
	original, err := os.ReadFile(definitionPath)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := spec.DecodeDefinition(original)
	if err != nil {
		t.Fatal(err)
	}
	definition.Supported = false
	changed, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(definitionPath, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	run("disagrees with generated support", append(slices.Clone(args), "-refresh-origin")...)
	if err := os.WriteFile(definitionPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := spec.SourceRevision(root, support.Profile)
	if err != nil {
		t.Fatal(err)
	}
	run("", append(slices.Clone(args), "-refresh-origin")...)
	after, err := spec.SourceRevision(root, support.Profile)
	if err != nil || before != after {
		t.Fatal("provenance-only bootstrap changed adapter revision", err)
	}
	run("", "profiles", "-root=.")
	run("", "profiles", "-root=.", "-check")
	origin, err := spec.LoadOrigin(os.DirFS(dir), support.Profile)
	if err != nil {
		t.Fatal(err)
	}
	if origin.BaselineToolchain != tc.Version {
		t.Fatal("bootstrap did not record running patch")
	}
	untouched, err := os.ReadFile(other)
	if err != nil || !bytes.Equal(protected, untouched) {
		t.Fatal("another origin changed", err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".gosim")); err != nil {
		t.Fatal(err)
	}
	run("", append(slices.Clone(args), "-check")...)
	if _, err := os.Stat(filepath.Join(root, ".gosim")); !os.IsNotExist(err) {
		t.Fatal("check published ignored output", err)
	}
}

func copyGenerationSource(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	err = filepath.WalkDir(source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if slices.Contains([]string{".git", ".gosim", "docs", "test-reports", "__pycache__"}, entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}
