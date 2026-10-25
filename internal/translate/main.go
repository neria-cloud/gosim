package translate

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compose"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/translate/cache"
)

type TranslateInput struct {
	Packages []string
	Cfg      gosimtool.BuildConfig
}

func Translate(input *TranslateInput) (output *gosimtool.TranslateOutput, err error) {
	if input == nil || len(input.Packages) == 0 {
		return nil, fmt.Errorf("translation requires explicit packages and build selection")
	}
	if err := input.Cfg.Validate(); err != nil {
		return nil, err
	}
	if err := validateLoaderWorker(input.Cfg.Toolchain); err != nil {
		return nil, err
	}
	// Own all selection slices before asynchronous package loading/translation.
	cfg, err := gosimtool.NewBuildConfig(input.Cfg.Context, input.Cfg.Selection, input.Cfg.Resolution)
	if err != nil {
		return nil, err
	}
	project, err := cfg.ProjectRoot()
	if err != nil {
		return nil, err
	}
	namespace, err := createSelectedDirectory(project, filepath.Join(".gosim", "v2", cfg.Digest))
	if err != nil {
		return nil, err
	}
	db, err := cache.NewDB(filepath.Join(project, ".gosim", "cache.sqlite3"))
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	cached := cache.NewCache(db)
	defer func() { err = errors.Join(err, cached.Clean()) }()
	stage, err := os.MkdirTemp(namespace, "translation-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(stage))
		}
	}()
	return translatePackages(cached, slices.Clone(input.Packages), stage, cfg)
}

func translatePackages(cached *cache.Cache, patterns []string, rootOutputDir string, cfg gosimtool.BuildConfig) (*gosimtool.TranslateOutput, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := validateLoaderWorker(cfg.Toolchain); err != nil {
		return nil, err
	}
	policies, err := Policies(cfg.Selection)
	if err != nil {
		return nil, err
	}
	projectPath, err := cfg.ContextFile()
	if err != nil {
		return nil, err
	}
	project := filepath.Dir(projectPath)
	inputDir, composition, err := prepareComposition(project, cfg)
	if err != nil {
		return nil, err
	}
	appRoots, runtimeRoots, graph, err := loadSourceGraph(cfg.Resolution.Context.Directory, inputDir, patterns, cfg, policies)
	if err != nil {
		return nil, err
	}
	infrastructure := make(map[string]bool)
	var markInfrastructure func(string)
	markInfrastructure = func(id string) {
		if infrastructure[id] {
			return
		}
		infrastructure[id] = true
		for _, dep := range graph.deps[id] {
			markInfrastructure(dep)
		}
	}
	for _, root := range runtimeRoots {
		markInfrastructure(root.ID)
	}
	definitions, err := pushedDefinitions(graph, policies)
	if err != nil {
		return nil, err
	}

	toolHash, err := computeTranslateToolHash(cfg.Identity)
	if err != nil {
		return nil, err
	}
	hashes := make(map[string]Hash)
	results := make(map[string]*TranslatePackageResult)
	misses := make(map[string]bool)
	for _, id := range graph.order {
		dependencies := make(map[string]Hash)
		for _, dep := range graph.deps[id] {
			dependencies[dep] = hashes[dep]
		}
		hash, err := computePackageHash(toolHash, graph.nodes[id], dependencies)
		if err != nil {
			return nil, err
		}
		hashes[id] = hash
		if graph.native[id] {
			continue
		}
		result, hit, err := cacheGet(cached, hash, cfg.Identity)
		if err != nil {
			return nil, err
		}
		if hit {
			results[id] = result
		} else {
			misses[id] = true
		}
	}
	typed, err := reloadSelectedPackages(cfg.Resolution.Context.Directory, inputDir, appRoots, graph, misses, cfg)
	if err != nil {
		return nil, err
	}
	replaced, names := selectedPackageNames(graph, policies)
	compositionPath, err := compose.PackagePath(cfg.Digest)
	if err != nil {
		return nil, err
	}
	for _, id := range graph.order {
		if graph.native[id] || !misses[id] {
			continue
		}
		imports := make(map[string]*TranslatePackageResult)
		var gather func(string)
		seen := make(map[string]bool)
		gather = func(id string) {
			if seen[id] {
				return
			}
			seen[id] = true
			for _, dep := range graph.deps[id] {
				gather(dep)
				if result := results[dep]; result != nil {
					imports[dep] = result
				}
			}
		}
		gather(id)
		result, err := translatePackage(&translatePackageArgs{cfg: cfg, Selection: cfg.Selection, infrastructure: infrastructure[id], definitions: definitions, pkg: graph.nodes[id], hooksPackage: compositionPath, packageNames: names, pkgWithTypesAndAst: typed[id], replacedPkgs: replaced, importResults: imports})
		if err != nil {
			return nil, err
		}
		if err := cachePut(cached, hashes[id], CacheEntryV2{Format: 2, IdentityDigest: cfg.Digest, PackageDigest: hashHex(hashes[id]), Result: *result}); err != nil {
			return nil, err
		}
		results[id] = result
	}
	files := make(map[string][]byte)
	for _, id := range graph.order {
		if result := results[id]; result != nil {
			for name, data := range result.TranslatedFiles {
				if previous, exists := files[name]; exists && !bytes.Equal(previous, data) {
					return nil, fmt.Errorf("conflicting translated output %s", name)
				}
				files[name] = data
			}
		}
	}
	moduleFiles, err := selectedModuleFiles(cfg, "translated")
	if err != nil {
		return nil, err
	}
	for name, data := range moduleFiles {
		files[name] = data
	}
	if err := compose.ValidateImports(composition, cfg.Selection); err != nil {
		return nil, err
	}
	if err := gosimtool.MaterializeFiles(rootOutputDir, files); err != nil {
		return nil, err
	}
	dependencies, err := packageDependencies(projectPath, appRoots, runtimeRoots, graph)
	if err != nil {
		return nil, err
	}
	output := &gosimtool.TranslateOutput{RootOutputDir: rootOutputDir, Deps: dependencies, Identity: cfg.Identity}
	for pkg := range dependencies {
		output.Packages = append(output.Packages, pkg)
	}
	slices.Sort(output.Packages)
	return output, nil
}

