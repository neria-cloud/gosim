package translate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

type selectedGraph struct {
	all    map[string]*packages.Package
	nodes  map[string]*packages.Package
	deps   map[string][]string
	native map[string]bool
	order  []string
}

func selectGraph(roots []*packages.Package, p PackagePolicies, available ...*packages.Package) (*selectedGraph, error) {
	g := &selectedGraph{
		all: make(map[string]*packages.Package), nodes: make(map[string]*packages.Package),
		deps: make(map[string][]string), native: make(map[string]bool),
	}
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
	// Replacement packages are available for lookup, but only become selected
	// when a reachable import uses them. Optional shims must not force their
	// dependencies or initialization into every composition.
	for _, root := range available {
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
				// Symbol adapters are dependencies of the native boundary. Only
				// their owned packages are traversed, never native internals.
				visiting[pkg.ID] = true
				var targets []string
				for from, to := range p.replacements {
					if from.Pkg == pkg.PkgPath {
						targets = append(targets, to.Pkg)
					}
				}
				slices.Sort(targets)
				for _, path := range slices.Compact(targets) {
					target := byPath[path]
					if target == nil {
						return fmt.Errorf("selected adapter %s is not loaded", path)
					}
					if err := visit(target); err != nil {
						return err
					}
					g.deps[pkg.ID] = append(g.deps[pkg.ID], target.ID)
				}
				delete(visiting, pkg.ID)
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
		// A translated package may replace selected public symbols without
		// admitting the rest of that package to native execution.
		for from, to := range p.replacements {
			if from.Pkg == pkg.PkgPath {
				dep := byPath[to.Pkg]
				if dep == nil {
					return fmt.Errorf("selected adapter %s is not loaded", to.Pkg)
				}
				dependencies = append(dependencies, dep)
			}
		}
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

// A bodyless declaration is valid when another selected translated package
// pushes its implementation with linkname (notably os -> net.newUnixFile).
// Definitions removed by hooks and all native destinations are excluded.
func pushedDefinitions(graph *selectedGraph, policies PackagePolicies) (map[PackageSelector]bool, error) {
	definitions := make(map[PackageSelector]bool)
	for _, id := range graph.order {
		if graph.native[id] {
			continue
		}
		pkg := graph.nodes[id]
		for _, path := range pkg.CompiledGoFiles {
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
			if err != nil {
				return nil, err
			}
			bodies := make(map[string]bool)
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil && fn.Recv == nil {
					_, hooked := policies.hooks[PackageSelector{Pkg: pkg.PkgPath, Selector: fn.Name.Name}]
					bodies[fn.Name.Name] = !hooked
				}
			}
			for _, group := range file.Comments {
				for _, comment := range group.List {
					fields := strings.Fields(comment.Text)
					if len(fields) != 3 || fields[0] != "//go:linkname" || !bodies[fields[1]] {
						continue
					}
					target, err := splitSymbol(fields[2])
					if err != nil {
						return nil, err
					}
					policy, err := policies.packagePolicy(
						target.Pkg, !strings.Contains(strings.Split(target.Pkg, "/")[0], "."),
					)
					if err == nil && policy.Mode == "translate" {
						definitions[target] = true
					}
				}
			}
		}
	}
	return definitions, nil
}
