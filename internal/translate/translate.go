package translate

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log/slog"
	"maps"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gotypes"
	"github.com/dave/dst/dstutil"
	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

type translatePackageArgs struct {
	Selection          buildspec.Selection
	legacy             bool // TODO(migration): remove in Task 22.
	cfg                gosimtool.BuildConfig
	pkg                *packages.Package
	hooksPackage       string
	packageNames       map[string]string
	pkgWithTypesAndAst *packages.Package
	replacedPkgs       map[string]string
	importResults      map[string]*TranslatePackageResult
}

type PackageGlobalInfo struct {
	Container   map[string]string
	ShouldShare map[string]bool
	Rewritten   map[string]string
}

type GlobalInfo struct {
	ByPackage map[string]*PackageGlobalInfo
}

func (gi *GlobalInfo) merge(pkg string, globals *PackageGlobalInfo) {
	// XXX: these copies are a little jank. fixes race. handle more cleverly?
	// (eg. keep both around?) happens for packages with test versions. why do
	// they even show up here anyway? you can't import them.
	if existing, ok := gi.ByPackage[pkg]; ok {
		maps.Copy(existing.Container, globals.Container)
		maps.Copy(existing.Rewritten, globals.Rewritten)
	} else {
		gi.ByPackage[pkg] = &PackageGlobalInfo{
			Container:   maps.Clone(globals.Container),
			ShouldShare: maps.Clone(globals.ShouldShare),
			Rewritten:   maps.Clone(globals.Rewritten),
		}
	}
}

// exported for gob (?)
type TranslatePackageResult struct {
	PackageName     string
	PackagePath     string
	CollectedTests  []PackageSelector
	GlobalInfo      *PackageGlobalInfo
	TranslatedFiles map[string][]byte
}

