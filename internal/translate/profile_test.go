package translate

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"runtime"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/selected"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

func profileSelection(t *testing.T, features ...string) buildspec.Selection {
	t.Helper()
	p := selected.Profile()
	patch := runtime.Version()
	d := strings.Repeat("a", 64)
	id := buildspec.Identity{
		Schema: 2, Toolchain: patch, Profile: p.ID, AdapterRevision: p.Revision,
		ToolchainDigest: d, StdlibDigest: d, ContractDigest: d, RuntimeRevision: d, HostOS: runtime.GOOS, HostArch: runtime.GOARCH,
		SimulationOS: "linux", SimulationArch: runtime.GOARCH, Features: features, Tags: []string{"linkname", "sim"},
		ModuleGraphDigest: d, TranslatorDigest: d, DebugPolicyDigest: d,
	}
	for _, feature := range features {
		if feature != "rt" {
			id.ProviderRevisions = append(id.ProviderRevisions, buildspec.Revision{Name: feature, Digest: d})
		}
	}
	id.ContractDigest, _ = p.ContractDigest()
	s, err := p.Select(id)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPoliciesSelectedMetadata(t *testing.T) {
	t.Parallel()
	for _, features := range [][]string{{"rt"}, {"rt", "fs"}, {"rt", "net"}, {"rt", "fs", "net"}} {
		t.Run(strings.Join(features, ","), func(t *testing.T) {
			s := profileSelection(t, features...)
			policies, err := Policies(s)
			if err != nil {
				t.Fatal(err)
			}
			family := s.Identity.Profile
			if policies.packages["reflect"] != gosimtool.Module+"/internal/compat/"+family+"/shims/reflect" {
				t.Fatal("wrong reflect substitution")
			}
			for _, hook := range policies.hooks {
				if !strings.HasPrefix(hook.Pkg, gosimtool.Module+"/internal/compat/"+family+"/hooks/") {
					t.Fatalf("hook escapes family: %+v", hook)
				}
			}
			if !policies.shared[PackageSelector{Pkg: gosimtool.Module + "/internal/control", Selector: "binding"}] {
				t.Fatal("transport binding is not shared")
			}
			for _, feature := range []string{"fs", "net"} {
				_, err := policies.packagePolicy(gosimtool.Module+"/internal/"+feature+"/model", false)
				if (err == nil) != slices.Contains(features, feature) {
					t.Fatalf("provider %s membership: %v", feature, err)
				}
			}
			s.Identity.Features[0] = "mutated"
			s.RuntimePackages[0] = "mutated"
			if policies.features[0] != "rt" || policies.profile.RuntimePackages[0] == "mutated" {
				t.Fatal("policies retain caller-owned selection")
			}
		})
	}
}

func TestPoliciesRejectForgedSelection(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]func(*buildspec.Selection){
		"empty":   func(s *buildspec.Selection) { *s = buildspec.Selection{} },
		"profile": func(s *buildspec.Selection) { s.Identity.Profile = "go124" },
		"other-family": func(s *buildspec.Selection) {
			if s.Identity.Profile == "go126" {
				s.Identity.Profile = "go125"
				s.Identity.Toolchain = "go1.25.14"
			} else {
				s.Identity.Profile = "go126"
				s.Identity.Toolchain = "go1.26.9"
			}
		},
		"unsupported-family": func(s *buildspec.Selection) { s.Identity.Toolchain = "go1.27.0" },
		"arch":               func(s *buildspec.Selection) { s.Identity.SimulationArch = "386" },
		"revision":           func(s *buildspec.Selection) { s.Identity.AdapterRevision = strings.Repeat("b", 64) },
		"hooks":              func(s *buildspec.Selection) { s.HookManifestDigest = strings.Repeat("b", 64) },
		"missing-runtime":    func(s *buildspec.Selection) { s.RuntimePackages = s.RuntimePackages[1:] },
		"additional-runtime": func(s *buildspec.Selection) { s.RuntimePackages = append(s.RuntimePackages, "net/http") },
		"provider":           func(s *buildspec.Selection) { s.ProviderImports = []string{gosimtool.Module + "/internal/fs"} },
		"features":           func(s *buildspec.Selection) { s.Identity.Features = []string{"rt", "net", "fs"} },
		"legacy-identity":    func(s *buildspec.Selection) { s.Identity.Schema = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			s := profileSelection(t, "rt")
			change(&s)
			if _, err := Policies(s); err == nil {
				t.Fatal("forged selection accepted")
			}
		})
	}
}

