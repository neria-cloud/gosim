package translate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"

	"golang.org/x/tools/go/packages"
)

type selectedGraph struct {
	all    map[string]*packages.Package
	nodes  map[string]*packages.Package
	deps   map[string][]string
	native map[string]bool
	order  []string
}

func selectGraph(roots []*packages.Package, p PackagePolicies) (*selectedGraph, error) {
	g := &selectedGraph{all: make(map[string]*packages.Package), nodes: make(map[string]*packages.Package), deps: make(map[string][]string), native: make(map[string]bool)}
	byPath := make(map[string]*packages.Package)
	var gather func(*packages.Package)
	gather = func(pkg *packages.Package) {
		if g.all[pkg.ID] != nil {
			return
		}
		g.all[pkg.ID] = pkg
		if pkg.ID == pkg.PkgPath {
			byPath[pkg.PkgPath] = pkg
		}
		for _, key := range sortedImportPaths(pkg) {
			gather(pkg.Imports[key])
		}
	}
	for _, root := range roots {
		gather(root)
	}
	visiting := make(map[string]bool)
	var visit func(*packages.Package) error
	visit = func(pkg *packages.Package) error {
		if visiting[pkg.ID] {
			return fmt.Errorf("translated dependency cycle at %s", pkg.ID)
		}
		if g.nodes[pkg.ID] != nil {
			return nil
		}
		if len(pkg.Errors) > 0 {
			return fmt.Errorf("source package %s: %s", pkg.ID, pkg.Errors[0])
		}
		kind, base := classifyPackage(pkg)
		if kind != PackageKindTestBinary {
			policy, err := p.packagePolicy(pkg.PkgPath, standardPackage(pkg))
			if err != nil {
				return err
			}
			if policy.Mode != "translate" {
				if replacement := p.packages[pkg.PkgPath]; replacement != "" {
					target := byPath[replacement]
					if target == nil {
						return fmt.Errorf("selected replacement %s is not loaded", replacement)
					}
					return visit(target)
				}
				g.nodes[pkg.ID] = pkg
				g.native[pkg.ID] = true
				g.order = append(g.order, pkg.ID)
				return nil
			}
		}
		visiting[pkg.ID] = true
		native, err := sourceNativeImports(pkg)
		if err != nil {
			return err
		}
		used, err := survivingImports(pkg, p)
		if err != nil {
			return err
		}
		var dependencies []*packages.Package
		for _, path := range sortedImportPaths(pkg) {
			dep := pkg.Imports[path]
			if kind == PackageKindTestBinary {
				_, depBase := classifyPackage(dep)
				if depBase != base {
					continue
				}
			} else {
				if !used[path] {
					continue
				}
				if err := p.importPolicy(pkg.PkgPath, dep.PkgPath, standardPackage(dep), native[path]); err != nil {
					return fmt.Errorf("%s imports %s: %w", pkg.ID, path, err)
				}
				if native[path] {
					continue
				}
			}
			if replacement := p.packages[dep.PkgPath]; replacement != "" {
				dep = byPath[replacement]
				if dep == nil {
					return fmt.Errorf("replacement %s is not loaded", replacement)
				}
			}
			dependencies = append(dependencies, dep)
		}
		if kind == PackageKindForTest || kind == PackageKindTests {
			if dep := byPath[base]; dep != nil {
				dependencies = append(dependencies, dep)
			} else {
				return fmt.Errorf("test package %s lacks base %s", pkg.ID, base)
			}
		}
		slices.SortFunc(dependencies, func(a, b *packages.Package) int {
			if a.ID < b.ID {
				return -1
			}
			if a.ID > b.ID {
				return 1
			}
			return 0
		})
		for _, dep := range dependencies {
			if err := visit(dep); err != nil {
				return fmt.Errorf("%s -> %s: %w", pkg.ID, dep.ID, err)
			}
			if len(g.deps[pkg.ID]) == 0 || g.deps[pkg.ID][len(g.deps[pkg.ID])-1] != dep.ID {
				g.deps[pkg.ID] = append(g.deps[pkg.ID], dep.ID)
			}
		}
		delete(visiting, pkg.ID)
		g.nodes[pkg.ID] = pkg
		g.order = append(g.order, pkg.ID)
		return nil
	}
	for _, root := range roots {
		if err := visit(root); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// Hook bodies are replaced before output, so their exclusive dependencies must
// not enter the translated graph. Keep signatures and all other declarations;
// matching identifiers conservatively retains imports shadowed by local names.
// Dot and side-effect imports are always retained for subsequent policy checks.
func survivingImports(pkg *packages.Package, p PackagePolicies) (map[string]bool, error) {
	used := make(map[string]bool)
	for _, path := range pkg.CompiledGoFiles {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return nil, err
		}
		names := make(map[string]string)
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return nil, err
			}
			dep := pkg.Imports[path]
			if dep == nil {
				return nil, fmt.Errorf("unresolved source import %s in %s", path, pkg.ID)
			}
			name := dep.Name
			if imp.Name != nil {
				name = imp.Name.Name
			}
			if name == "." || name == "_" {
				used[path] = true
			} else {
				names[name] = path
			}
		}
		for _, decl := range file.Decls {
			if d, ok := decl.(*ast.GenDecl); ok && d.Tok == token.IMPORT {
				continue
			}
			if fn, ok := decl.(*ast.FuncDecl); ok {
				if _, hooked := p.hooks[PackageSelector{Pkg: pkg.PkgPath, Selector: fn.Name.Name}]; hooked {
					fn.Body = nil // parsed private copy; original type checking stays intact
				}
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				if ident, ok := node.(*ast.Ident); ok {
					if path := names[ident.Name]; path != "" {
						used[path] = true
					}
				}
				return true
			})
		}
	}
	return used, nil
}

func sortedImportPaths(pkg *packages.Package) []string {
	paths := make([]string, 0, len(pkg.Imports))
	for path := range pkg.Imports {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

// A path is native only if every selected source import of it has the explicit
// directive. Metadata alone does not silently turn ordinary imports native.
func sourceNativeImports(pkg *packages.Package) (map[string]bool, error) {
	native := make(map[string]bool)
	for _, path := range pkg.CompiledGoFiles {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return nil, err
			}
			n := nativeDirective(imp)
			if old, exists := native[path]; exists {
				n = n && old
			}
			native[path] = n
		}
	}
	return native, nil
}
