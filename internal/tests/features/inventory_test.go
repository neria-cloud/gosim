package features

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFeatureApplicabilityAndPreservedInventory(t *testing.T) {
	for _, test := range []struct{ features, packages []string }{
		{[]string{"rt"}, []string{"./internal/tests/rt"}},
		{[]string{"rt", "fs"}, []string{"./internal/tests/rt", "./internal/tests/fs"}},
		{[]string{"rt", "net"}, []string{"./internal/tests/rt", "./internal/tests/net"}},
		{[]string{"rt", "fs", "net"}, []string{"./internal/tests/rt", "./internal/tests/fs", "./internal/tests/net", "./internal/tests/integration", "./nemesis", "./examples"}},
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
