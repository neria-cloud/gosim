package routegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

// Private unix uses syscall.Stat_t while the public syscall/x/sys adapters
// expose unqualified records. Both must use the reviewed layout, without
// rewriting an unrelated package's identically named type.
func TestAdapterStatSelector(t *testing.T) {
	for _, tc := range []struct{ source, pkg, owner, want string }{
		{"*syscall.Stat_t", "internal/syscall/unix", "fs", "*kh.Stat_t"},
		{"*Stat_t", "syscall", "fs", "*kh.Stat_t"},
		{"*Stat_t", "golang.org/x/sys/unix", "fs", "*kh.XStat_t"},
		{"*other.Stat_t", "internal/syscall/unix", "fs", "*other.Stat_t"},
	} {
		t.Run(tc.pkg+"/"+tc.source, func(t *testing.T) {
			expr, err := parser.ParseExpr(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if got := sourceText(adapterType(expr, tc.pkg, tc.owner)); got != tc.want {
				t.Fatalf("adapter type = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestPrivateStatSignaturePreservesSourceContract(t *testing.T) {
	expr, err := parser.ParseExpr("func(int, string, *syscall.Stat_t, int) error")
	if err != nil {
		t.Fatal(err)
	}
	source := expr.(*ast.FuncType)
	const wantSource = "func(int, string, *syscall.Stat_t, int) error"
	const wantTarget = "func(p0 int, p1 string, p2 *kh.Stat_t, p3 int) error"
	got, err := signatureType(source, true, "internal/syscall/unix", "fs")
	if err != nil || got != wantTarget {
		t.Fatalf("target = %s, %v", got, err)
	}
	got, err = signatureType(source, false, "internal/syscall/unix", "fs")
	if err != nil || got != wantSource {
		t.Fatalf("source contract changed: %s, %v", got, err)
	}
}

func TestAdapterSourceAndLayoutDrift(t *testing.T) {
	t.Parallel()
	tc, profile, _ := currentGeneration(t)
	sources, err := loadSources(tc, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renderLayouts(sources, "amd64", "//go:build go1.25\n"); err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"syscall", "golang.org/x/sys/unix"} {
		for _, name := range layoutNames {
			t.Run(pkg+"/"+name, func(t *testing.T) {
				original := sources[pkg].Types[name]
				delete(sources[pkg].Types, name)
				if _, err := renderLayouts(sources, "amd64", "//go:build go1.25\n"); err == nil {
					t.Fatal("missing type accepted")
				}
				wrong, _ := parser.ParseExpr("struct{ Pointer *byte }")
				sources[pkg].Types[name] = wrong
				if _, err := renderLayouts(sources, "amd64", "//go:build go1.25\n"); err == nil {
					t.Fatal("wrong pointer layout accepted")
				}
				sources[pkg].Types[name] = original
			})
		}
		original := sources[pkg].Constants["SYS_FCNTL"]
		sources[pkg].Constants["SYS_FCNTL"] = &ast.BasicLit{Kind: token.INT, Value: "9999"}
		if _, err := renderLayouts(sources, "amd64", "//go:build go1.25\n"); err == nil {
			t.Fatal("wrong syscall number accepted")
		}
		sources[pkg].Constants["SYS_FCNTL"] = original
	}
	for _, pkg := range []string{"syscall", "golang.org/x/sys/unix", "internal/syscall/unix"} {
		for _, name := range adapterSymbols(pkg) {
			original := sources[pkg].Functions[name]
			delete(sources[pkg].Functions, name)
			if _, err := adapterHooks(profile, sources); err == nil {
				t.Fatal("missing required declaration", pkg, name)
			}
			sources[pkg].Functions[name] = original
		}
	}
	hooks := profile.Hooks
	for i, h := range hooks {
		if h.Package == "syscall" && h.Symbol == "FcntlFlock" {
			hooks[i].Signature = "func(uintptr, int, uintptr) error"
			break
		}
	}
	if err := spec.VerifyHooks(tc, hooks); err == nil || !strings.Contains(err.Error(), "ABI drift") {
		t.Fatal("wrong pointer signature", err)
	}
}

func TestAdapterInventoryAndProviderBoundary(t *testing.T) {
	t.Parallel()
	classified := map[string]bool{}
	for _, pkg := range []string{"syscall", "golang.org/x/sys/unix"} {
		for _, symbol := range adapterSymbols(pkg) {
			classified[adapterPrefix(pkg)+symbol] = true
		}
	}
	// Frozen declaration names from the old go125 syscall/x/sys generated
	// wrappers at 7c57f63, independent of the active adapter symbol table.
	data, err := os.ReadFile("testdata/legacy_adapters.txt")
	if err != nil {
		t.Fatal(err)
	}
	if buildspec.Hash(data) != "84e163616526ccf7410afbf16e0659f7c5a4b03c1ecb1fece03fe09b8ef3c0c4" {
		t.Fatal("baseline wrapper inventory changed")
	}
	for _, name := range strings.Fields(string(data)) {
		if !classified[name] {
			t.Error("missing legacy wrapper", name)
		}
	}
	for _, owner := range []string{"kernel", "fs", "net"} {
		data, err := os.ReadFile("../compat/hooks/" + owner + "/syscalls_amd64.go")
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(token.NewFileSet(), "generated.go", data, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			if strings.Contains(imp.Path.Value, "gosim/internal/fs") || strings.Contains(imp.Path.Value, "gosim/internal/net") {
				t.Fatal("adapter imports provider", imp.Path.Value)
			}
		}
		if owner != "kernel" {
			for _, d := range f.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || !strings.Contains(fn.Name.Name, "_") {
					continue
				}
				expr, ok := fn.Body.List[0].(*ast.ExprStmt)
				if !ok || !strings.HasPrefix(sourceText(expr.X), "kh.Require(") {
					t.Fatal("optional wrapper lacks first-operation gate", fn.Name.Name)
				}
			}
		}
	}
}

func TestArchitectureRouteParity(t *testing.T) {
	t.Parallel()
	tc, profile, _ := currentGeneration(t)
	amd, err := loadSources(tc, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	arm, err := loadSources(tc, "arm64")
	if err != nil {
		t.Fatal(err)
	}
	a, err := adapterHooks(profile, amd)
	if err != nil {
		t.Fatal(err)
	}
	b, err := adapterHooks(profile, arm)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("logical hooks or capabilities differ by architecture")
	}
	if _, err := renderLayouts(arm, "arm64", "//go:build go1.25\n"); err != nil {
		t.Fatal(err)
	}
	stat := arm["syscall"].Types["Stat_t"]
	arm["syscall"].Types["Stat_t"] = amd["syscall"].Types["Stat_t"]
	if _, err := renderLayouts(arm, "arm64", "//go:build go1.25\n"); err == nil {
		t.Fatal("amd64 stat accepted as arm64")
	}
	arm["syscall"].Types["Stat_t"] = stat
	arm["golang.org/x/sys/unix"].Constants["SYS_NEWFSTATAT"] = &ast.BasicLit{Kind: token.INT, Value: "262"}
	if _, err := renderLayouts(arm, "arm64", "//go:build go1.25\n"); err == nil {
		t.Fatal("wrong fstatat alias accepted")
	}
}
