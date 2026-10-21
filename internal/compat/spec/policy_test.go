package spec_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	p125 "github.com/neria-cloud/gosim/internal/compat/go125/profile"
	p126 "github.com/neria-cloud/gosim/internal/compat/go126/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestFamiliesPackagePolicies(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"go125", "go126"} {
		t.Run(family, func(t *testing.T) {
			p, m := p125.Descriptor(), p125.PolicyManifest()
			if family == "go126" {
				p, m = p126.Descriptor(), p126.PolicyManifest()
			}
			if err := p.Validate(); err != nil {
				t.Fatal(err)
			}
			if err := m.Validate(p); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "context", "time", "net", "io", "io/fs", "net/http", "errors", "strconv", "sync", "os", "golang.org/x/sys/unix"} {
				policy, err := p.Policy(path, !strings.Contains(strings.Split(path, "/")[0], "."))
				if err != nil || policy.Mode != "translate" {
					t.Errorf("%s: %+v %v", path, policy, err)
				}
			}
			for _, path := range []string{"net/http/httptest", "github.com/modern-go/reflect2", "C", "testing/synctest", "crypto/internal/sysrand", "crypto/internal/fips140", "weak"} {
				if _, err := p.Policy(path, true); err == nil {
					t.Errorf("unsafe package %s admitted", path)
				}
			}
			for _, path := range []string{"crypto/sha256", "crypto/aes", "math/big"} {
				if policy, err := p.Policy(path, true); err != nil || policy.Mode != "native-pure" {
					t.Fatalf("computation boundary %s: %+v %v", path, policy, err)
				}
			}
			for _, item := range [][2]string{{"crypto", "RegisterHash"}, {"crypto/cipher", "NewGCMWithRandomNonce"}, {"crypto/subtle", "WithDataIndependentTiming"}} {
				if _, err := m.ResolveSymbol(p, item[0], item[1], true); err == nil {
					t.Errorf("host state/entropy escape admitted: %v", item)
				}
			}
			for _, path := range []string{"crypto", "crypto/cipher", "crypto/ed25519", "hash/crc32"} {
				if err := m.AllowNativeImport(p, "example.test/application", path, true); err == nil {
					t.Errorf("native directive bypassed symbol checks: %s", path)
				}
			}
			for _, item := range [][2]string{{"crypto/ed25519", "Sign"}, {"crypto/ed25519", "Verify"}, {"crypto/ed25519", "GenerateKey"}, {"hash/crc32", "Checksum"}, {"hash/crc32", "MakeTable"}, {"hash/crc32", "IEEETable"}, {"hash/crc32", "NewIEEE"}, {"hash/crc32", "ChecksumIEEE"}} {
				if target, err := m.ResolveSymbol(p, item[0], item[1], true); err != nil || target.Package != item[0] {
					t.Fatalf("computation did not stay native: %v %v", target, err)
				}
			}
			if policy, err := p.Policy("crypto/rand", true); err != nil || policy.Mode != "bridge" {
				t.Fatalf("exact deterministic bridge did not override crypto rejection: %+v %v", policy, err)
			}
			if policy, err := p.Policy("example.test/application", false); err != nil || policy.Mode != "translate" {
				t.Fatal("application code must be translated")
			}
			if _, err := p.Policy("future/stdlib", true); err == nil {
				t.Fatal("unknown standard package became native")
			}
			if policy, err := p.Policy("google.golang.org/grpc-extra", false); err != nil || policy.Mode != "translate" {
				t.Fatal("subtree rule matched adjacent path")
			}
			for _, path := range []string{"", "../net", "net\\http", "net http"} {
				if _, err := p.Policy(path, true); err == nil {
					t.Errorf("invalid path %q accepted", path)
				}
			}
		})
	}
}

