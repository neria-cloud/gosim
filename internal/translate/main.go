package translate

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/buildcfg"
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
	cfg, err := gosimtool.NewBuildConfig(input.Cfg.Selection, input.Cfg.Toolchain)
	if err != nil {
		return nil, err
	}
	project, err := gosimtool.FindGoModDir()
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
	projectPath, mod, err := gosimtool.FindGoMod()
	if err != nil {
		return nil, err
	}
	project := filepath.Dir(projectPath)
	inputDir, composition, frameworkRoot, err := prepareComposition(project, mod, cfg)
	if err != nil {
		return nil, err
	}
	appRoots, runtimeRoots, graph, err := loadSourceGraph(project, inputDir, patterns, cfg, policies)
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
	typed, err := reloadSelectedPackages(project, inputDir, appRoots, graph, misses, cfg)
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
	moduleFiles, err := selectedModuleFiles(project, mod, frameworkRoot, "translated", cfg.Toolchain.Version)
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
	for id := range misses {
		kind, path := classifyPackage(graph.nodes[id])
		if kind == PackageKindTestBinary {
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
		roots, err := loadSelectedPackages(dir, patterns, cfg, loadSyntaxAndTypes, tests)
		if err != nil {
			return nil, err
		}
		for _, pkg := range roots {
			typed[pkg.ID] = pkg
		}
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

func prepareComposition(project string, mod *modfile.File, cfg gosimtool.BuildConfig) (string, compose.GeneratedComposition, string, error) {
	cmd := buildcfg.Command(cfg.Toolchain, project, "list", "-m", "-json", gosimtool.Module)
	data, err := cmd.Output()
	if err != nil {
		return "", compose.GeneratedComposition{}, "", fmt.Errorf("locate framework module: %w", err)
	}
	var framework struct{ Dir string }
	if err := json.Unmarshal(data, &framework); err != nil {
		return "", compose.GeneratedComposition{}, "", err
	}
	if !filepath.IsAbs(framework.Dir) {
		return "", compose.GeneratedComposition{}, "", fmt.Errorf("framework module has no absolute source directory")
	}
	input, err := createSelectedDirectory(project, filepath.Join(".gosim", "v2", cfg.Digest, "input"))
	if err != nil {
		return "", compose.GeneratedComposition{}, "", err
	}
	dir, err := createSelectedDirectory(input, "gosimcomposition")
	if err != nil {
		return "", compose.GeneratedComposition{}, "", err
	}
	generated, err := compose.Generate(cfg.Selection, dir)
	if err != nil {
		return "", generated, "", err
	}
	modulePath := gosimtool.Module + "/.gosim/v2/" + cfg.Digest + "/input"
	files, err := selectedModuleFiles(project, mod, framework.Dir, modulePath, cfg.Toolchain.Version)
	if err != nil {
		return "", generated, "", err
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		if err := publishInputFile(filepath.Join(input, name), files[name], name == "go.sum"); err != nil {
			return "", generated, "", err
		}
	}
	return input, generated, framework.Dir, nil
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

func selectedModuleFiles(project string, original *modfile.File, frameworkRoot, modulePath, version string) (map[string][]byte, error) {
	data, err := original.Format()
	if err != nil {
		return nil, err
	}
	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, err
	}
	originalPath := mod.Module.Mod.Path
	if err := mod.AddModuleStmt(modulePath); err != nil {
		return nil, err
	}
	if err := mod.AddGoStmt(strings.TrimPrefix(version, "go")); err != nil {
		return nil, err
	}
	// Composition adds framework packages absent from the application's imports.
	// Include its declared graph up front, retaining the higher application
	// versions, so read-only package loading never rewrites this shared module.
	frameworkData, err := os.ReadFile(filepath.Join(frameworkRoot, "go.mod"))
	if err != nil {
		return nil, err
	}
	frameworkMod, err := modfile.Parse("go.mod", frameworkData, nil)
	if err != nil {
		return nil, err
	}
	versions := make(map[string]string)
	for _, requirement := range mod.Require {
		versions[requirement.Mod.Path] = requirement.Mod.Version
	}
	for _, requirement := range frameworkMod.Require {
		path, version := requirement.Mod.Path, requirement.Mod.Version
		if old := versions[path]; old == "" || semver.Compare(old, version) < 0 {
			if err := mod.AddRequire(path, version); err != nil {
				return nil, err
			}
		}
	}
	for _, replacement := range slices.Clone(mod.Replace) {
		if replacement.New.Version == "" && !filepath.IsAbs(replacement.New.Path) {
			if err := mod.AddReplace(replacement.Old.Path, replacement.Old.Version, filepath.Join(project, replacement.New.Path), ""); err != nil {
				return nil, err
			}
		}
	}
	for _, item := range []struct{ path, dir string }{{gosimtool.Module, frameworkRoot}, {originalPath, project}} {
		_, major, ok := module.SplitPathVersion(item.path)
		if !ok {
			return nil, fmt.Errorf("invalid module path %s", item.path)
		}
		v := "v0.0.0"
		if major != "" {
			v = strings.TrimPrefix(strings.TrimPrefix(major, "/"), ".") + ".0.0"
		}
		// The original application remains a dependency of this private module.
		// Preserve its higher framework requirement so MVS does not demand a
		// go.mod update while reloading application dependencies read-only.
		if old := versions[item.path]; semver.Compare(old, v) > 0 {
			v = old
		}
		if err := mod.AddRequire(item.path, v); err != nil {
			return nil, err
		}
		if err := mod.AddReplace(item.path, "", item.dir, ""); err != nil {
			return nil, err
		}
	}
	data, err = mod.Format()
	if err != nil {
		return nil, err
	}
	sums := make(map[string]bool)
	for _, root := range []string{project, frameworkRoot} {
		data, err := os.ReadFile(filepath.Join(root, "go.sum"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line != "" {
				sums[line] = true
			}
		}
	}
	lines := make([]string, 0, len(sums))
	for line := range sums {
		lines = append(lines, line)
	}
	slices.Sort(lines)
	return map[string][]byte{"go.mod": data, "go.sum": []byte(strings.Join(lines, "\n") + "\n")}, nil
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
