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

func collectGlobalsAndTests(files []*dst.File, astMap decorator.AstMap, pkgPath string, typesInfo *types.Info, dontTranslate map[PackageSelector]bool) *globalsCollector {
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

var globalsDontTranslateGo123 = map[PackageSelector]bool{
	{Pkg: "crypto/sha512", Selector: "_K"}: true,
	// {pkg: "sync/atomic", selector: "firstStoreInProgress"}: true, // no way... universe strikes once again
	{Pkg: "encoding/json", Selector: "fieldCache"}:   true,
	{Pkg: "encoding/json", Selector: "encoderCache"}: true,
	// this would be nice but breaks determinism because sometimes the cache works.......
	// XXX: register all types in our own registry, warm up the cache?
	{Pkg: "vendor/golang.org/x/text/unicode/norm", Selector: "decomps"}:          true,
	{Pkg: "vendor/golang.org/x/text/unicode/norm", Selector: "nfcValues"}:        true,
	{Pkg: "vendor/golang.org/x/text/unicode/norm", Selector: "nfcIndex"}:         true,
	{Pkg: "vendor/golang.org/x/text/unicode/norm", Selector: "nfcSparseValues"}:  true,
	{Pkg: "vendor/golang.org/x/text/unicode/norm", Selector: "nfkcValues"}:       true,
	{Pkg: "vendor/golang.org/x/text/unicode/norm", Selector: "nfkcIndex"}:        true,
	{Pkg: "vendor/golang.org/x/text/unicode/norm", Selector: "nfkcSparseValues"}: true,
	{Pkg: "golang.org/x/text/unicode/norm", Selector: "decomps"}:                 true,
	{Pkg: "golang.org/x/text/unicode/norm", Selector: "nfcValues"}:               true,
	{Pkg: "golang.org/x/text/unicode/norm", Selector: "nfcIndex"}:                true,
	{Pkg: "golang.org/x/text/unicode/norm", Selector: "nfcSparseValues"}:         true,
	{Pkg: "golang.org/x/text/unicode/norm", Selector: "nfkcValues"}:              true,
	{Pkg: "golang.org/x/text/unicode/norm", Selector: "nfkcIndex"}:               true,
	{Pkg: "golang.org/x/text/unicode/norm", Selector: "nfkcSparseValues"}:        true,
	{Pkg: "vendor/golang.org/x/text/unicode/bidi", Selector: "bidiValues"}:       true,
	{Pkg: "vendor/golang.org/x/text/unicode/bidi", Selector: "bidiIndex"}:        true,
	{Pkg: "golang.org/x/text/unicode/bidi", Selector: "bidiValues"}:              true,
	{Pkg: "golang.org/x/text/unicode/bidi", Selector: "bidiIndex"}:               true,
	{Pkg: "vendor/golang.org/x/net/idna", Selector: "idnaValues"}:                true,
	{Pkg: "vendor/golang.org/x/net/idna", Selector: "idnaIndex"}:                 true,
	{Pkg: "vendor/golang.org/x/net/idna", Selector: "idnaSparseValues"}:          true,
	{Pkg: "golang.org/x/net/idna", Selector: "idnaValues"}:                       true,
	{Pkg: "golang.org/x/net/idna", Selector: "idnaIndex"}:                        true,
	{Pkg: "golang.org/x/net/idna", Selector: "idnaSparseValues"}:                 true,
	{Pkg: reflectPackage, Selector: "mapInterfaceType"}:                          true,
	{Pkg: reflectPackage, Selector: "jankHashMap"}:                               true,

	// xxx amd64 asm
	{Pkg: "crypto/sha256", Selector: "useSHA"}:                                true,
	{Pkg: "crypto/sha256", Selector: "useAVX2"}:                               true,
	{Pkg: "vendor/golang.org/x/crypto/chacha20poly1305", Selector: "useAVX2"}: true,
	{Pkg: "crypto/internal/bigmod", Selector: "supportADX"}:                   true,

	{Pkg: "github.com/cespare/xxhash/v2", Selector: "primes"}: true,
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

			n := name.Name

			shouldShare := false

			if t.dontTranslate[PackageSelector{Pkg: t.pkgPath, Selector: name.Name}] {
				shouldShare = true
			}

			// XXX: reuse existing err globals?
			/*
				if t.pkgPath == "io" && initializer.Lhs[0].Name() == "EOF" {
					rhs = &dst.Ident{
						Path: "io",
						Name: "EOF",
					}
				}
			*/
			/*
				// XXX: this causes problems for errors.Is(statErr, ErrNotExist)
				if typ, ok := t.TypesInfo.Types[initializer.Rhs]; ok {
					if named, ok := typ.Type.(*types.Named); ok && named.Obj().Name() == "error" {
						if name := initializer.Lhs[0].Name(); token.IsExported(name) && !isInternal(t.pkgPath) && !isVendor(t.pkgPath) {
							// XXX: can we go even deeper and rewrite all references / skip the variable altogether?
							rhs = &dst.Ident{
								Name: name,
								Path: t.pkgPath,
							}
						}
					}
				}
			*/

			if len(spec.Values) > 0 {
				if callExpr, ok := spec.Values[0].(*dst.CallExpr); ok {
					if ident, ok := callExpr.Fun.(*dst.Ident); ok && ident.Name == "MustCompile" && ident.Path == "regexp" {
						shouldShare = true
					}
					if ident, ok := callExpr.Fun.(*dst.Ident); ok && ident.Name == "NewReplacer" && ident.Path == "strings" {
						shouldShare = true
					}
				}
				if callExpr, ok := spec.Values[0].(*dst.CallExpr); ok {
					if ident, ok := callExpr.Fun.(*dst.Ident); ok && ident.Name == "New" && ident.Path == "errors" {
						shouldShare = true
					}
				}
			}

			if t.pkgPath == "golang.org/x/net/http2/hpack" && n == "staticTable" {
				shouldShare = true
			}
			if t.pkgPath == "regexp/syntax" && n == "posixGroup" {
				shouldShare = true
			}
			if t.pkgPath == "hash/crc32" && n == "IEEETable" {
				shouldShare = true
			}
			if t.pkgPath == "crypto/internal/edwards25519" {
				shouldShare = true
			}
			if t.pkgPath == "vendor/golang.org/x/net/http2/hpack" && n == "staticTable" {
				shouldShare = true
			}
			if t.pkgPath == "golang.org/x/net/http/httpguts" && n == "badTrailer" {
				shouldShare = true
			}
			if t.pkgPath == "google.golang.org/grpc/internal/transport" && n == "http2ErrConvTab" {
				shouldShare = true
			}
			if t.pkgPath == "google.golang.org/grpc/internal/transport" && n == "HTTPStatusConvTab" {
				shouldShare = true
			}
			if t.pkgPath == "github.com/google/go-cmp/cmp" && n == "randBool" {
				shouldShare = true
			}
			if t.pkgPath == "github.com/google/go-cmp/cmp/internal/diff" && n == "randBool" {
				shouldShare = true
			}
			if t.pkgPath == "html/template" {
				shouldShare = true
			}
			if t.pkgPath == "unicode" {
				shouldShare = true
			}

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
