package spec_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	p125 "github.com/neria-cloud/gosim/internal/compat/go125/profile"
	p126 "github.com/neria-cloud/gosim/internal/compat/go126/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

type declarationEntry struct {
	File, Kind, Name, Signature, Disposition, Target, Package, Symbol, Note string
	Arches                                                                  []string
	AbsentIn                                                                []string `json:"absent_in"`
}
type declarationLedger struct {
	Baseline           string
	InventoryDigest    string `json:"inventory_digest"`
	DeclarationsDigest string `json:"declarations_digest"`
	Declarations       []declarationEntry
}

func readDeclarationLedger(t *testing.T) declarationLedger {
	t.Helper()
	data, err := os.ReadFile("testdata/declaration_ledger.json")
	if err != nil {
		t.Fatal(err)
	}
	var ledger declarationLedger
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ledger); err != nil {
		t.Fatal(err)
	}
	if ledger.Baseline != "e6122b1" || len(ledger.Declarations) != 964 || !buildspec.ValidDigest(ledger.InventoryDigest) {
		t.Fatal("changed baseline inventory")
	}
	var baseline strings.Builder
	seen := map[string]bool{}
	for _, d := range ledger.Declarations {
		key := d.File + ":" + d.Name
		if seen[key] || d.Signature == "" || !slices.Contains([]string{"func", "type"}, d.Kind) {
			t.Fatal("invalid baseline declaration", d)
		}
		seen[key] = true
		baseline.WriteString(strings.Join([]string{d.File, d.Kind, d.Name, d.Signature}, "\x00") + "\n")
	}
	const digest = "13d35fd9e126af80ec94d45a0d0793428ffc6087dfd3c96c7681adb9ea191f73"
	if ledger.DeclarationsDigest != digest || buildspec.Hash([]byte(baseline.String())) != digest {
		t.Fatal("baseline declaration removed or changed")
	}
	return ledger
}

func sourceContext(family, arch string) build.Context {
	ctx := build.Context{GOOS: "linux", GOARCH: arch, Compiler: "gc", BuildTags: strings.Split(buildspec.TranslatedSourceTags(false), ",")}
	minor, err := strconv.Atoi(strings.TrimPrefix(family, "go1"))
	if err != nil {
		panic(err)
	}
	for v := 1; v <= minor; v++ {
		ctx.ReleaseTags = append(ctx.ReleaseTags, fmt.Sprintf("go1.%d", v))
	}
	return ctx
}

// Selected declarations are parsed per package, family and simulated architecture.
// No build of another family's executable source is needed for this audit.
func ownedDeclarations(t *testing.T, pkg string, ctx build.Context) map[string]string {
	t.Helper()
	dir := filepath.Join("../../..", pkg)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		selected, err := ctx.MatchFile(dir, name)
		if err != nil {
			t.Fatal(err)
		}
		if !selected {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		add := func(name, kind string) {
			if _, ok := result[name]; ok {
				t.Fatal("duplicate selected declaration", pkg, name)
			}
			result[name] = kind
		}
		for _, decl := range f.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				name := decl.Name.Name
				if name == "init" || name == "_" {
					continue
				}
				if decl.Recv != nil {
					var b bytes.Buffer
					if err := printer.Fprint(&b, token.NewFileSet(), decl.Recv.List[0].Type); err != nil {
						t.Fatal(err)
					}
					name = b.String() + "." + name
				}
				add(name, "func")
			case *ast.GenDecl:
				for _, s := range decl.Specs {
					if typ, ok := s.(*ast.TypeSpec); ok {
						if typ.Name.Name != "_" {
							add(typ.Name.Name, "type")
						}
					}
					if values, ok := s.(*ast.ValueSpec); ok {
						for _, name := range values.Names {
							if name.Name != "_" {
								add(name.Name, decl.Tok.String())
							}
						}
					}
				}
			}
		}
	}
	if len(result) == 0 {
		t.Fatal("empty selected package inventory", pkg, ctx.GOOS, ctx.GOARCH)
	}
	return result
}