func TestFamiliesNativeBridges(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"go125", "go126"} {
		t.Run(family, func(t *testing.T) {
			p, m := p125.Descriptor(), p125.PolicyManifest()
			if family == "go126" {
				p, m = p126.Descriptor(), p126.PolicyManifest()
			}
			for _, symbol := range []string{"Gosched", "GOOS", "SetFinalizer", "NumCPU", "GOMAXPROCS", "ReadMemStats", "AddCleanup", "SetBlockProfileRate"} {
				target, err := m.ResolveSymbol(p, "runtime", symbol, true)
				if err != nil || !strings.HasPrefix(target.Package, "github.com/neria-cloud/gosim/") {
					t.Fatalf("runtime.%s did not use a model: %+v %v", symbol, target, err)
				}
			}
			for _, symbol := range []string{"Caller", "CallersFrames", "Goexit", "KeepAlive", "Version"} {
				if target, err := m.ResolveSymbol(p, "runtime", symbol, true); err != nil || target.Package != "runtime" {
					t.Fatalf("reviewed primitive bridge %s: %+v %v", symbol, target, err)
				}
			}
			for _, symbol := range []string{"newFutureRuntimeFunction"} {
				if _, err := m.ResolveSymbol(p, "runtime", symbol, true); err == nil {
					t.Errorf("native runtime.%s admitted", symbol)
				}
			}
			for _, path := range []string{"reflect", "testing", "internal/synctest", "crypto/rand", "crypto/tls", "crypto/x509", "os/exec", "os/signal", "runtime/pprof"} {
				target, err := m.ResolveSymbol(p, path, "TypeFor", true)
				if err != nil || spec.ClassifyCompatPackage(target.Package) != spec.CompatShims {
					t.Fatalf("whole shim %s: %+v %v", path, target, err)
				}
			}
			if target, err := m.ResolveSymbol(p, "internal/race", "Acquire", true); err != nil || target.Target != "InternalRace_Acquire" {
				t.Fatalf("race bridge=%+v %v", target, err)
			}
			if _, err := m.ResolveSymbol(p, "iter", "Pull", true); err == nil {
				t.Fatal("native coroutine escape admitted")
			}
			if _, err := m.ResolveSymbol(p, "iter", "Seq", true); err != nil {
				t.Fatal(err)
			}
			if _, err := m.ResolveSymbol(p, "embed", "", true); err != nil {
				t.Fatal("blank embed import rejected")
			}
			if _, err := m.ResolveSymbol(p, "embed", "FS", true); err == nil {
				t.Fatal("native filesystem type admitted")
			}
		})
	}
}

func TestMinimalImportInitClassification(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"go125", "go126"} {
		t.Run(family, func(t *testing.T) {
			m := p125.PolicyManifest()
			if family == "go126" {
				m = p126.PolicyManifest()
			}
			for _, features := range [][]string{{"rt"}, {"rt", "fs"}, {"rt", "net"}, {"rt", "fs", "net"}} {
				for _, item := range []struct {
					pkg, symbol string
					args        map[string]int64
				}{
					{"os", "runtime_args", nil},
					{"syscall", "runtime_envs", nil},
					{"syscall", "Getrlimit", map[string]int64{"resource": 7}},
					{"internal/syscall/unix", "Fcntl", map[string]int64{"fd": 1, "command": 3, "argument": 0}},
					{"sync", "runtime_notifyListCheck", nil},
					{"sync", "runtime_registerPoolCleanup", nil},
					{"internal/godebug", "setUpdate", nil},
					{"internal/godebug", "setNewIncNonDefault", nil},
					{"testing", "Init", nil},
				} {
					probe, err := m.Probe(item.pkg, item.symbol, features, item.args)
					if err != nil || probe.Phase != "init" || probe.Disposition != "modeled" || probe.Feature != "rt" {
						t.Fatalf("%v init %s.%s: %+v %v", features, item.pkg, item.symbol, probe, err)
					}
				}
			}
			for _, args := range []map[string]int64{nil, {"fd": 3, "command": 3, "argument": 0}, {"fd": 1, "command": 4, "argument": 0}, {"fd": 1, "command": 3, "argument": 1}, {"fd": 1, "command": 3, "argument": 0, "extra": 0}} {
				if _, err := m.Probe("internal/syscall/unix", "Fcntl", []string{"rt"}, args); err == nil {
					t.Fatalf("blanket diagnostic fcntl success: %v", args)
				}
			}
			if _, err := m.Probe("syscall", "Getrlimit", []string{"rt"}, map[string]int64{"resource": 1}); err == nil {
				t.Fatal("unmodeled resource limit accepted")
			}
			if _, err := m.Probe("net", "*ipStackCapabilities.probe", []string{"rt"}, nil); err == nil || !strings.Contains(err.Error(), "FeatureDisabled") {
				t.Fatalf("net gate: %v", err)
			}
			if probe, err := m.Probe("net", "*ipStackCapabilities.probe", []string{"rt", "net"}, nil); err != nil || probe.Phase != "first-use" {
				t.Fatalf("lazy owned net probe: %+v %v", probe, err)
			}
			for _, symbol := range []string{"dnsReadConfig", "readHosts"} {
				if _, err := m.Probe("net", symbol, []string{"rt", "net"}, nil); err == nil || !strings.Contains(err.Error(), "feature=fs") {
					t.Fatalf("implicit fs enablement: %s %v", symbol, err)
				}
			}
			for _, item := range [][2]string{{"os", "checkClonePidfd"}, {"internal/synctest", "Run"}, {"syscall", "unknownOperation"}} {
				if _, err := m.Probe(item[0], item[1], []string{"rt", "fs", "net"}, nil); err == nil {
					t.Fatalf("unknown/unsupported init success: %v", item)
				}
			}
		})
	}
}

