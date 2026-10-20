package translate

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"golang.org/x/tools/go/packages"
)

func TestReplacementGlobalUsesMachineContainer(t *testing.T) {
	t.Parallel()
	pkg := types.NewPackage("crypto/rand", "rand")
	obj := types.NewVar(token.NoPos, pkg, "Reader", types.Typ[types.Int])
	pkg.Scope().Insert(obj)
	original := ast.NewIdent("Reader")
	ident := &dst.Ident{Name: "Reader", Path: "translated/rand"}
	tr := packageTranslator{
		astMap:     decorator.AstMap{Nodes: map[dst.Node]ast.Node{ident: original}},
		typesInfo:  &types.Info{Uses: map[*ast.Ident]types.Object{original: obj}},
		profile:    &PackagePolicies{packages: map[string]string{"crypto/rand": "owned/rand"}},
		globalInfo: &GlobalInfo{ByPackage: map[string]*PackageGlobalInfo{"owned/rand": {Container: map[string]string{"Reader": "G"}}}},
	}
	got, ok := tr.isGlobalUse(ident)
	if !ok {
		t.Fatal("replacement global was not rewritten")
	}
	sel := got.(*dst.SelectorExpr)
	call := sel.X.(*dst.CallExpr).Fun.(*dst.Ident)
	if call.Name != "G" || call.Path != ident.Path || sel.Sel.Name != "Reader" {
		t.Fatalf("wrong container: %#v", got)
	}
}

func TestOptionalReplacementDoesNotSelectUnusedDependencies(t *testing.T) {
	t.Parallel()
	p, err := Policies(profileSelection(t, "rt"))
	if err != nil {
		t.Fatal(err)
	}
	root := &packages.Package{ID: "example.com/app", PkgPath: "example.com/app", Module: &packages.Module{Path: "example.com/app"}}
	// An unavailable operation in an unused shim must not poison this graph.
	optional := &packages.Package{ID: "net/http/httptest", PkgPath: "net/http/httptest"}
	graph, err := selectGraph([]*packages.Package{root}, p, optional)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(graph.order, []string{root.ID}) {
		t.Fatal(graph.order)
	}
}

func TestNamedLinknameAttachesToNamedDeclaration(t *testing.T) {
	t.Parallel()
	file, err := decorator.Parse(`package example
import _ "unsafe"
//go:linkname target example/other.target
func unrelated() {}
func target()
`)
	if err != nil {
		t.Fatal(err)
	}
	attachNamedLinknames(file)
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok {
			continue
		}
		found := false
		for _, comment := range fn.Decs.Start {
			if comment == "//go:linkname target example/other.target" {
				found = true
			}
		}
		if found != (fn.Name.Name == "target") {
			t.Fatalf("directive misplaced on %s", fn.Name.Name)
		}
	}
}

func TestNativeBoundarySelectsOnlyOwnedAdapters(t *testing.T) {
	t.Parallel()
	p, err := Policies(profileSelection(t, "rt"))
	if err != nil {
		t.Fatal(err)
	}
	target := p.replacements[PackageSelector{Pkg: "crypto/ed25519", Selector: "GenerateKey"}].Pkg
	adapter := &packages.Package{ID: target, PkgPath: target, Module: &packages.Module{Path: "github.com/neria-cloud/gosim"}}
	native := &packages.Package{ID: "crypto/ed25519", PkgPath: "crypto/ed25519", Imports: map[string]*packages.Package{
		"crypto/internal/sysrand": {ID: "crypto/internal/sysrand", PkgPath: "crypto/internal/sysrand"},
	}}
	graph, err := selectGraph([]*packages.Package{native}, p, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(graph.order, []string{target, native.ID}) || !graph.native[native.ID] || graph.native[target] {
		t.Fatalf("native computation or translated adapter misclassified: %v", graph.order)
	}
}

func TestSymbolAdapterGlobalsAndNativeImports(t *testing.T) {
	t.Parallel()
	pkg := types.NewPackage("hash/crc32", "crc32")
	obj := types.NewVar(token.NoPos, pkg, "IEEETable", types.Typ[types.Int])
	pkg.Scope().Insert(obj)
	original := ast.NewIdent("IEEETable")
	ident := &dst.Ident{Name: "IEEETable", Path: "hash/crc32"}
	tr := packageTranslator{
		astMap:     decorator.AstMap{Nodes: map[dst.Node]ast.Node{ident: original}},
		typesInfo:  &types.Info{Uses: map[*ast.Ident]types.Object{original: obj}},
		profile:    &PackagePolicies{replacements: map[PackageSelector]PackageSelector{{Pkg: pkg.Path(), Selector: obj.Name()}: {Pkg: "owned/crc32", Selector: obj.Name()}}},
		globalInfo: &GlobalInfo{ByPackage: map[string]*PackageGlobalInfo{"owned/crc32": {Container: map[string]string{obj.Name(): "G"}}}},
	}
	if _, ok := tr.isGlobalUse(ident); !ok {
		t.Fatal("symbol-adapted global missing")
	}
	tr.nativeImports = map[string]bool{pkg.Path(): true}
	if _, ok := tr.isGlobalUse(ident); ok {
		t.Fatal("owned native import was adapted recursively")
	}
	tr.rewriteIdentImpl(ident)
	if ident.Path != pkg.Path() {
		t.Fatal("native symbol redirected into its own adapter")
	}
}
