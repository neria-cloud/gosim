package translate

import (
	"cmp"
	_ "embed"
	"flag"
	"go/token"
	"log"
	"os"
	"path"
	"runtime/pprof"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/modfile"
	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/translate/cache"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

var skippedPackagesGo123 = map[string]bool{
	"runtime": true, // XXX wait what
	"errors":  true,
	"reflect": true,
	// "strings": true,
	"strconv": true,
	// embed: true, // this will also have io.EOF problems... unless we can just use the original there please???
	// XXX: for all unconverted packages, figure out all references to converted packages, and have a plan.
	"embed":    true, // XXX can we link back to the original here somehow???
	"math":     true,
	"math/big": true,

	"unsafe": true,

	"runtime/coverage": true, // XXX for now
	"runtime/metrics":  true, // XXX for now
	"runtime/pprof":    true, // XXX for now

	// XXX: rewrite internal/cpu to golang.org/x/sys/cpu?
	"vendor/golang.org/x/sys/cpu": true, // XXX for now

	"unique": true, // XXX: yes

	// Legacy private-runtime dependency. The new profile rejects this package;
	// skipping translation does not make its runtime map access safe.
	"github.com/ugorji/go/codec": true,

	// TODO(migration): Task 21 replaces this legacy skip policy with reviewed
	// version profiles. Native crypto may access entropy, scheduling and process
	// state; these entries are not evidence of simulated crypto support.
	// TODO(not planned): General crypto/TLS translation repair is separate work.
	"crypto/internal/fips140":                    true,
	"crypto/internal/fips140/alias":              true,
	"crypto/internal/fips140/subtle":             true,
	"crypto/internal/fips140/sha256":             true,
	"crypto/internal/fips140/sha3":               true,
	"crypto/internal/fips140/sha512":             true,
	"crypto/internal/fips140/aes":                true,
	"crypto/internal/fips140/aes/gcm":            true,
	"crypto/internal/fips140/nistec":             true,
	"crypto/internal/fips140/nistec/fiat":        true,
	"crypto/internal/fips140/bigmod":             true,
	"crypto/internal/fips140/ecdsa":              true,
	"crypto/internal/fips140/edwards25519/field": true,
	"crypto/internal/fips140/check":              true,
	"crypto/internal/fips140/drbg":               true,
	"crypto/internal/fips140/hmac":               true,
	"crypto/internal/fips140/hkdf":               true,
	"crypto/internal/fips140/mlkem":              true,
	"crypto/internal/fips140/pbkdf2":             true,
	"crypto/internal/fips140/rsa":                true,
	"crypto/internal/fips140/tls12":              true,
	"crypto/internal/fips140/tls13":              true,
	"crypto/internal/fips140cache":               true,
	"crypto/internal/fips140hash":                true,
	"crypto/internal/fips140only":                true,
	"crypto/internal/fips140deps/godebug":        true,
	"crypto/internal/fips140deps/byteorder":      true,
	"crypto/internal/fips140deps/cpu":            true,
	"crypto/internal/sysrand":                    true,
	// Non-internal crypto packages that transitively import fips140 internals.
	// These legacy exclusions do not establish a safe native boundary.
	"crypto/aes":      true,
	"crypto/cipher":   true,
	"crypto/des":      true,
	"crypto/dsa":      true,
	"crypto/ecdh":     true,
	"crypto/ecdsa":    true,
	"crypto/ed25519":  true,
	"crypto/elliptic": true,
	"crypto/fips140":  true,
	"crypto/hkdf":     true,
	"crypto/hmac":     true,
	"crypto/md5":      true,
	"crypto/mlkem":    true,
	"crypto/pbkdf2":   true,
	"crypto/rand":     true,
	"crypto/rc4":      true,
	"crypto/rsa":      true,
	"crypto/sha1":     true,
	"crypto/sha256":   true,
	"crypto/sha3":     true,
	"crypto/sha512":   true,
	"crypto/subtle":   true,
	"crypto/tls":      true,
	// boring and nistec are also now covered by skipping the above
	"crypto/internal/boring/sig": true,
	"crypto/internal/nistec":     true,

	// Native HTTP bypasses simulated networking and crosses translated net/time
	// types. New profiles reject it explicitly; this is only the legacy path.
	// TODO(not planned): HTTP/TLS translation support is separate compatibility work.
	"net/http":          true,
	"net/http/httputil": true, // uses real net/http types
	// crypto/x509 uses real crypto types (rsa, ecdsa, etc.) which are skipped.
	// Translating it causes type mismatches with real crypto.Signer etc.
	"crypto/x509":      true,
	"crypto/x509/pkix": true,
	// golang.org/x/net/http2 uses real net/http types (req.Context() etc.)
	// which clash with translated/context.Context.
	"golang.org/x/net/http2": true,
	// selfcert uses crypto/x509 (skipped) with time.Time and net.IP;
	// translating it causes type mismatches with real crypto types.
	"github.com/glycerine/rpc25519/selfcert": true,
	// net/url uses //go:linkname setpath which causes issues when translated.
	// Skipping it means all packages (translated and skipped alike) use the
	// real net/url.URL type, avoiding mismatches with crypto/x509.URIs etc.
	"net/url": true,

	// internal/runtime/maps is the runtime's map implementation in Go 1.25+.
	// It has many 2-part //go:linkname directives pointing to the runtime, and
	// 3-part linknames that define runtime.mapaccess1 etc. Translating it causes
	// duplicate symbol errors at link time (both translated and real versions
	// define runtime.mapaccess1 etc.). Since gosim replaces map[K]V with
	// gosimruntime.Map[K,V] everywhere, translated code never calls into this
	// package directly anyway.
	"internal/runtime/maps": true,
	// hash/maphash is NOT skipped — it must be translated so its
	// runtime_rand linkname gets hooked to gosim's deterministic RNG.
	// The internal/runtime/maps import (used only for maps.Use64BitHash)
	// is redirected to internal/maps_shim via replacedPkgs.

	"testing":          true,
	"testing/synctest": true,
	// "internal/synctest" is replaced by gosim's stub (synctestPackage).
	// The stdlib internal/synctest cannot be imported outside stdlib, so we
	// skip translating it and redirect imports to our stub via replacedPkgs.
	"internal/synctest":         true,
	"testing/internal/testdeps": true,
	"internal/reflectlite":      true,
	// internal/race is replaced by gosim's version (racePackage) via replacedPkgs.
	// Go 1.26 changed stdlib internal/race to use body-less //go:linkname stubs
	// pushed from runtime, which break when translated. Gosim's version calls
	// runtime.Race* functions directly.
	"internal/race":               true,
	gosimruntimePackage:           true,
	gosimruntimePackage + "_test": true, // eh
	gosimruntimePackage + ".test": true, // eh
	runtimeABIPackage:             true,
	runtimeABIPackage + "_test":   true,
	runtimeABIPackage + ".test":   true,
	// reflectPackage: true,
}

// skippedPrefixesGo123 lists import-path prefixes for packages that should be
// treated as skipped (not translated). Any package whose path has one of these
// prefixes will be omitted from translation and used as-is.
//
// Native gRPC crosses context/net/time types and bypasses simulated networking.
// TODO(migration): Task 21 activates explicit profile rejection for applications.
// TODO(not planned): gRPC translation repair remains separate compatibility work.
var skippedPrefixesGo123 = []string{
	"google.golang.org/grpc",
	// Native QUIC likewise has no supported simulated transport boundary.
	"github.com/quic-go/quic-go",
	// golang.org/x/crypto sub-packages depend on skipped stdlib crypto/*
	// packages (hmac, cipher, etc.). Translating them causes type mismatches
	// between translated/hash.Hash and hash.Hash etc.
	"golang.org/x/crypto",
	// cloudflare/circl is a crypto library depending on skipped crypto packages.
	"github.com/cloudflare/circl",
	// zygomys uses reflect with maps; gosim's map replacement causes
	// translated/reflect.Type vs reflect.Type mismatches.
	"github.com/glycerine/zygomys",
	// blake3 has a variable/type name collision (var job *job) that
	// trips the translator. Pure hash computation, no gosim interaction.
	"github.com/glycerine/blake3",
}

var keepAsmPackagesGo123 = map[string]bool{
	"crypto/internal/bigmod":                       true,
	"crypto/internal/edwards25519/field":           true,
	"vendor/golang.org/x/crypto/chacha20":          true,
	"vendor/golang.org/x/crypto/internal/poly1305": true,
	"vendor/golang.org/x/crypto/chacha20poly1305":  true,
	"vendor/golang.org/x/crypto/sha3":              true,
	"hash/crc32":                                   true,

	"github.com/cespare/xxhash/v2": true,

	// klauspost/compress has assembly-backed functions (CPU detection,
	// Huffman, S2, Zstd, xxHash). Pure computation, no gosim interaction.
	"github.com/klauspost/compress/internal/cpuinfo":     true,
	"github.com/klauspost/compress/huff0":                true,
	"github.com/klauspost/compress/s2":                   true,
	"github.com/klauspost/compress/zstd":                 true,
	"github.com/klauspost/compress/zstd/internal/xxhash": true,
	"github.com/klauspost/cpuid/v2":                      true,

	// Other third-party packages with assembly-backed functions.
	// All pure computation (compression, hashing, CPU detection).
	"github.com/glycerine/blake3/guts":            true,
	"github.com/glycerine/rpc25519/bytes":         true,
	"github.com/minio/minlz":                      true,
	"github.com/pierrec/lz4/v4/internal/lz4block": true,
	"github.com/templexxx/cpu":                    true,

	// Non-vendored golang.org/x/* modules (vendored versions are separate entries above).
	"golang.org/x/crypto/argon2":            true,
	"golang.org/x/crypto/blake2b":           true,
	"golang.org/x/crypto/chacha20poly1305":  true,
	"golang.org/x/crypto/internal/poly1305": true,
	"golang.org/x/sys/cpu":                  true,

	// internal/runtime/sys contains pure compiler intrinsics (GetCallerPC,
	// GetCallerSP, GetClosurePtr) and architecture-specific helpers (EnableDIT,
	// DisableDIT). None interact with gosim scheduling, so pass them through.
	// This package became a direct dependency of crypto/subtle in Go 1.25.
	"internal/runtime/sys": true,

	// internal/runtime/syscall/linux contains Syscall6, which is
	// assembly-backed. Pass through as-is.
	"internal/runtime/syscall/linux": true,
}

var PublicExportHacks = map[string][]string{
	"encoding/binary":                  {"littleEndian", "bigEndian"},
	"internal/poll":                    {"errNetClosing"},
	"github.com/golang/protobuf/proto": {"enumsByName"},
}

// XXX: replace the package instead
var replacements = map[packageSelector]packageSelector{
	{Pkg: "runtime", Selector: "SetFinalizer"}: {Pkg: gosimruntimePackage, Selector: "SetFinalizer"},
	{Pkg: "runtime", Selector: "GOOS"}:         {Pkg: gosimruntimePackage, Selector: "GOOS"},
	{Pkg: "runtime", Selector: "Gosched"}:      {Pkg: gosimruntimePackage, Selector: "Yield"},
}

const gosimModPath = gosimtool.Module

const (
	gosimruntimePackage = gosimModPath + "/rt/runtime"
	runtimeABIPackage   = gosimruntimePackage + "/abi"
	hooksGo123Package   = gosimModPath + "/internal/hooks/go123"
	reflectPackage      = gosimModPath + "/internal/reflect"
	simulationPackage   = gosimModPath + "/internal/simulation"
	testingPackage      = gosimModPath + "/internal/testing"
	synctestPackage     = gosimModPath + "/internal/synctest"
	mapsShimPackage     = gosimModPath + "/internal/maps_shim"
	cryptorandPackage   = gosimModPath + "/internal/cryptorand"
	racePackage         = gosimModPath + "/internal/race"
)

var TranslatedRuntimePackages = []string{
	hooksGo123Package,
	reflectPackage,
	simulationPackage,
	testingPackage,
	synctestPackage,
	mapsShimPackage,
	cryptorandPackage,
}

const (
	// loadDepGraph fairly quickly loads the dependency graph
	loadDepGraph = packages.NeedName | packages.NeedFiles | packages.NeedModule | packages.NeedImports | packages.NeedDeps
	// loadSyntaxAndTypes relatively slowly loads detailed types and syntax information
	loadSyntaxAndTypes = packages.NeedSyntax | packages.NeedName | packages.NeedTypes |
		packages.NeedTypesInfo | packages.NeedFiles | packages.NeedImports
)

func loadPackages(patterns []string, b gosimtool.BuildConfig, mode packages.LoadMode, tests bool) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode:  mode,
		Tests: tests,
		Fset:  token.NewFileSet(),
	}

	// apply build config to packages config
	// TODO: build tags plan (rename files, force GOOS and GOARCH)
	tags := []string{"sim"}
	if b.Race {
		tags = append(tags, "race")
	}
	// TODO: if we support varying GOARCH here, we should select the arch-specific hooks at runtime...
	env := append(os.Environ(), "GOOS="+b.GOOS, "GOARCH="+b.GOARCH, "CGO_ENABLED=0")
	cfg.BuildFlags = []string{"-tags", strings.Join(tags, ",")}
	cfg.Env = env

	// load packages
	packages, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		log.Println(err)
		log.Fatal("failed to load packages... missing go.mod deps?")
	}

	// check for errors
	for _, pkg := range packages {
		if pkg.Errors != nil {
			// TODO: ignore errors in tests outside of what we care about?
			log.Println("errors for ", pkg.PkgPath)
			for _, err := range pkg.Errors {
				log.Println(err)
			}
			os.Exit(1)
		}
	}

	return packages, nil
}