func filterTestFiles(files []*ast.File, fset *token.FileSet) []*ast.File {
	var filtered []*ast.File
	for _, file := range files {
		fullPath := fset.File(file.Package).Name()
		if strings.HasSuffix(filepath.Base(fullPath), "_test.go") {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

// renameFile removes any implied build constraints from filenames that might
// not be satisfied.
//
// Translate always translates with GOOS set to linux, while the translated code
// might run on GOOS set to another OS like darwin. The filename can imply a GOOS
// constraint (see https://pkg.go.dev/cmd/go#hdr-Build_constraints). If that is
// the case, neuter the constraint by adding a "_" to the name. We no longer
// need the constraint because all translated code is compiled together.
func renameFile(cfg gosimtool.BuildConfig, filePath string) string {
	dotIdx := strings.LastIndex(filePath, ".")
	withoutDot := filePath[:dotIdx]
	ext := filePath[dotIdx:]

	parts := strings.Split(withoutDot, "_")
	if len(parts) >= 2 && parts[len(parts)-1] == "test" {
		parts = parts[:len(parts)-1]
	}

	matched := false
	if len(parts) >= 2 && parts[len(parts)-2] == cfg.Identity.SimulationOS && parts[len(parts)-1] == cfg.Identity.SimulationArch {
		matched = true
	}
	if len(parts) >= 1 && parts[len(parts)-1] == cfg.Identity.SimulationOS {
		matched = true
	}

	if !matched {
		return filePath
	}

	// for now, after translate all files should be included in the build
	// modify the filename minimally to neuter any excluded files
	if strings.HasSuffix(withoutDot, "_test") {
		return strings.TrimSuffix(withoutDot, "_test") + "__test" + ext
	}
	return withoutDot + "_" + ext
}

func pathForFile(pkg, file string) string {
	if !strings.HasPrefix(pkg, "translated/") {
		panic("bad path")
	}
	pkg = strings.TrimPrefix(pkg, "translated/")
	pkg = strings.TrimSuffix(pkg, "_test")
	return filepath.Join(pkg, file)
}

func translatePackage(args *translatePackageArgs) (result *TranslatePackageResult, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			result = nil
			err = fmt.Errorf("translate %s: %v\n%s", args.pkg.ID, failure, debug.Stack())
		}
	}()
	var policies *PackagePolicies
	if !args.legacy {
		resolved, err := Policies(args.Selection)
		if err != nil {
			return nil, err
		}
		policies = &resolved
	}

	kind, extractedPath := classifyPackage(args.pkg)

	writer := newOutputWriter()

	// build meta-test
	if kind == PackageKindTestBinary {
		var collectedTests []PackageSelector
		var packageName string

		for _, dep := range args.pkg.Imports {
			kind2, path2 := classifyPackage(dep)
			if path2 == extractedPath && (kind2 == PackageKindTests || kind2 == PackageKindForTest) {
				results := args.importResults[dep.ID]
				if results != nil && results.CollectedTests != nil {
					collectedTests = append(collectedTests, results.CollectedTests...)
				}
				if results != nil && results.PackageName != "" {
					packageName = results.PackageName
				}
			}
		}

		outputPackage := args.replacedPkgs[extractedPath]

		// XXX: help
		t := &packageTranslator{
			replacedPkgs: args.replacedPkgs,
			profile:      policies,
		}

		file, err := t.makeMetaTestFile(packageName, collectedTests, args.hooksPackage) // XXX get right pkg name please....
		if err != nil {
			return nil, err
		}
		bytes, err := dstFileToBytes(file, args.packageNames, nil, args.replacedPkgs[extractedPath]+"_test")
		if err != nil {
			return nil, err
		}
		if err := writer.stage(pathForFile(outputPackage, "gosim_meta_test.go"), bytes); err != nil {
			return nil, err
		}
		return &TranslatePackageResult{TranslatedFiles: writer.extract()}, nil
	}

	if kind != PackageKindBase && kind != PackageKindForTest && kind != PackageKindTests {
		panic("help")
	}

	allGlobals := &GlobalInfo{
		ByPackage: make(map[string]*PackageGlobalInfo),
	}
	for _, prev := range args.importResults {
		if prev != nil && prev.GlobalInfo != nil {
			allGlobals.merge(prev.PackagePath, prev.GlobalInfo)
		}
	}

	if args.pkgWithTypesAndAst == nil {
		return nil, fmt.Errorf("could not load package %s", args.pkg.ID)
	}
	args.pkg = args.pkgWithTypesAndAst
	if policies != nil {
		if err := ValidateBoundary(args.pkg, *policies); err != nil {
			return nil, err
		}
	}

	pkgFiles := args.pkg.Syntax
	if kind == PackageKindForTest {
		// The PackageKindForTest build for package "foo" includes both _test.go
		// files with "package foo" (non-test) and normal .go files with
		// "package foo". The normal files are already translated with
		// PackageKindBase, so skip them here. Translating again would duplicate
		// the globals.
		pkgFiles = filterTestFiles(pkgFiles, args.pkg.Fset)
	}

	dec := decorator.NewDecorator(args.pkg.Fset)
	dec.Resolver = gotypes.New(args.pkg.TypesInfo.Uses)
	dec.Path = args.pkg.PkgPath

	var dstFiles []*dst.File
	dstFilePaths := make(map[*dst.File]string)

	collectedAliases := make(map[string]string)

	// translate package files
	for _, file := range pkgFiles {
		filePath := args.pkg.Fset.File(file.Package).Name()
		dstFile, err := dec.DecorateFile(file)
		if err != nil {
			return nil, fmt.Errorf("decorate %s: %w", filePath, err)
		}
		dstFiles = append(dstFiles, dstFile)
		dstFilePaths[dstFile] = filePath

		for _, imp := range dstFile.Imports {
			if imp.Name != nil && imp.Name.Name != "_" && imp.Name.Name != "." {
				if p, err := strconv.Unquote(imp.Path.Value); err == nil {
					collectedAliases[args.replacedPkgs[p]] = imp.Name.Name
				}
			}
		}
	}

	testFuncs := make(map[string]bool)

	var results TranslatePackageResult

	shared := globalsDontTranslateGo123
	if policies != nil {
		shared = policies.shared
	}
	globals := collectGlobalsAndTests(dstFiles, dec.Ast, args.pkg.PkgPath, args.pkg.TypesInfo, shared)
	globalInfo := &PackageGlobalInfo{
		Container:   make(map[string]string),
		ShouldShare: make(map[string]bool),
		Rewritten:   make(map[string]string),
	}

	// if args.pkg.PkgPath == "net/http" {
	// log.Println(args.pkg.PkgPath, collectGlobalsSSA(args.pkg))
	ssaGlobals := ssaGlobalsInfo{}
	if args.legacy {
		ssaGlobals = collectGlobalsSSA(args.pkg)
	}
	// }

	globalField := map[packageKind]string{
		// TODO: share these constants with the global file code
		PackageKindBase:    "G",
		PackageKindTests:   "G",
		PackageKindForTest: "GForTest",
	}[kind]
	for _, global := range globals.globalsFields {
		if _, ok := globalInfo.Container[global]; ok {
			return nil, fmt.Errorf("duplicate global %s.%s", args.pkg.PkgPath, global)
		}
		if !globals.shouldShare[global] {
			globalInfo.Container[global] = globalField
		}
	}
	for global, val := range globals.shouldShare {
		globalInfo.ShouldShare[global] = val
	}
	for _, info := range ssaGlobals.readonlyMaps {
		globalInfo.ShouldShare[info.name] = true
		delete(globalInfo.Container, info.name)
		if info.onceName != "" && info.onceFn != "" {
			globalInfo.ShouldShare[info.onceName] = true
			delete(globalInfo.Container, info.onceName)
		}
	}

	if kind == PackageKindBase || kind == PackageKindForTest {
		results.PackageName = args.pkg.Name
	}
	if kind == PackageKindTests {
		results.PackageName = strings.TrimSuffix(args.pkg.Name, "_test")
	}
	if kind == PackageKindForTest || kind == PackageKindTests {
		for _, test := range globals.tests {
			testFuncs[test] = true
			results.CollectedTests = append(results.CollectedTests, PackageSelector{Pkg: args.pkg.PkgPath, Selector: test})
			globalInfo.Rewritten[test] = "Impl" + test
		}
	}

	outputPackage := args.replacedPkgs[args.pkg.PkgPath]

	// handle standard library vendored packages
	localReplacedPkgs := maps.Clone(args.replacedPkgs)
	for path, dep := range args.pkg.Imports {
		if path != dep.PkgPath {
			if args.pkg.Module != nil {
				return nil, fmt.Errorf("non-stdlib package has vendored import %s", path)
			}
			if dep.Module != nil {
				return nil, fmt.Errorf("non-stdlib vendored dependency %s", dep.PkgPath)
			}
			// XXX: check if this should be rewritten at all?
			if _, ok := localReplacedPkgs[dep.PkgPath]; ok {
				localReplacedPkgs[path] = localReplacedPkgs[dep.PkgPath]
			}
		}
	}

	forTest := kind == PackageKindForTest

	allGlobals.merge(args.pkg.PkgPath, globalInfo)

	hooks := maps.Clone(hooksGo123)
	maps.Copy(hooks, hooksGensyscallGo123ByArch[args.cfg.Identity.SimulationArch])

	translator := &packageTranslator{
		profile:             policies,
		typesInfo:           args.pkg.TypesInfo,
		astMap:              dec.Map.Ast,
		dstMap:              dec.Map.Dst,
		replacedPkgs:        localReplacedPkgs,
		pkgPath:             args.pkg.PkgPath,
		implicitConversions: buildImplicitConversions(args.pkg),
		globalInfo:          allGlobals,
		testFuncs:           testFuncs,
		forTest:             forTest,
		collect: translateCollect{
			bindspecs: make(map[bindspec]struct{}),
			maps:      ssaGlobals.readonlyMaps,
		},
		hooks:             hooks,
		acceptedLinknames: acceptedgo123Linknames,
		keepAsmPkgs:       keepAsmPackagesGo123,
	}

	if policies != nil {
		translator.hooks = policies.hooks
		translator.acceptedLinknames = nil
		translator.keepAsmPkgs = nil
	}
	// translate package files
	for _, dstFile := range dstFiles {
		filePath := dstFilePaths[dstFile]
		file, err := translator.translateFile(dstFile)
		if err != nil {
			return nil, fmt.Errorf("translate %s: %w", filePath, err)
		}
		bytes, err := dstFileToBytes(file, args.packageNames, nil, "main")
		if err != nil {
			return nil, err
		}
		if err := writer.stage(pathForFile(outputPackage, renameFile(args.cfg, filepath.Base(filePath))), bytes); err != nil {
			return nil, err
		}
	}

	if globalsName, ok := map[packageKind]string{
		PackageKindBase:    "gosim_globals.go",
		PackageKindForTest: "gosim_globals_for_test.go",
		PackageKindTests:   "gosim_globals_test.go",
	}[kind]; ok {
		// write a globals.go, if applicable
		if len(translator.collect.globalFields) > 0 || len(translator.collect.inits) > 0 || len(translator.collect.bindspecs) > 0 || len(translator.collect.sharedGlobalFields) > 0 || len(translator.collect.maps) > 0 {
			file, err := makeDetgoGlobalsFile(translator, args.pkg.Name, forTest)
			if err != nil {
				return nil, err
			}
			bytes, err := dstFileToBytes(file, args.packageNames, collectedAliases, "main")
			if err != nil {
				return nil, err
			}
			if err := writer.stage(pathForFile(outputPackage, globalsName), bytes); err != nil {
				return nil, err
			}
		}
	}

	if kind == PackageKindBase && translator.keepAsmPkgs[args.pkg.PkgPath] {
		// copy over assembly implementations for specific packages
		for _, filePath := range args.pkg.OtherFiles {
			slog.Debug("copying other file", "pkg", args.pkg.PkgPath, "file", filePath)
			bytes, err := os.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			// prepend our header so cleanup will delete it,
			// even though we do not modify the file
			prefixed := append([]byte(header), bytes...)
			if err := writer.stage(pathForFile(outputPackage, filepath.Base(filePath)), prefixed); err != nil {
				return nil, err
			}
		}
	}

	// write a gosimaliashack.go file if needed
	if kind == PackageKindBase {
		// XXX: compute these automatically?
		if vars, ok := PublicExportHacks[args.pkg.PkgPath]; ok {
			file, err := makePublicExportHackFile(path.Base(outputPackage), vars)
			if err != nil {
				return nil, err
			}
			bytes, err := dstFileToBytes(file, args.packageNames, collectedAliases, "main")
			if err != nil {
				return nil, err
			}
			if err := writer.stage(pathForFile(outputPackage, "gosimaliashack.go"), bytes); err != nil {
				return nil, err
			}
		}
	}

	results.GlobalInfo = globalInfo
	results.PackagePath = args.pkg.PkgPath
	results.TranslatedFiles = writer.extract()

	return &results, nil
}

type translateCollect struct {
	globalFields       []*dst.Field
	sharedGlobalFields []dst.Spec
	initIdx            int
	inits              []string
	bindspecs          map[bindspec]struct{}
	maps               []mapInitializer
}

type PackageSelector struct {
	Pkg      string
	Selector string
}

type packageTranslator struct {
	profile       *PackagePolicies
	typesInfo     *types.Info
	suffixCounter int
	astMap        decorator.AstMap
	dstMap        decorator.DstMap
	replacedPkgs  map[string]string
	pkgPath       string

	hooks             map[PackageSelector]PackageSelector
	acceptedLinknames map[PackageSelector]PackageSelector
	keepAsmPkgs       map[string]bool

	collect             translateCollect
	implicitConversions map[ast.Expr]types.Type

	globalInfo *GlobalInfo

	needsUnsafe bool

	forTest bool

	testFuncs map[string]bool
}

func (t *packageTranslator) isBuiltIn(expr dst.Expr) bool {
	return t.typesInfo.Types[t.astMap.Nodes[expr].(ast.Expr)].IsBuiltin()
}

func (t *packageTranslator) isNamedBuiltIn(expr dst.Expr, name string) bool {
	if ident, ok := expr.(*dst.Ident); ok {
		return ident.Name == name && t.isBuiltIn(expr)
	}
	return false
}

func isDualAssign(c *dstutil.Cursor) (rhs *dst.Expr, isDualAssign bool) {
	// read from chan or map in (x, ok) form
	// "x, ok =" and "x, ok :="
	if assignStmt, ok := c.Node().(*dst.AssignStmt); ok && len(assignStmt.Lhs) == 2 && len(assignStmt.Rhs) == 1 {
		return &assignStmt.Rhs[0], true
	}
	// "var x, ok"
	if varSpec, ok := c.Node().(*dst.ValueSpec); ok && len(varSpec.Names) == 2 && len(varSpec.Values) == 1 {
		return &varSpec.Values[0], true
	}

	return nil, false
}

func (t *packageTranslator) suffix() string {
	suffix := fmt.Sprint(t.suffixCounter)
	t.suffixCounter++
	return suffix
}

func (t *packageTranslator) getType(expr dst.Expr) (types.Type, bool) {
	astExpr, ok := t.astMap.Nodes[expr].(ast.Expr)
	if !ok {
		// XXX hmmm??? why does this happen??
		return nil, false
	}
	if typ, ok := t.typesInfo.Types[astExpr]; ok {
		return typ.Type, true
	}
	return nil, false
}

func (t *packageTranslator) markSyncFuncsNorace(c *dstutil.Cursor) {
	node := c.Node()

	decl, ok := node.(*dst.FuncDecl)
	if !ok {
		return
	}

	if t.pkgPath == "sync" || t.pkgPath == "internal/sync" {
		// These packages implement synchronization using their own race hooks.
		// Instrumenting their mutex state also reports races between simulated
		// goroutines that the scheduler runs one at a time.
		decl.Decs.Start = append(decl.Decs.Start, "//go:norace")
	}
}

func (t *packageTranslator) newRuntimeSelector(name string) *dst.Ident {
	return &dst.Ident{
		Name: name,
		Path: t.replacedPkgs[gosimruntimePackage],
	}
}

func (t *packageTranslator) apply(node dst.Node) dst.Node {
	return dstutil.Apply(node, t.preApply, nil)
}

func (t *packageTranslator) preApply(c *dstutil.Cursor) bool {
	if f, ok := c.Node().(*dst.File); ok {
		for i, dec := range f.Decs.Start {
			if strings.HasPrefix(dec, "//go:build") {
				f.Decs.Start[i] = "//" // "//gosim filtered: " + dec
			}
			if strings.HasPrefix(dec, "// +build") {
				f.Decs.Start[i] = "//" // "//gosim filtered: " + dec
			}
		}
	}

	t.rewriteTestFunc(c)
	t.rewriteIdent(c)
	t.rewriteImport(c)
	t.rewriteNotifyListHack(c)
	t.rewriteJsonGlobalsHack(c)

	// TODO: think about and make this ordering less brittle

	// converisons, nil
	t.rewriteMapImplicitConversion(c)
	t.rewriteMapNil(c)
	t.rewriteMapLiteral(c)
	t.rewriteChanImplicitConversion(c)
	t.rewriteChanNil(c)

	// make
	t.rewriteMakeMap(c)
	t.rewriteMakeChan(c)

	// type
	t.rewriteMapType(c)
	t.rewriteChanType(c)

	// ops
	t.rewriteMapAssign(c)
	t.rewriteMapClear(c)
	t.rewriteMapDelete(c)
	t.rewriteMapGetOk(c)
	t.rewriteMapIndex(c)
	t.rewriteMapLen(c)
	t.rewriteChanCap(c)
	t.rewriteChanClose(c)
	t.rewriteChanLen(c)
	t.rewriteChanRange(c)
	t.rewriteChanRecvOk(c)
	t.rewriteChanRecvSimpleExpr(c)
	t.rewriteChanSend(c)

	t.rewriteGlobalDef(c)
	t.rewriteInit(c)
	t.markSyncFuncsNorace(c)
	t.rewriteStdlibEmptyAndLinkname(c)
	t.rewriteDanglingLinknames(c)
	t.rewriteGo(c)
	t.rewriteGlobalRead(c) // XXX: after rewriteGO?? think abou

	return true
}

func (t *packageTranslator) translateFile(dstFile *dst.File) (*dst.File, error) {
	// XXX: janky
	var keptPkgs []string
	for _, decl := range dstFile.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok {
			continue
		}
		if genDecl.Tok != token.IMPORT {
			continue
		}
		for _, spec := range genDecl.Specs {
			importSpec := spec.(*dst.ImportSpec)
			decs := importSpec.Decs.End.All()
			if len(decs) >= 1 && decs[0] == "//gosim:notranslate" {
				unquoted, err := strconv.Unquote(importSpec.Path.Value)
				if err != nil {
					return nil, err
				}
				keptPkgs = append(keptPkgs, unquoted)
			}
		}
	}
	if len(keptPkgs) > 0 {
		old := t.replacedPkgs
		defer func() {
			t.replacedPkgs = old
		}()
		t.replacedPkgs = maps.Clone(t.replacedPkgs)
		for _, pkg := range keptPkgs {
			delete(t.replacedPkgs, pkg)
		}
	}

	t.needsUnsafe = false

	// in a first pass, rewrite all select stmts. unclear if this is necessary,
	// but trying to do it preApply didn't translate inner nested selects.
	dstFile = dstutil.Apply(dstFile, nil, func(c *dstutil.Cursor) bool {
		t.rewriteSelectStmt(c)
		t.rewriteMapRange(c)
		t.rewriteGlobalDef(c) // XXX: help. this is for var closedchan = make(chan struct{})
		return true
	}).(*dst.File)

	dstFile = t.apply(dstFile).(*dst.File)

	if t.needsUnsafe {
		var importDecl *dst.GenDecl

		for _, decl := range dstFile.Decls {
			genDecl, ok := decl.(*dst.GenDecl)
			if !ok {
				continue
			}

			if genDecl.Tok != token.IMPORT {
				continue
			}

			importDecl = genDecl
		}

		// TODO: do this earlier, messes up comments
		if t.needsUnsafe {
			if importDecl == nil {
				importDecl = &dst.GenDecl{
					Tok: token.IMPORT,
				}
				dstFile.Decls = append([]dst.Decl{importDecl}, dstFile.Decls...)
			}
			importDecl.Specs = append(importDecl.Specs,
				&dst.ImportSpec{
					Name: dst.NewIdent("_"),
					Path: &dst.BasicLit{Kind: token.STRING, Value: strconv.Quote("unsafe")},
				},
			)
		}
	}

	return dstFile, nil
}

func makePublicExportHackFile(pkgName string, names []string) (*dst.File, error) {
	slices.Sort(names)
	var specs []dst.Spec
	for _, name := range names {
		specs = append(specs, &dst.TypeSpec{
			Name:   dst.NewIdent("GosimPublicExportHack" + name),
			Assign: true,
			Type:   dst.NewIdent(name),
		})
	}

	dstFile := &dst.File{
		Name: dst.NewIdent(pkgName),
		Decls: []dst.Decl{
			&dst.GenDecl{
				Tok:   token.TYPE,
				Specs: specs,
			},
		},
	}

	return dstFile, nil
}
