package spec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func inspectionReferenceAllowed(path, function string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	switch path {
	case "internal/buildspec/command.go", "internal/buildspec/modules.go", "internal/buildspec/modulefiles.go", "internal/buildcfg/inspection.go":
		return true
	case "internal/compat/spec/hooks.go":
		return function == "PrepareSourceInspectionContext"
	case "internal/compat/spec/source.go":
		return function == "inspectSourcesForObservation"
	}
	return strings.HasPrefix(path, "internal/compat/audit/")
}

func TestInspectionConsumerBoundary(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			function := ""
			if fn, ok := decl.(*ast.FuncDecl); ok {
				function = fn.Name.Name
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				id, ok := node.(*ast.Ident)
				if !ok {
					return true
				}
				switch id.Name {
				case "SourceInspectionCommand", "SourceInspectionApplicationCommand", "ResolveInspectionModuleGraph", "SourceInspectionModuleFiles", "PrepareSourceInspectionContext", "DiscoverInspection":
					seen[id.Name] = true
					if !inspectionReferenceAllowed(rel, function) {
						t.Errorf("inspection wrapper %s used by %s:%s", id.Name, rel, function)
					}
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 6 {
		t.Fatal("boundary scan did not visit all wrappers", seen)
	}
	for _, path := range []string{"internal/buildcfg/toolchain.go", "internal/buildrun/build.go", "internal/compat/selected/selection.go", "internal/compat/spec/contracts.go", "internal/compat/spec/hooks.go"} {
		if inspectionReferenceAllowed(path, "VerifyContracts") {
			t.Fatal("ordinary path admitted", path)
		}
	}
}