// reloadUncachedPackages loads the types and AST for the given uncached package
// paths. It takes special care to load tests only where appropriate, because
// broken tests in dependencies are not uncommon.
//
// loadPackages has a binary flag for loading tests, which if enabled loads
// tests for the explicitly listed packages. It does not load tests for
// dependencies.
//
// When we reload packages here we explicitly list all packages we want to load,
// including dependencies whose tests we do not care about. To not load tests
// for those we make to calls to loadPackages, one with and one without tests.
func reloadUncachedPackages(listedPkgs []*packages.Package, uncachedPackages map[string]struct{}, cfg gosimtool.BuildConfig) map[string]*packages.Package {
	// determine packages we want to load tests for based on
	// the original command line arguments
	listedPkgPaths := make(map[string]struct{})
	for _, pkg := range listedPkgs {
		if fromGosim := slices.Contains(TranslatedRuntimePackages, pkg.PkgPath); fromGosim {
			continue
		}
		listedPkgPaths[pkg.PkgPath] = struct{}{}
	}

	// reloadByTests is packages to reload, with tests enabled or not
	reloadByTests := make(map[bool][]string)
	for path := range uncachedPackages {
		_, ok := listedPkgPaths[path]
		reloadByTests[ok] = append(reloadByTests[ok], path)
	}

	// reload packages in parallel
	var mu sync.Mutex
	reloadedByTests := make(map[bool][]*packages.Package)
	var g errgroup.Group
	for test, pkgs := range reloadByTests {
		g.Go(func() error {
			reloaded, err := loadPackages(pkgs, cfg, loadSyntaxAndTypes, test)
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			reloadedByTests[test] = reloaded
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}

	// merge results
	pkgsWithTypesAndAst := make(map[string]*packages.Package)
	for _, pkgs := range reloadedByTests {
		for _, pkg := range pkgs {
			pkgsWithTypesAndAst[pkg.ID] = pkg
		}
	}

	return pkgsWithTypesAndAst
}

type packageKind string

const (
	PackageKindTestBinary = "testbinary"
	PackageKindBase       = "base"
	PackageKindForTest    = "fortest"
	PackageKindTests      = "tests"
)

func classifyPackage(pkg *packages.Package) (packageKind, string) {
	switch {
	case pkg.Name == "main" && strings.HasSuffix(pkg.PkgPath, ".test"):
		// XXX: is this correct?
		return PackageKindTestBinary, strings.TrimSuffix(pkg.PkgPath, ".test")

	case strings.HasSuffix(pkg.Name, "_test"):
		return PackageKindTests, strings.TrimSuffix(pkg.PkgPath, "_test")

	case strings.HasSuffix(pkg.ID, ".test]"):
		return PackageKindForTest, pkg.PkgPath

	case !strings.HasSuffix(pkg.Name, "_test") && pkg.PkgPath == pkg.ID:
		return PackageKindBase, pkg.PkgPath

	default:
		log.Fatal("weird package", pkg.Name, pkg.PkgPath, pkg.ID)
		panic("unreachable")
	}
}

// isSkippedByPrefix returns true if pkgPath equals a prefix exactly, or starts
// with prefix + "/". This avoids false positives from module names that share
// a common start (e.g. "grpc" vs "grpc-gateway").
func isSkippedByPrefix(pkgPath string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if pkgPath == prefix || strings.HasPrefix(pkgPath, prefix+"/") {
			return true
		}
	}
	return false
}

// collectImports collects all packages reachable from roots, excluding those
// in skip (exact match) and skipPrefixes (prefix match). Pass nil skipPrefixes
// to collect all packages including those matching skippedPrefixesGo123.
func collectImports(roots []*packages.Package, skip map[string]bool, skipPrefixes []string) []*packages.Package {
	seen := make(map[*packages.Package]bool)
	var order []*packages.Package
	var visit func(pkg *packages.Package)
	visit = func(pkg *packages.Package) {
		if skip[pkg.PkgPath] || seen[pkg] || isSkippedByPrefix(pkg.PkgPath, skipPrefixes) {
			return
		}
		seen[pkg] = true
		order = append(order, pkg)
		for _, dep := range pkg.Imports {
			visit(dep)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	slices.SortFunc(order, func(a, b *packages.Package) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return order
}

func writeGoModFile(modDir string, modFile *modfile.File, writer *outputWriter) {
	origModPath := modFile.Module.Mod.Path
	isGosim := origModPath == gosimModPath
	// take the existing go.mod and make it work for a sub-directory containing
	// a module translated
	if err := modFile.AddModuleStmt("translated"); err != nil {
		log.Fatal(err)
	}

	if isGosim {
		// special case running translate in the gosim module
		if err := modFile.AddRequire(gosimModPath, "v0.0.0"); err != nil {
			log.Fatal(err)
		}
		if err := modFile.AddReplace(gosimModPath, "", "../../../", ""); err != nil {
			log.Fatal(err)
		}
	} else {
		// adjust relative paths for the new module's location
		for _, replace := range modFile.Replace {
			// > Third, filesystem paths found in "replace" directives are
			// represented by a path with an empty version.
			if replace.New.Version == "" && !path.IsAbs(replace.New.Path) {
				// The output module is located three directories deeper than the old module.
				newPath := path.Join("../../../", replace.New.Path)
				if err := modFile.AddReplace(replace.Old.Path, replace.Old.Version, newPath, ""); err != nil {
					log.Fatal(err)
				}
			}
		}

		// Make the original module available so that skipped sub-packages
		// (which remain as untranslated imports) can be resolved.
		// Added after the adjust loop to avoid double-adjusting the path.
		if err := modFile.AddRequire(origModPath, "v0.0.0"); err != nil {
			log.Fatal(err)
		}
		if err := modFile.AddReplace(origModPath, "", "../../../", ""); err != nil {
			log.Fatal(err)
		}
	}

	bytes, err := modFile.Format()
	if err != nil {
		log.Fatal(err)
	}
	if err := writer.stage("go.mod", bytes); err != nil {
		log.Fatal(err)
	}
	goSumBytes, err := os.ReadFile(path.Join(modDir, "go.sum"))
	if err != nil {
		log.Fatal(err)
	}
	if err := writer.stage("go.sum", goSumBytes); err != nil {
		log.Fatal(err)
	}
}

type TranslateInput struct {
	Packages []string
	Cfg      gosimtool.BuildConfig
}

func Translate(input *TranslateInput) (*gosimtool.TranslateOutput, error) {
	// XXX: dedup?
	modDir, err := gosimtool.FindGoModDir()
	if err != nil {
		log.Fatal(err)
	}
	rootOutputDir := path.Join(modDir, gosimtool.OutputDirectory, "translated", input.Cfg.AsDirname())

	cacheDir := path.Join(modDir, gosimtool.OutputDirectory)
	if override := os.Getenv("GOSIMCACHE"); override != "" {
		cacheDir = override
	}

	cachePath := path.Join(cacheDir, "cache.sqlite3")
	if err := os.MkdirAll(path.Dir(cachePath), 0o755); err != nil {
		log.Fatal(err)
	}
	db, err := cache.NewDB(cachePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	cache := cache.NewCache(db)
	defer cache.Clean()
	// XXX: cap size at something reasonable? 100x working set?

	if err := os.MkdirAll(rootOutputDir, 0o755); err != nil {
		log.Fatal(err)
	}
	return translatePackages(cache, input.Packages, rootOutputDir, input.Cfg)
}

func buildReplacePackagesAndPackageNames(convertPkgs, allPkgs []*packages.Package) (replacedPkgs map[string]string, packageNames map[string]string) {
	packageNames = make(map[string]string)
	packageNames["golang.org/x/sys/cpu"] = "cpu" // XXX: here because we replace the vendored one with the common one
	for _, pkg := range allPkgs {
		packageNames[pkg.PkgPath] = pkg.Name
	}

	replacedPkgs = make(map[string]string)
	for _, pkg := range convertPkgs {
		inputPackage := pkg.PkgPath
		outputPackage := "translated/" + gosimtool.ReplaceSpecialPackages(pkg.PkgPath)
		replacedPkgs[inputPackage] = outputPackage
		packageNames[outputPackage] = packageNames[inputPackage]
	}

	// override reflect, testing, synctest, and shim packages
	replacedPkgs["reflect"] = replacedPkgs[reflectPackage]
	replacedPkgs["internal/reflectlite"] = replacedPkgs[reflectPackage]
	replacedPkgs["testing"] = replacedPkgs[testingPackage]
	replacedPkgs["internal/synctest"] = replacedPkgs[synctestPackage]
	// Redirect internal/runtime/maps to our shim (only provides Use64BitHash).
	// hash/maphash is the only translated package that imports this.
	replacedPkgs["internal/runtime/maps"] = replacedPkgs[mapsShimPackage]
	// Redirect crypto/rand to our deterministic implementation.
	// The real crypto/rand is skipped (fips140 dep chain) but user code
	// that imports crypto/rand should get deterministic random bytes.
	replacedPkgs["crypto/rand"] = replacedPkgs[cryptorandPackage]
	// Redirect stdlib internal/race to gosim's internal/race.
	// Go 1.26 added 1-part //go:linkname directives to internal/race that
	// break when translated. Gosim's version calls runtime.Race* directly.
	replacedPkgs["internal/race"] = replacedPkgs[racePackage]

	// handle the linkname in the os package
	replacedPkgs["net"] = "translated/" + gosimtool.ReplaceSpecialPackages("net")

	// not replaced, but need to know for rewrites
	replacedPkgs[gosimruntimePackage] = gosimruntimePackage
	replacedPkgs[runtimeABIPackage] = runtimeABIPackage
	return
}

func checkGosimDep(modFile *modfile.File) {
	// work in the gosim module
	if isGosim := modFile.Module.Mod.Path == gosimModPath; isGosim {
		return
	}
	for _, req := range modFile.Require {
		// work if there is an explicit dependency
		if req.Mod.Path == gosimModPath {
			return
		}
	}
	// complain otherwise
	log.Fatalf("current module does not depend on %v, try running init", gosimModPath)
}

func checkSingleModule(modPath string, pkgs []*packages.Package) {
	for _, pkg := range pkgs {
		// allow packages from the current module
		if pkg.Module.GoMod == modPath {
			continue
		}
		// allow packages explicitly added by translate
		_, path := classifyPackage(pkg)
		if slices.Contains(TranslatedRuntimePackages, path) {
			continue
		}
		// complain about others
		log.Fatal("packages from outside module modules ", pkg.PkgPath, " ", modPath)
	}
}

func translatePackages(cache *cache.Cache, listPatterns []string, rootOutputDir string, cfg gosimtool.BuildConfig) (*gosimtool.TranslateOutput, error) {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	listPatterns = append(listPatterns, TranslatedRuntimePackages...)

	listedPkgs, err := loadPackages(listPatterns, cfg, loadDepGraph, true)
	if err != nil {
		log.Fatal(err)
	}
	if len(listedPkgs) == 0 {
		log.Fatal("no packages")
	}

	modPath, modFile, err := gosimtool.FindGoMod()
	if err != nil {
		log.Fatal(err)
	}
	modDir := path.Dir(modPath)

	checkGosimDep(modFile)
	checkSingleModule(modPath, listedPkgs)

	allPkgs := collectImports(listedPkgs, nil, nil)
	convertPkgs := collectImports(listedPkgs, skippedPackagesGo123, skippedPrefixesGo123)

	packageGraph := newDepGraph()
	basePkgs := make(map[string]*packages.Package)
	pkgById := make(map[string]*packages.Package)
	for _, pkg := range convertPkgs {
		packageGraph.addNode(pkg.ID)
		pkgById[pkg.ID] = pkg

		if kind, path := classifyPackage(pkg); kind == PackageKindBase {
			basePkgs[path] = pkg
		}
	}

	for _, pkg := range convertPkgs {
		for _, dep := range pkg.Imports {
			if _, ok := packageGraph.nodes[dep.ID]; !ok {
				// XXX?
				continue
			}
			packageGraph.addDep(pkg.ID, dep.ID)
		}

		// XXX: add a package from the "for test" to the "main" package
		kind, path := classifyPackage(pkg)
		if kind == PackageKindForTest || kind == PackageKindTests {
			if basePkgs[path] != nil {
				packageGraph.addDep(pkg.ID, basePkgs[path].ID)
			} else {
				log.Println("huh", kind, pkg.PkgPath, path)
				log.Fatal(":(")
			}
		}
	}

	translateToolHash := computeTranslateToolHash(cfg)

	numWorkers := 32

	packageHashes := make(map[string]Hash)

	buildInParallel(packageGraph, numWorkers, packageHashes, func(pkgId string, importHashes map[string]Hash) Hash {
		return computePackageHash(translateToolHash, pkgById[pkgId], importHashes)
	})

	allResults := make(map[string]*TranslatePackageResult)
	cacheHits := make(map[Hash]bool)

	uncachedPackages := make(map[string]struct{})
	for pkgId, hash := range packageHashes {
		pkg := pkgById[pkgId]

		res, err := cacheGet(cache, hash)
		if err != nil {
			log.Fatal(err)
		}
		if res != nil {
			allResults[pkgId] = res
			cacheHits[hash] = true // record cache hits so we don't update the cache for them
		} else {
			uncachedPackages[strings.TrimSuffix(pkg.PkgPath, "_test")] = struct{}{}
		}
	}

	pkgsWithTypesAndAst := reloadUncachedPackages(listedPkgs, uncachedPackages, cfg)

	replacedPkgs, packageNames := buildReplacePackagesAndPackageNames(convertPkgs, allPkgs)

	buildInParallel(packageGraph, numWorkers, allResults, func(pkgId string, localResults map[string]*TranslatePackageResult) *TranslatePackageResult {
		return translatePackage(&translatePackageArgs{
			cfg:                cfg,
			pkg:                pkgById[pkgId],
			replacedPkgs:       replacedPkgs,
			hooksPackage:       hooksGo123Package,
			packageNames:       packageNames,
			importResults:      localResults,
			pkgWithTypesAndAst: pkgsWithTypesAndAst[pkgId],
		})
	})

	for pkgId, res := range allResults {
		hash := packageHashes[pkgId]
		if !cacheHits[hash] {
			if err := cachePut(cache, hash, res); err != nil {
				log.Fatal(err)
			}
		}
	}

	writer := newOutputWriter()
	for _, res := range allResults {
		if err := writer.merge(res.TranslatedFiles); err != nil {
			log.Fatal(err)
		}
	}

	writeGoModFile(modDir, modFile, writer)

	if err := writer.writeFiles(rootOutputDir); err != nil {
		log.Fatal(err)
	}
	if err := writer.maybeDeleteGeneratedFiles(rootOutputDir); err != nil {
		log.Fatal(err)
	}

	var out []string
	for _, pkg := range listedPkgs {
		kind, _ := classifyPackage(pkg)
		if kind != PackageKindBase {
			continue
		}
		if fromGosim := slices.Contains(TranslatedRuntimePackages, pkg.PkgPath); fromGosim {
			continue
		}
		out = append(out, replacedPkgs[pkg.PkgPath])
	}

	deps := make(map[string]map[string]time.Time)
	modTimeCache := make(map[string]time.Time)
	for _, pkg := range listedPkgs {
		kind, path := classifyPackage(pkg)
		if kind != PackageKindTestBinary {
			continue
		}

		// TODO: also add deps from the mandatory linked packages?
		files := findAllDepFiles(pkg, modDir)
		times, err := loadModTimes(files, modTimeCache)
		if err != nil {
			log.Fatal(err)
		}
		deps[replacedPkgs[path]] = times
	}

	return &gosimtool.TranslateOutput{
		RootOutputDir: rootOutputDir,
		Packages:      out,
		Deps:          deps,
	}, nil
}

func findAllDepFiles(pkg *packages.Package, root string) []string {
	seen := make(map[*packages.Package]struct{})
	var walk func(*packages.Package)
	var files []string
	walk = func(pkg *packages.Package) {
		if _, ok := seen[pkg]; ok {
			return
		}
		seen[pkg] = struct{}{}
		for _, dep := range pkg.Imports {
			walk(dep)
		}
		for _, file := range pkg.GoFiles {
			if strings.HasPrefix(file, root) {
				files = append(files, file)
			}
		}
	}
	walk(pkg)
	slices.Sort(files)
	return files
}

func loadModTimes(files []string, cache map[string]time.Time) (map[string]time.Time, error) {
	result := make(map[string]time.Time)
	for _, file := range files {
		if t, ok := cache[file]; ok {
			result[file] = t
			continue
		}

		info, err := os.Stat(file)
		if err != nil {
			return nil, err
		}
		t := info.ModTime()
		cache[file] = t
		result[file] = t
	}
	return result, nil
}

// Go version?
// - GOTOOLCHAIN aware?

// Test perf?
// - only run once for all testdata?

// Cmd
// - nicer flags for gosim test

// TODO: hash more singletons?
// - env vars? anything influencing go packages?
// - use go packages export file hash?

// next:
// - verify determinism
// - deal with logging output

// log progress?
// log.Println(results.pkgID, hex.EncodeToString(results.inputHash[:]))
