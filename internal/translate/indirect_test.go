package translate

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

func TestIndirectTestDependencyClassification(t *testing.T) {
	for _, tc := range []struct {
		id, path, name, owner string
		kind                  packageKind
	}{
		{"example/leaf", "example/leaf", "leaf", "", PackageKindBase},
		{"example/leaf [example/leaf.test]", "example/leaf", "leaf", "example/leaf", PackageKindForTest},
		{"example/parent [example/leaf.test]", "example/parent", "parent", "example/leaf", PackageKindBase},
		{"example/leaf_test [example/leaf.test]", "example/leaf_test", "leaf_test", "example/leaf", PackageKindTests},
		{"example/leaf.test", "example/leaf.test", "main", "", PackageKindTestBinary},
	} {
		t.Run(tc.id, func(t *testing.T) {
			p := &packages.Package{ID: tc.id, PkgPath: tc.path, Name: tc.name}
			got, _ := classifyPackage(p)
			if got != tc.kind || testOwner(p) != tc.owner {
				t.Fatalf("kind %s owner %q", got, testOwner(p))
			}
		})
	}
}

func TestIndirectTestDependencyGraph(t *testing.T) {
	dir := t.TempDir()
	write := func(name, source string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	leafSource := write("leaf.go", "package leaf\nvar Value = 1\n")
	extraSource := write("extra_test.go", "package leaf\nvar TestOnly = Value\n")
	parentSource := write("parent.go", "package parent\nimport \"example/leaf\"\nvar Value = leaf.TestOnly\n")
	externalSource := write("external_test.go", "package leaf_test\nimport \"example/parent\"\nvar Value = parent.Value\n")
	module := &packages.Module{Path: "example"}
	leaf := &packages.Package{ID: "example/leaf", PkgPath: "example/leaf", Name: "leaf", Module: module, CompiledGoFiles: []string{leafSource}}
	augmented := &packages.Package{ID: "example/leaf [example/leaf.test]", PkgPath: leaf.PkgPath, Name: leaf.Name, Module: module, CompiledGoFiles: []string{leafSource, extraSource}}
	parent := &packages.Package{ID: "example/parent [example/leaf.test]", PkgPath: "example/parent", Name: "parent", Module: module, CompiledGoFiles: []string{parentSource}, Imports: map[string]*packages.Package{leaf.PkgPath: augmented}}
	external := &packages.Package{ID: "example/leaf_test [example/leaf.test]", PkgPath: "example/leaf_test", Name: "leaf_test", Module: module, CompiledGoFiles: []string{externalSource}, Imports: map[string]*packages.Package{parent.PkgPath: parent}}
	policies, err := Policies(profileSelection(t, "rt"))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := selectGraph([]*packages.Package{leaf, augmented, external}, policies)
	if err != nil {
		t.Fatal(err)
	}
	if graph.nodes[parent.ID] != parent || graph.nodes[parent.PkgPath] != nil {
		t.Fatal("induced identity was replaced with a nonexistent base")
	}
	if !slices.Contains(graph.deps[parent.ID], augmented.ID) {
		t.Fatal("test-context dependency was lost")
	}
	h, err := computePackageHash(Hash{}, parent, map[string]Hash{augmented.ID: {1}})
	if err != nil {
		t.Fatal(err)
	}
	other := *parent
	other.ID = "example/parent [example/other.test]"
	h2, err := computePackageHash(Hash{}, &other, map[string]Hash{augmented.ID: {1}})
	if err != nil {
		t.Fatal(err)
	}
	h3, err := computePackageHash(Hash{}, parent, map[string]Hash{augmented.ID: {2}})
	if err != nil {
		t.Fatal(err)
	}
	if h == h2 || h == h3 {
		t.Fatal("test context or dependency omitted from cache identity")
	}
}

// This small CLI regression is ordinary coverage, independent of the optional
// large translation sweep. It checks actual production globals/initializers,
// a dependency using test-only declarations, batch loading and cached reruns.
func TestIndirectTestDependencyTranslation(t *testing.T) {
	framework, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	write := func(name, source string) {
		t.Helper()
		p := filepath.Join(fixture, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	modBytes, err := os.ReadFile(filepath.Join(framework, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := modfile.Parse("go.mod", modBytes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := mod.AddModuleStmt("example.com/indirecttest"); err != nil {
		t.Fatal(err)
	}
	if err := mod.AddRequire(gosimModPath, "v0.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := mod.AddReplace(gosimModPath, "", framework, ""); err != nil {
		t.Fatal(err)
	}
	modBytes, err = mod.Format()
	if err != nil {
		t.Fatal(err)
	}
	write("go.mod", string(modBytes))
	sums, err := os.ReadFile(filepath.Join(framework, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	write("go.sum", string(sums))
	write("leaf/leaf.go", `package leaf
var Value = 1
func init() { Value++ }
`)
	write("leaf/internal_test.go", `package leaf
import "testing"
var TestOnly = 5
func TestInternal(t *testing.T) { if Value != 2 { t.Fatal(Value) } }
`)
	write("parent.go", `package parent
import "example.com/indirecttest/leaf"
var Captured = leaf.Value
func init() { Captured++ }
func Read() int { return leaf.Value }
func Add(v int) { leaf.Value += v }
`)
	write("helper/helper.go", `package helper
import "example.com/indirecttest/leaf"
func ReadTestOnly() int { return leaf.TestOnly }
`)
	write("leaf/external_test.go", `package leaf_test
import (
 "testing"
 parent "example.com/indirecttest"
 "example.com/indirecttest/helper"
)
func TestExternal(t *testing.T) {
 if parent.Captured != 3 || parent.Read() != 2 || helper.ReadTestOnly() != 5 { t.Fatal("initializers or test context lost") }
 parent.Add(4)
 if parent.Read() != 6 { t.Fatal("global mutation lost") }
}
`)
	write("other/other.go", "package other\n")
	write("other/other_test.go", `package other
import (
 "testing"
 parent "example.com/indirecttest"
)
func TestOther(t *testing.T) {
 if parent.Captured != 3 || parent.Read() != 2 { t.Fatal("base and induced globals differ") }
}
`)
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	env := append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "PATH="+filepath.Join(runtime.GOROOT(), "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	run := func(dir, bin string, args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), bin, args...)
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", bin, args, err, out)
		}
		return out
	}
	launcher := filepath.Join(t.TempDir(), "gosim")
	run(framework, goBin, "build", "-ldflags=-checklinkname=0", "-tags=linkname", "-o", launcher, "./cmd/gosim")
	native := run(fixture, goBin, "test", "-ldflags=-checklinkname=0", "-tags=linkname", "-count=1", "-v", "./leaf", "./other")
	for _, name := range []string{"TestInternal", "TestExternal", "TestOther"} {
		if !bytes.Contains(native, []byte("--- PASS: "+name+" (")) {
			t.Fatalf("native %s missing:\n%s", name, native)
		}
	}
	// Both cold and warm runs must execute every requested test, with the same
	// complete identity, without compiling parent/helper as independent test roots.
	for attempt := range 2 {
		out := run(fixture, launcher, "test", "-features", "rt", "-seeds", "1-2", "-run", "^Test(Internal|External|Other)$", "./leaf", "./other")
		for _, name := range []string{"TestInternal", "TestExternal", "TestOther"} {
			if bytes.Count(out, []byte("--- PASS: "+name+" (")) != 2 {
				t.Fatalf("attempt %d: missing passes for %s:\n%s", attempt, name, out)
			}
		}
		if strings.Contains(string(out), "--- SKIP:") {
			t.Fatalf("skipped regression:\n%s", out)
		}
	}
}