// Preserve immutable ledger spellings; resolve moved targets against the current
// package layout without changing historical inventory or its digest.
func currentLedgerTarget(t *testing.T, target, family string) string {
	t.Helper()
	pkg, _, _ := strings.Cut(target, ".")
	if _, err := os.Stat(filepath.Join("../../..", pkg)); err == nil {
		return target
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return strings.Replace(target, "internal/compat/"+family+"/", "internal/compat/", 1)
}

func TestAllInventoriedHookAndShimDeclarations(t *testing.T) {
	t.Parallel()
	ledger := readDeclarationLedger(t)
	for _, family := range []struct {
		name           string
		descriptor     func(string) (spec.Profile, error)
		policyManifest func() spec.PolicyManifest
	}{{"go125", p125.DescriptorForArch, p125.PolicyManifest}, {"go126", p126.DescriptorForArch, p126.PolicyManifest}} {
		for _, arch := range []string{"amd64", "arm64"} {
			t.Run(family.name+"/"+arch, func(t *testing.T) {
				p, err := family.descriptor(arch)
				if err != nil {
					t.Fatal(err)
				}
				hooks := map[string]spec.Hook{}
				for _, h := range p.Hooks {
					hooks[h.Package+"."+h.Symbol] = h
				}
				bridges := map[string]string{}
				for _, bridge := range family.policyManifest().Bridges {
					bridges[bridge.Package] = bridge.Replacement
				}
				packages := map[string]map[string]string{}
				for _, d := range ledger.Declarations {
					label := d.File + ":" + d.Name
					if d.Disposition == "package-reject" || d.Disposition == "package-bridge" {
						policy, err := p.Policy(d.Package, true)
						if d.Disposition == "package-reject" && (err == nil || policy.Mode != "reject" || d.Note == "") {
							t.Error("missing package rejection", label)
						}
						if d.Disposition == "package-bridge" && (err != nil || policy.Mode != "bridge" || bridges[d.Package] == "" || d.Note == "") {
							t.Error("missing translated package bridge", label)
						}
						continue
					}
					if !slices.Contains([]string{"hook", "moved-hook", "binding", "replacement"}, d.Disposition) {
						t.Fatal("unreviewed disposition", label)
					}
					target := currentLedgerTarget(t, strings.ReplaceAll(d.Target, "{family}", family.name), family.name)
					key := d.Package + "." + d.Symbol
					if len(d.Arches) > 0 && !slices.Contains(d.Arches, arch) || slices.Contains(d.AbsentIn, family.name) {
						if _, ok := hooks[key]; ok {
							t.Error("absent declaration selected", label)
						}
						if d.Note == "" && d.Package != "internal/cpu" {
							t.Error("unexplained absence", label)
						}
						continue
					}
					if d.Disposition == "hook" || d.Disposition == "moved-hook" {
						h, ok := hooks[key]
						if !ok || h.Target != "github.com/neria-cloud/gosim/"+target {
							t.Error("missing/wrong hook target", label, target)
						}
					} else if d.Note == "" {
						t.Error("unexplained replacement/binding", label)
					}
					pkg, symbol, ok := strings.Cut(target, ".")
					if !ok {
						t.Fatal("invalid target", target)
					}
					if packages[pkg] == nil {
						packages[pkg] = ownedDeclarations(t, pkg, sourceContext(family.name, arch))
					}
					if got := packages[pkg][symbol]; got != d.Kind {
						t.Error("missing/wrong declaration kind", label, target, got, d.Kind)
					}
				}
			})
		}
	}
}

func TestPinnedRemovedHookDeclarations(t *testing.T) {
	t.Parallel()
	// Verify absence, not just a changed signature. The four mutex helpers now
	// belong to internal/sync; Go 1.25 Clearenv predates runtimeClearenv(map).
	for _, arch := range []string{"amd64", "arm64"} {
		tc := currentToolchain()
		tc.HostArch = arch
		removed := []spec.Hook{}
		for _, name := range []string{"runtime_SemacquireMutex", "runtime_nanotime", "runtime_canSpin", "runtime_doSpin"} {
			removed = append(removed, spec.Hook{Package: "sync", Symbol: name, Signature: "removed"})
		}
		if strings.HasPrefix(runtime.Version(), "go1.25.") {
			removed = append(removed, spec.Hook{Package: "syscall", Symbol: "runtimeClearenv", Signature: "removed"})
		}
		for _, h := range removed {
			if err := spec.VerifyHooks(tc, []spec.Hook{h}); err == nil || !strings.Contains(err.Error(), "is missing") {
				t.Fatalf("obsolete declaration %s.%s: %v", h.Package, h.Symbol, err)
			}
		}
	}
}

func TestSelectedCompatibilityInventories(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"go125", "go126"} {
		for _, arch := range []string{"amd64", "arm64"} {
			for _, race := range []bool{false, true} {
				count := 0
				for _, root := range []string{"../hooks", "../shims", "../host", "../" + family + "/hooks", "../" + family + "/shims", "../" + family + "/host"} {
					if _, err := os.Stat(root); os.IsNotExist(err) {
						continue
					}
					ctx := sourceContext(family, arch)
					if strings.HasSuffix(root, "/host") {
						ctx.GOOS = runtime.GOOS
						ctx.BuildTags = []string{"linkname"}
						ctx.CgoEnabled = race
					}
					if race {
						ctx.BuildTags = append(ctx.BuildTags, "race")
					}
					err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
						if err != nil {
							return err
						}
						if !entry.IsDir() {
							return nil
						}
						files, err := os.ReadDir(path)
						if err != nil {
							return err
						}
						hasSource := false
						for _, file := range files {
							if !file.IsDir() && strings.HasSuffix(file.Name(), ".go") && !strings.HasSuffix(file.Name(), "_test.go") {
								hasSource = true
							}
						}
						if !hasSource {
							return nil
						}
						baseDir, err := filepath.Abs("../../..")
						if err != nil {
							return err
						}
						absolute, err := filepath.Abs(path)
						if err != nil {
							return err
						}
						pkg, err := filepath.Rel(baseDir, absolute)
						if err != nil {
							return err
						}
						count += len(ownedDeclarations(t, filepath.ToSlash(pkg), ctx))
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
				}
				if count == 0 {
					t.Fatal("empty compatibility inventory", family, arch, race)
				}
			}
		}
	}
}