func TestPolicyFixtures(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"go125", "go126"} {
		t.Run(family, func(t *testing.T) {
			p, m := p125.Descriptor(), p125.PolicyManifest()
			if family == "go126" {
				p, m = p126.Descriptor(), p126.PolicyManifest()
			}
			fixtures, err := filepath.Glob("testdata/policy/*.go")
			if err != nil {
				t.Fatal(err)
			}
			for _, filename := range fixtures {
				t.Run(filepath.Base(filename), func(t *testing.T) {
					file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ParseComments)
					if err != nil {
						t.Fatal(err)
					}
					imports := make(map[string]string)
					var rejected error
					for _, imp := range file.Imports {
						path, err := strconv.Unquote(imp.Path.Value)
						if err != nil {
							t.Fatal(err)
						}
						alias := filepath.Base(path)
						if imp.Name != nil {
							alias = imp.Name.Name
						}
						imports[alias] = path
						standard := !strings.Contains(strings.Split(path, "/")[0], ".")
						if _, err := p.Policy(path, standard); err != nil {
							rejected = err
						}
						for _, comments := range []*ast.CommentGroup{imp.Doc, imp.Comment} {
							if hasNativeDirective(comments) {
								if err := m.AllowNativeImport(p, "example.test/application", path, standard); err != nil {
									rejected = err
								}
							}
						}
					}
					ast.Inspect(file, func(node ast.Node) bool {
						selector, ok := node.(*ast.SelectorExpr)
						if !ok {
							return true
						}
						ident, ok := selector.X.(*ast.Ident)
						if !ok {
							return true
						}
						path, ok := imports[ident.Name]
						if !ok {
							return true
						}
						_, err := m.ResolveSymbol(p, path, selector.Sel.Name, !strings.Contains(strings.Split(path, "/")[0], "."))
						if err != nil {
							rejected = err
						}
						return true
					})
					shouldPass := (filepath.Base(filename) == "minimal.go" || filepath.Base(filename) == "grpc.go")
					if shouldPass && rejected != nil {
						t.Fatal(rejected)
					}
					if !shouldPass && rejected == nil {
						t.Fatal("unsafe boundary fixture was admitted")
					}
				})
			}
		})
	}
}

