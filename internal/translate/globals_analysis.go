package translate

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

type globalsCollector struct {
	typesInfo *types.Info
	astMap    decorator.AstMap
	pkgPath   string

	globalsFields []string
	shouldShare   map[string]bool
	tests         []string

	dontTranslate map[PackageSelector]bool
}

func collectGlobalsAndTests(
	files []*dst.File,
	astMap decorator.AstMap,
	pkgPath string,
	typesInfo *types.Info,
	dontTranslate map[PackageSelector]bool,
) *globalsCollector {
	c := &globalsCollector{
		typesInfo: typesInfo,
		astMap:    astMap,
		pkgPath:   pkgPath,

		shouldShare: make(map[string]bool),

		dontTranslate: dontTranslate,
	}

	for _, file := range files {
		c.collectGlobals(file)
	}

	return c
}

func (t *globalsCollector) collectGlobalsDecl(genDecl *dst.GenDecl) {
	if genDecl.Tok != token.VAR {
		return
	}
	for _, spec := range genDecl.Specs {
		spec := spec.(*dst.ValueSpec)

		// XXX: handle single/multi assignment gracefully
		// XXX: do initializers in TypesInfo get split?

		for _, name := range spec.Names {
			obj, ok := t.typesInfo.Defs[t.astMap.Nodes[name].(*ast.Ident)]
			if !ok {
				continue
			}

			if _, ok := obj.(*types.Var); !ok {
				continue
			}

			if name.Name == "_" {
				continue
			}

			shouldShare := t.dontTranslate[PackageSelector{Pkg: t.pkgPath, Selector: name.Name}]

			t.globalsFields = append(t.globalsFields, name.Name)
			if shouldShare {
				t.shouldShare[name.Name] = true
			}
		}
	}
}

func (t *globalsCollector) collectFuncDecl(funcDecl *dst.FuncDecl) {
	if !strings.HasPrefix(funcDecl.Name.Name, "Test") {
		return
	}
	if funcDecl.Recv != nil {
		return
	}
	if funcDecl.Type.TypeParams != nil {
		return
	}
	if funcDecl.Type.Results != nil {
		return
	}
	if len(funcDecl.Type.Params.List) != 1 {
		return
	}
	param := funcDecl.Type.Params.List[0]
	if len(param.Names) != 1 {
		return
	}
	unaryExpr, ok := param.Type.(*dst.StarExpr)
	if !ok {
		return
	}
	ident, ok := unaryExpr.X.(*dst.Ident)
	if !ok {
		return
	}
	if ident.Path != "testing" || ident.Name != "T" {
		return
	}
	t.tests = append(t.tests, funcDecl.Name.Name)
}

func (t *globalsCollector) collectGlobals(file *dst.File) {
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*dst.GenDecl); ok {
			t.collectGlobalsDecl(genDecl)
		}
		if funcDecl, ok := decl.(*dst.FuncDecl); ok {
			t.collectFuncDecl(funcDecl)
		}
	}
}