func TestUnsupportedFamilyGuards(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{"../selected", "../../hostabi", "../host"} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		for _, family := range []string{"go124", "go125", "go126", "go127"} {
			ctx := sourceContext(family, "arm64")
			selected, err := ctx.MatchFile(dir, "unsupported.go")
			if err != nil {
				t.Fatal(err)
			}
			want := family == "go124" || family == "go127"
			if selected != want {
				t.Fatal("wrong direct support guard", dir, family, selected)
			}
		}
	}
}

func TestAuditedShimLimitationsAreNamed(t *testing.T) {
	t.Parallel()
	for _, family := range []struct {
		p spec.Profile
		m spec.PolicyManifest
	}{{p125.Descriptor(), p125.PolicyManifest()}, {p126.Descriptor(), p126.PolicyManifest()}} {
		for _, symbol := range []string{"StructOf", "unsafe_New", "typedmemmove", "unsafe_NewArray", "typedslicecopy", "mapassign", "mapaccess", "mapiterinit", "mapiternext", "ifaceE2I", "makemap"} {
			if _, err := family.m.ResolveSymbol(family.p, "reflect", symbol, true); err == nil {
				t.Error("private/unimplemented reflection admitted", family.p.ID, symbol)
			}
		}
		for _, item := range []struct{ pkg, symbol, status string }{
			{"testing", "Init", "stub"},
			{"testing", "CoverMode", "stub"},
			{"testing", "Verbose", "limited"},
			{"internal/synctest", "IsInBubble", "stub"},
			{"internal/synctest", "Associate", "stub"},
			{"internal/synctest", "Disassociate", "stub"},
			{"internal/synctest", "IsAssociated", "stub"},
			{"internal/synctest", "Acquire", "stub"},
			{"internal/synctest", "Bubble.Run", "limited"},
			{"internal/synctest", "Bubble.Release", "limited"},
		} {
			found := false
			for _, c := range family.m.Capabilities {
				if c.Package == item.pkg && c.Symbol == item.symbol {
					found = true
					if c.Status != item.status || c.Reason == "" {
						t.Error("undocumented limitation", family.p.ID, c)
					}
				}
			}
			if !found {
				t.Error("missing named limitation", family.p.ID, item)
			}
		}
	}
}