func TestOwnedNativeImportAudit(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"go125", "go126"} {
		t.Run(family, func(t *testing.T) {
			p, m := p125.Descriptor(), p125.PolicyManifest()
			if family == "go126" {
				p, m = p126.Descriptor(), p126.PolicyManifest()
			}
			for _, entry := range m.NativeImports {
				if err := m.AllowNativeImport(p, entry.From, entry.Package, true); err != nil {
					t.Fatal(err)
				}
				if err := m.AllowNativeImport(p, "example.test/application", entry.Package, true); err == nil {
					t.Fatalf("trusted import leaked to application: %+v", entry)
				}
			}
			var paths []string
			for _, pattern := range []string{"../" + family + "/shims/*/*.go", "../shims/*/*.go", "../hooks/rt/*.go"} {
				matches, err := filepath.Glob(pattern)
				if err != nil {
					t.Fatal(err)
				}
				paths = append(paths, matches...)
			}
			if len(paths) == 0 {
				t.Fatal("empty native import audit")
			}
			for _, filename := range paths {
				if strings.HasSuffix(filename, "_test.go") {
					continue
				}
				active := false
				for _, arch := range []string{"amd64", "arm64"} {
					for _, race := range []bool{false, true} {
						ctx := sourceContext(family, arch)
						if race {
							ctx.BuildTags = append(ctx.BuildTags, "race")
						}
						match, err := ctx.MatchFile(filepath.Dir(filename), filepath.Base(filename))
						if err != nil {
							t.Fatal(err)
						}
						active = active || match
					}
				}
				if !active {
					continue
				}
				file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ParseComments)
				if err != nil {
					t.Fatal(err)
				}
				owner := "github.com/neria-cloud/gosim/internal/compat/" + filepath.ToSlash(strings.TrimPrefix(filepath.Dir(filename), "../"))
				for _, imp := range file.Imports {
					for _, comments := range []*ast.CommentGroup{imp.Doc, imp.Comment} {
						if !hasNativeDirective(comments) {
							continue
						}
						path, err := strconv.Unquote(imp.Path.Value)
						if err != nil {
							t.Fatal(err)
						}
						if err := m.AllowNativeImport(p, owner, path, true); err != nil {
							t.Fatalf("%s: %v", filename, err)
						}
					}
				}
			}
		})
	}
}

func TestPolicyManifestRejectsInvalidOwnership(t *testing.T) {
	t.Parallel()
	p := p125.Descriptor()
	cases := []func(*spec.PolicyManifest){
		func(m *spec.PolicyManifest) { m.SharedGlobals[0].Package = "example.test/application" },
		func(m *spec.PolicyManifest) {
			m.Bridges[0].Replacement = "github.com/neria-cloud/gosim/internal/compat/go126/shims/reflect"
		},
		func(m *spec.PolicyManifest) { m.Bridges[0].NativeIsland = true },
		func(m *spec.PolicyManifest) { m.Probes = append(m.Probes, m.Probes[0]) },
		func(m *spec.PolicyManifest) { m.Probes[0].Disposition = "success" },
		func(m *spec.PolicyManifest) { m.NativeImports[0].From = "example.test/untrusted" },
		func(m *spec.PolicyManifest) { m.Modules[0].Version = "" },
	}
	for _, change := range cases {
		m := p125.PolicyManifest()
		change(&m)
		if err := m.Validate(p); err == nil {
			t.Fatal("invalid policy manifest accepted")
		}
	}
}

// CommentGroup.Text omits directive comments; inspect the original tokens.
func hasNativeDirective(group *ast.CommentGroup) bool {
	if group != nil {
		for _, comment := range group.List {
			if strings.TrimSpace(strings.TrimPrefix(comment.Text, "//")) == "gosim:notranslate" {
				return true
			}
		}
	}
	return false
}

func TestInitProbeSourceEvidence(t *testing.T) {
	t.Parallel()
	sources := make(map[string]bool)
	for _, source := range p125.VerificationOrigin().Files {
		sources[source.Path] = true
	}
	for _, probe := range p125.PolicyManifest().Probes {
		if !sources[probe.Source] {
			t.Errorf("probe %s.%s lacks pinned source evidence: %s", probe.Package, probe.Symbol, probe.Source)
		}
	}
}
