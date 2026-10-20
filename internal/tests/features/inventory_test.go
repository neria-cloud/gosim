package features

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestFeatureApplicabilityAndPreservedInventory(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ features, packages []string }{
		{[]string{"rt"}, []string{"./internal/tests/rt", "./examples/simple"}},
		{[]string{"rt", "fs"}, []string{"./internal/tests/rt", "./internal/tests/fs", "./examples/simple", "./examples/bolt"}},
		{[]string{"rt", "net"}, []string{"./internal/tests/rt", "./internal/tests/net", "./examples/simple", "./examples/network/machines", "./examples/network/latency"}},
		{[]string{"rt", "fs", "net"}, []string{"./internal/tests/rt", "./internal/tests/fs", "./internal/tests/net", "./internal/tests/integration", "./nemesis", "./examples/simple", "./examples/bolt", "./examples/network/machines", "./examples/network/latency"}},
	} {
		got, err := Packages(test.features)
		if err != nil || !slices.Equal(got, test.packages) {
			t.Fatal(test.features, got, err)
		}
	}
	data, err := os.ReadFile("inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct{ Source, Destination, Name, Constraint string }
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 207 {
		t.Fatal("baseline inventory changed", len(entries))
	}
	for _, entry := range entries {
		path := filepath.Join("..", entry.Destination)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, data, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		if entry.Constraint != "" && !strings.HasPrefix(string(data), entry.Constraint+"\n") {
			t.Errorf("changed applicability: %s %s", entry.Destination, entry.Name)
		}
		found := false
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == entry.Name {
				found = true
			}
		}
		if !found {
			t.Errorf("lost baseline %s:%s (mapped to %s)", entry.Source, entry.Name, entry.Destination)
		}
	}
}

func TestNativeUnitsExcludeDisabledModels(t *testing.T) {
	t.Parallel()
	for _, names := range [][]string{{"rt"}, {"rt", "fs"}, {"rt", "net"}} {
		packages, err := NativePackages(names)
		if err != nil {
			t.Fatal(err)
		}
		args := append([]string{"list", "-tags=linkname", "-deps", "-test"}, packages...)
		command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), args...)
		command.Dir = "../../.."
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("native graph: %v: %s", err, output)
		}
		for _, domain := range []string{"fs", "net"} {
			if slices.Contains(names, domain) {
				continue
			}
			prefix := "github.com/neria-cloud/gosim/internal/" + domain
			for _, path := range strings.Fields(string(output)) {
				if path == prefix || strings.HasPrefix(path, prefix+"/") {
					t.Fatalf("%v imports disabled native model %s", names, path)
				}
			}
		}
	}
}

func TestMetatestPackagesExcludeSimOnlyExamples(t *testing.T) {
	t.Parallel()
	for _, names := range [][]string{{"rt"}, {"rt", "fs"}, {"rt", "net"}, {"rt", "fs", "net"}} {
		packages, err := MetatestPackages(names)
		if err != nil {
			t.Fatal(err)
		}
		positive, err := Packages(names)
		if err != nil {
			t.Fatal(err)
		}
		for _, pkg := range packages {
			if !slices.Contains(positive, pkg) || strings.HasPrefix(pkg, "./examples/") && pkg != "./examples/simple" {
				t.Fatalf("invalid native metatest package %s for %v", pkg, names)
			}
		}
		args := append([]string{"list", "-tags=linkname"}, packages...)
		command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), args...)
		command.Dir = "../../.."
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("native metatest packages: %v: %s", err, output)
		}
		if !slices.Contains(packages, "./internal/tests/rt") || !slices.Contains(packages, "./examples/simple") {
			t.Fatalf("lost runtime metatest coverage: %v", packages)
		}
	}
}