func hashHex(hash Hash) string { return hex.EncodeToString(hash[:]) }

func reloadSelectedPackages(project, inputDir string, appRoots []*packages.Package, graph *selectedGraph, misses map[string]bool, cfg gosimtool.BuildConfig) (map[string]*packages.Package, error) {
	app := make(map[string]bool)
	for _, pkg := range appRoots {
		kind, path := classifyPackage(pkg)
		if kind == PackageKindBase {
			app[path] = true
		}
	}
	groups := map[bool]map[string]bool{false: {}, true: {}}
	indirectTests := false
	for id := range misses {
		pkg := graph.nodes[id]
		kind, path := classifyPackage(pkg)
		if kind == PackageKindTestBinary {
			continue
		}
		if owner := testOwner(pkg); owner != "" {
			groups[true][owner] = true
			if kind == PackageKindBase {
				indirectTests = true
			}
			continue
		}
		groups[app[path]][path] = true
	}
	typed := make(map[string]*packages.Package)
	for _, tests := range []bool{false, true} {
		var patterns []string
		for path := range groups[tests] {
			patterns = append(patterns, path)
		}
		slices.Sort(patterns)
		if len(patterns) == 0 {
			continue
		}
		dir := inputDir
		if tests {
			dir = project
		}
		mode := loadSyntaxAndTypes
		if tests && indirectTests {
			// Induced variants are dependencies of the owning test roots, not
			// roots themselves. Request their syntax and types as well.
			mode |= packages.NeedDeps
		}
		roots, err := loadSelectedPackages(dir, patterns, cfg, mode, tests)
		if err != nil {
			return nil, err
		}
		packages.Visit(roots, func(pkg *packages.Package) bool {
			if misses[pkg.ID] {
				typed[pkg.ID] = pkg
			}
			return tests && indirectTests
		}, nil)
	}
	return typed, nil
}

func selectedPackageNames(graph *selectedGraph, p PackagePolicies) (map[string]string, map[string]string) {
	replaced, names := make(map[string]string), make(map[string]string)
	for _, pkg := range graph.all {
		names[pkg.PkgPath] = pkg.Name
	}
	for id, pkg := range graph.nodes {
		if graph.native[id] {
			replaced[pkg.PkgPath] = pkg.PkgPath
			continue
		}
		if kind, _ := classifyPackage(pkg); kind == PackageKindTestBinary {
			continue
		}
		target := "translated/" + gosimtool.ReplaceSpecialPackages(pkg.PkgPath)
		replaced[pkg.PkgPath] = target
		names[target] = pkg.Name
	}
	for from, to := range p.packages {
		replaced[from] = replaced[to]
	}
	for path := range p.native {
		replaced[path] = path
	}
	return replaced, names
}

func prepareComposition(project string, cfg gosimtool.BuildConfig) (string, compose.GeneratedComposition, error) {
	input, err := createSelectedDirectory(project, filepath.Join(".gosim", "v2", cfg.Digest, "input"))
	if err != nil {
		return "", compose.GeneratedComposition{}, err
	}
	dir, err := createSelectedDirectory(input, "gosimcomposition")
	if err != nil {
		return "", compose.GeneratedComposition{}, err
	}
	generated, err := compose.Generate(cfg.Selection, dir)
	if err != nil {
		return "", generated, err
	}
	modulePath := gosimtool.Module + "/.gosim/v2/" + cfg.Digest + "/input"
	files, err := selectedModuleFiles(cfg, modulePath)
	if err != nil {
		return "", generated, err
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		if err := publishInputFile(filepath.Join(input, name), files[name], name == "go.sum"); err != nil {
			return "", generated, err
		}
	}
	return input, generated, nil
}

// Publish a complete file exclusively. go.sum may gain verified entries during
// loading; retain an existing regular sum file instead of racing Go's updates.
func publishInputFile(path string, data []byte, mutable bool) error {
	check := func() error {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("input %s is not a regular file", path)
		}
		if mutable {
			return nil
		}
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(existing, data) {
			return fmt.Errorf("composition module changed under the same identity: %s", path)
		}
		return nil
	}
	if err := check(); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".module-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	writeErr := temp.Chmod(0o644)
	if writeErr == nil {
		_, writeErr = temp.Write(data)
	}
	if err := errors.Join(writeErr, temp.Close()); err != nil {
		return err
	}
	if err := os.Link(temp.Name(), path); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return check()
}

func selectedModuleFiles(cfg gosimtool.BuildConfig, modulePath string) (map[string][]byte, error) {
	modules, err := cfg.ModuleGraph()
	if err != nil {
		return nil, err
	}
	return buildspec.ModuleFiles(cfg.Resolution, modules, modulePath)
}

func createSelectedDirectory(root, relative string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("source project must have an absolute path")
	}
	if err := gosimtool.ValidateArtifactPath(filepath.ToSlash(relative)); err != nil {
		return "", err
	}
	current := root
	for _, part := range strings.Split(filepath.ToSlash(relative), "/") {
		current = filepath.Join(current, part)
		if err := os.Mkdir(current, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", fmt.Errorf("selected source parent is not a real directory: %s", current)
		}
	}
	return current, nil
}
