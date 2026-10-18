package spec_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGo125PublicShimInterfaces(t *testing.T) {
	if runtime.Version() != "go1.25.14" {
		t.Skip("checked by the exact Go 1.25.14 adapter gate")
	}
	for _, item := range []struct{ pkg, name, source string }{{"reflect", "Type", "type.go"}, {"testing", "TB", "testing.go"}} {
		want := interfaceMethods(t, filepath.Join(runtime.GOROOT(), "src", item.pkg, item.source), item.name)
		got := interfaceMethods(t, filepath.Join("../go125/shims", item.pkg, item.source), item.name)
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

func interfaceMethods(t *testing.T, path, name string) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string)
	ast.Inspect(file, func(node ast.Node) bool {
		typ, ok := node.(*ast.TypeSpec)
		if !ok || typ.Name.Name != name {
			return true
		}
		iface, ok := typ.Type.(*ast.InterfaceType)
		if !ok {
			t.Fatalf("%s is not an interface", name)
		}
		for _, method := range iface.Methods.List {
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
		return false
	})
	if len(result) == 0 {
		t.Fatalf("no public methods found for %s", name)
	}
	return result
}
