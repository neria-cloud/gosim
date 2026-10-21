package spec_test

import (
	"bytes"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestPinnedPublicShimInterfaces(t *testing.T) {
	t.Parallel()
	version, err := buildspec.ParseGoVersion(runtime.Version())
	if err != nil {
		t.Fatal(err)
	}
	support, err := buildspec.LookupFamily(version)
	if err != nil {
		t.Fatal(err)
	}
	family := support.Profile

	ctx := sourceContext(family, runtime.GOARCH)
	for _, item := range []struct{ pkg, name string }{{"reflect", "Type"}, {"testing", "TB"}} {
		want := interfaceMethods(t, filepath.Join(runtime.GOROOT(), "src", item.pkg), item.name, ctx)
		dir := filepath.Join("..", "shims", item.pkg)
		got := interfaceMethods(t, dir, item.name, ctx)
		if len(got) != len(want) {
			t.Errorf("%s method count: got %d want %d", item.name, len(got), len(want))
		}
		for name, signature := range want {
			if got[name] != signature {
				t.Errorf("%s.%s: got %q want %q", item.name, name, got[name], signature)
			}
		}
	}
}

func interfaceMethods(t *testing.T, dir, name string, ctx build.Context) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	interfaces := map[string]*ast.InterfaceType{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		active, err := ctx.MatchFile(dir, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if !active {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			typ, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if iface, ok := typ.Type.(*ast.InterfaceType); ok {
				if interfaces[typ.Name.Name] != nil {
					t.Fatalf("duplicate interface %s in %s", typ.Name.Name, dir)
				}
				interfaces[typ.Name.Name] = iface
			}
			return false
		})
	}
	result := make(map[string]string)
	seen := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		iface := interfaces[name]
		if iface == nil || seen[name] {
			t.Fatalf("missing or repeated interface %s in %s", name, dir)
		}
		seen[name] = true
		for _, method := range iface.Methods.List {
			if len(method.Names) == 0 {
				embedded, ok := method.Type.(*ast.Ident)
				if !ok {
					t.Fatalf("unhandled interface embedding %T", method.Type)
				}
				visit(embedded.Name)
				continue
			}
			if len(method.Names) != 1 || !method.Names[0].IsExported() {
				continue
			}
			ast.Inspect(method.Type, func(node ast.Node) bool {
				fn, ok := node.(*ast.FuncType)
				if !ok {
					return true
				}
				for _, fields := range []*ast.FieldList{fn.Params, fn.Results} {
					if fields == nil {
						continue
					}
					var expanded []*ast.Field
					for _, field := range fields.List {
						for i := 0; i < max(1, len(field.Names)); i++ {
							expanded = append(expanded, &ast.Field{Type: field.Type})
						}
					}
					fields.List = expanded
				}
				return true
			})
			var output bytes.Buffer
			if err := printer.Fprint(&output, token.NewFileSet(), method.Type); err != nil {
				t.Fatal(err)
			}
			result[method.Names[0].Name] = output.String()
		}
	}
	visit(name)
	if len(result) == 0 {
		t.Fatalf("no public methods found for %s", name)
	}
	return result
}