// These are actual parsed/type-checked programs. Native go/types identities here
// describe input source; the check decides which values may cross after rewriting.
func typedBoundary(t *testing.T, path, source string, standard bool) *packages.Package {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "boundary.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Uses: make(map[*ast.Ident]types.Object), Defs: make(map[*ast.Ident]types.Object), Implicits: make(map[ast.Node]types.Object), Selections: make(map[*ast.SelectorExpr]*types.Selection)}
	cfg := types.Config{Importer: importer.Default()}
	tp, err := cfg.Check(path, fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &packages.Package{ID: path, PkgPath: path, Name: tp.Name(), Fset: fset, Types: tp, TypesInfo: info, Syntax: []*ast.File{file}, Imports: make(map[string]*packages.Package)}
	if !standard {
		pkg.Module = &packages.Module{Path: "example.com/app"}
	}
	for _, dep := range tp.Imports() {
		pkg.Imports[dep.Path()] = &packages.Package{PkgPath: dep.Path(), Types: dep}
	}
	return pkg
}

func TestTypedProfileBoundaries(t *testing.T) {
	t.Parallel()
	s := profileSelection(t, "rt")
	policies, err := Policies(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, source, want string }{
		{"pure", `package app; import "math"; var X=math.Sqrt(4)`, ""},
		{"modeled-time", `package app; import "time"; var X=time.Second`, ""},
		{"runtime-rewrite", `package app; import "runtime"; func F(){runtime.Gosched()}`, ""},
		{"stack-metadata", `package app; import "runtime"; func F(pc uintptr) string{return runtime.FuncForPC(pc).Name()}`, ""},
		{"runtime-host", `package app; import "runtime"; func F(){runtime.GC()}`, "unsupported runtime.GC"},
		{"runtime-dot", `package app; import . "runtime"; func F(){GC()}`, "unsupported runtime.GC"},
		{"runtime-value", `package app; import "runtime"; var F=runtime.GC`, "unsupported runtime.GC"},
		{"native-map", `package app; import "reflect"; var X=reflect.TypeOf(1)`, ""},
		{"embed-bytes", `package app; import _ "embed"; var X string`, ""},
		{"embed-filesystem", `package app; import "embed"; var X embed.FS`, "unsupported embed.FS"},
		{"iterator-type", `package app; import "iter"; var X iter.Seq[int]`, ""},
		{"native-coroutine", `package app; import "iter"; func F(x iter.Seq[int]){_,_=iter.Pull(x)}`, "unsupported iter.Pull"},
		{"raw-syscall", `package app; import "syscall"; func F(){syscall.RawSyscall(0,0,0,0)}`, "unsupported syscall.RawSyscall"},
		{"raw-syscall-dot", `package app; import . "syscall"; var F=RawSyscall`, "unsupported syscall.RawSyscall"},
		{"dynamic-reflection", `package app; import "reflect"; var F=reflect.StructOf`, "unsupported reflect.StructOf"},
		{"http", `package app; import _ "net/http"`, ""},
		{"native-clock", `package app; import "time" //gosim:notranslate
var X=time.Second`, "unreviewed native import time"},
		{"native-pure-directive", `package app; import "math" //gosim:notranslate
var X=math.Sqrt(4)`, ""},
		{"private-symbol", `package app; import _ "unsafe"
//go:linkname f runtime.nanotime
func f() int64`, "unreviewed native linkname"},
		{"escape-prefix", `package app; import _ "unsafe"
//go:linkname f gosimnotranslate/runtime.nanotime
func f() int64`, "unreviewed native linkname"},
		{"assembly", `package app; func f() int64`, "unreviewed bodyless function"},
		{"pushed-link", `package app; import _ "unsafe"
//go:linkname f
func f() int64`, "unreviewed bodyless function"},
		{"deterministic-runtime-random", `package app; import _ "unsafe"
//go:linkname f runtime.fastrandn
func f(n uint32) uint32`, ""},
		{"invalid-runtime-random", `package app; import _ "unsafe"
//go:linkname f runtime.fastrandn
func f(n uint64) uint32`, "invalid runtime link signature"},
		{"absent-runtime-auxv", `package app; import _ "unsafe"
//go:linkname f runtime.getAuxv
func f() []uintptr`, ""},
		{"invalid-runtime-auxv", `package app; import _ "unsafe"
//go:linkname f runtime.getAuxv
func f() []byte`, "invalid runtime link signature"},
		{"local-export", `package app; import _ "unsafe"
//go:linkname f
func f() int64{return 1}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg := typedBoundary(t, "example.com/app", tc.source, false)
			err := ValidateBoundary(pkg, policies)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestHookedSourceAndOwnedNativeImports(t *testing.T) {
	t.Parallel()
	s := profileSelection(t, "rt")
	p, err := Policies(s)
	if err != nil {
		t.Fatal(err)
	}
	hooked := typedBoundary(t, "time", `package time; func runtimeNano() int64`, true)
	if err := ValidateBoundary(hooked, p); err != nil {
		t.Fatal(err)
	}
	raw := typedBoundary(t, "os", `package os; import "syscall"; func F(){syscall.RawSyscall(0,0,0,0)}`, true)
	if err := ValidateBoundary(raw, p); err != nil {
		t.Fatal(err)
	}
	// The original body is replaced completely by the reviewed hook. Its native
	// selector is not evidence of a native call surviving in generated code.
	replaced := typedBoundary(t, "time", `package time; import "runtime"; func runtimeNano() int64{runtime.GC();return 0}`, true)
	if err := ValidateBoundary(replaced, p); err != nil {
		t.Fatal(err)
	}
	shim := gosimtool.Module + "/internal/compat/" + s.Identity.Profile + "/shims/reflect"
	source := `package reflect; import "reflect" //gosim:notranslate
var X=reflect.TypeOf(1)`
	if err := ValidateBoundary(typedBoundary(t, shim, source, false), p); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBoundary(typedBoundary(t, "example.com/app", source, false), p); err == nil {
		t.Fatal("application acquired shim's native exemption")
	}
	if err := ValidateBoundary(nil, p); err == nil {
		t.Fatal("nil source accepted")
	}
	if err := ValidateBoundary(&packages.Package{PkgPath: "example.com/app", Module: &packages.Module{Path: "example.com/app"}}, p); err == nil {
		t.Fatal("missing typed source accepted")
	}
	if err := ValidateBoundary(hooked, PackagePolicies{}); err == nil {
		t.Fatal("unresolved policies accepted")
	}
}

func TestProviderAndRetiredImportBoundaries(t *testing.T) {
	t.Parallel()
	s := profileSelection(t, "rt", "fs", "net")
	p, err := Policies(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, from := range []string{gosimtool.Module, gosimtool.Module + "/fs", gosimtool.Module + "/internal/kernel", gosimtool.Module + "/internal/control", gosimtool.Module + "/internal/contracts", "example.com/fake/gosimcomposition", gosimtool.Module + "/internal/net"} {
		if err := p.importPolicy(from, gosimtool.Module+"/internal/fs", false, false); err == nil {
			t.Fatalf("provider escape from %s", from)
		}
	}
	for _, from := range []string{p.compositionPackage, gosimtool.Module + "/internal/fs", gosimtool.Module + "/internal/fs/model"} {
		if err := p.importPolicy(from, gosimtool.Module+"/internal/fs/model", false, false); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"internal/unreviewed", gosimtool.Module + "/internal/simulation/fs", gosimtool.Module + "/internal/hooks/go125", gosimtool.Module + "/internal/hostabi", gosimtool.Module + "/internal/compat/go124/hooks/rt", gosimtool.Module + "/internal/compat/" + s.Identity.Profile + "/profile"} {
		if _, err := p.packagePolicy(path, strings.HasPrefix(path, "internal/")); err == nil {
			t.Fatalf("unexpected admission %s", path)
		}
	}
}
