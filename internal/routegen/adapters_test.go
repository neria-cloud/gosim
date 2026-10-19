package routegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	p125 "github.com/neria-cloud/gosim/internal/compat/go125/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestAdapterSourceAndLayoutDrift(t *testing.T) {
	tc, err := buildcfg.Resolve("go1.25.14")
	if err != nil {
		t.Fatal(err)
	}
	sources, err := loadSources(tc, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renderLayouts(sources, "amd64", "go125"); err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"syscall", "golang.org/x/sys/unix"} {
		for _, name := range layoutNames {
			t.Run(pkg+"/"+name, func(t *testing.T) {
				original := sources[pkg].Types[name]
				delete(sources[pkg].Types, name)
				if _, err := renderLayouts(sources, "amd64", "go125"); err == nil {
					t.Fatal("missing type accepted")
				}
				wrong, _ := parser.ParseExpr("struct{ Pointer *byte }")
				sources[pkg].Types[name] = wrong
				if _, err := renderLayouts(sources, "amd64", "go125"); err == nil {
					t.Fatal("wrong pointer layout accepted")
				}
				sources[pkg].Types[name] = original
			})
		}
		original := sources[pkg].Constants["SYS_FCNTL"]
		sources[pkg].Constants["SYS_FCNTL"] = &ast.BasicLit{Kind: token.INT, Value: "9999"}
		if _, err := renderLayouts(sources, "amd64", "go125"); err == nil {
			t.Fatal("wrong syscall number accepted")
		}
		sources[pkg].Constants["SYS_FCNTL"] = original
	}
	for _, pkg := range []string{"syscall", "golang.org/x/sys/unix", "internal/syscall/unix"} {
		for _, name := range adapterSymbols(pkg) {
			original := sources[pkg].Functions[name]
			delete(sources[pkg].Functions, name)
			if _, err := adapterHooks("go125", sources); err == nil {
				t.Fatal("missing required declaration", pkg, name)
			}
			sources[pkg].Functions[name] = original
		}
	}
	hooks := p125.Descriptor().Hooks
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
	classified := map[string]bool{}
	for _, pkg := range []string{"syscall", "golang.org/x/sys/unix"} {
		for _, symbol := range adapterSymbols(pkg) {
			classified[adapterPrefix(pkg)+symbol] = true
		}
	}
	// The generated legacy lists are an independent completeness oracle until
	// the authorized cleanup removes them in Task 26.
	for _, file := range []string{"syscall_gen_amd64.go", "golangorg_x_sys_unix_gen_amd64.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join("../hooks/go125", file), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && !classified[fn.Name.Name] {
				t.Error("missing legacy wrapper", fn.Name.Name)
			}
		}
	}
	for _, owner := range []string{"kernel", "fs", "net"} {
		data, err := os.ReadFile("../compat/go125/hooks/" + owner + "/syscalls_amd64.go")
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
	tc, err := buildcfg.Resolve("go1.25.14")
	if err != nil {
		t.Fatal(err)
	}
	amd, err := loadSources(tc, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	arm, err := loadSources(tc, "arm64")
	if err != nil {
		t.Fatal(err)
	}
	a, err := adapterHooks("go125", amd)
	if err != nil {
		t.Fatal(err)
	}
	b, err := adapterHooks("go125", arm)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("logical hooks or capabilities differ by architecture")
	}
	if _, err := renderLayouts(arm, "arm64", "go125"); err != nil {
		t.Fatal(err)
	}
	stat := arm["syscall"].Types["Stat_t"]
	arm["syscall"].Types["Stat_t"] = amd["syscall"].Types["Stat_t"]
	if _, err := renderLayouts(arm, "arm64", "go125"); err == nil {
		t.Fatal("amd64 stat accepted as arm64")
	}
	arm["syscall"].Types["Stat_t"] = stat
	arm["golang.org/x/sys/unix"].Constants["SYS_NEWFSTATAT"] = &ast.BasicLit{Kind: token.INT, Value: "262"}
	if _, err := renderLayouts(arm, "arm64", "go125"); err == nil {
		t.Fatal("wrong fstatat alias accepted")
	}
}
