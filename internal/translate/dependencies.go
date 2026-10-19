package translate

import (
	"fmt"
	"path/filepath"
	"slices"

	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/gosimtool"
)

// InspectDependencies selects and hashes current source inputs without reading
// the translation cache or compiling. Reloading the selected graph detects new
// and removed Go/embedded files as well as changes to existing file contents.
// Keys are translated package paths, exactly as in TranslateOutput.Deps.
func InspectDependencies(input *TranslateInput) (map[string]map[string]string, error) {
	if input == nil || len(input.Packages) == 0 {
		return nil, fmt.Errorf("dependency inspection requires explicit packages and build selection")
	}
	if err := input.Cfg.Validate(); err != nil {
		return nil, err
	}
	if err := validateLoaderWorker(input.Cfg.Toolchain); err != nil {
		return nil, err
	}
	cfg, err := gosimtool.NewBuildConfig(input.Cfg.Selection, input.Cfg.Toolchain)
	if err != nil {
		return nil, err
	}
	policies, err := Policies(cfg.Selection)
	if err != nil {
		return nil, err
	}
	projectPath, mod, err := gosimtool.FindGoMod()
	if err != nil {
		return nil, err
	}
	project := filepath.Dir(projectPath)
	inputDir, _, _, err := prepareComposition(project, mod, cfg)
	if err != nil {
		return nil, err
	}
	appRoots, runtimeRoots, graph, err := loadSourceGraph(project, inputDir, slices.Clone(input.Packages), cfg, policies)
	if err != nil {
		return nil, err
	}
	return packageDependencies(projectPath, appRoots, runtimeRoots, graph)
}

func loadSourceGraph(project, inputDir string, patterns []string, cfg gosimtool.BuildConfig, policies PackagePolicies) ([]*packages.Package, []*packages.Package, *selectedGraph, error) {
	// Composition exists before either application or infrastructure package load.
	appRoots, err := loadSelectedPackages(project, patterns, cfg, packages.NeedDeps, true)
	if err != nil {
		return nil, nil, nil, err
	}
	runtimeRoots, err := loadSelectedPackages(inputDir, []string{"./gosimcomposition"}, cfg, packages.NeedDeps, false)
	if err != nil {
		return nil, nil, nil, err
	}
	graph, err := selectGraph(append(slices.Clone(appRoots), runtimeRoots...), policies)
	return appRoots, runtimeRoots, graph, err
}

// A binary's inputs include its test variants and the selected composition, not
// unrelated packages named in the same build invocation. This makes a later
// single-package lookup agree with a batch build.
func packageDependencies(projectPath string, appRoots, runtimeRoots []*packages.Package, graph *selectedGraph) (map[string]map[string]string, error) {
	result := make(map[string]map[string]string)
	for _, root := range appRoots {
		kind, base := classifyPackage(root)
		if kind != PackageKindBase {
			continue
		}
		if graph.nodes[root.ID] == nil || graph.native[root.ID] {
			return nil, fmt.Errorf("root %s is not a translated application package", root.PkgPath)
		}
		seen := make(map[string]bool)
		files := []string{projectPath}
		var visit func(string)
		visit = func(id string) {
			if seen[id] {
				return
			}
			seen[id] = true
			p := graph.nodes[id]
			files = append(files, p.CompiledGoFiles...)
			files = append(files, p.OtherFiles...)
			files = append(files, p.EmbedFiles...)
			for m := p.Module; m != nil; m = m.Replace {
				if m.GoMod != "" {
					files = append(files, m.GoMod)
				}
			}
			for _, dep := range graph.deps[id] {
				visit(dep)
			}
		}
		for _, pkg := range appRoots {
			_, pkgBase := classifyPackage(pkg)
			if pkgBase == base {
				visit(pkg.ID)
			}
		}
		for _, pkg := range runtimeRoots {
			visit(pkg.ID)
		}
		digests, err := contentDigests(files)
		if err != nil {
			return nil, err
		}
		result[gosimtool.GetNewPackageName(root.PkgPath)] = digests
	}
	return result, nil
}
