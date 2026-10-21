package spec

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectedShimBindings(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal/compat/shims/fixture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("base.go", "//go:build go1.25\n\npackage fixture\ntype T struct{}\nfunc (*T) M() {}\n")
	write("old.go", "//go:build go1.25 && !go1.26\n\npackage fixture\nfunc F() int {return 1}\n")
	write("new.go", "//go:build go1.26\n\npackage fixture\nfunc F() uint {return 1}\n")
	write("cgo.go", "package fixture\nimport \"C\"\nfunc F() bool {return true}\n")
	write("ignored_windows.go", "package fixture\nfunc F() bool {return true}\n")
	write("ignored_test.go", "package fixture\nfunc F() bool {return true}\n")
	write("arm_arm64.go", "package fixture\nfunc ARM() {}\n")
	for _, family := range []string{"1.25", "1.26", "1.27"} {
		signature := "func() uint"
		if family == "1.25" {
			signature = "func() int"
		}
		r := ResolvedFamily{Family: family}
		for _, item := range []struct{ name, signature string }{{"F", signature}, {"*T.M", "func()"}, {"ARM", "func()"}} {
			r.Declarations = append(r.Declarations, Declaration{Hook: Hook{Package: "example", Symbol: item.name, Signature: item.signature, Target: module + "/internal/compat/shims/fixture." + item.name, Owner: "rt"}})
		}
		source := SourceContext{OS: "linux", Arch: "arm64"}
		if err := VerifyShimBindings(root, r, source); err != nil {
			t.Fatal(family, err)
		}
		source.Arch = "amd64"
		if err := VerifyShimBindings(root, r, source); err == nil {
			t.Fatal("inactive architecture file satisfied binding")
		}
		source.Arch = "arm64"
		write("duplicate.go", "package fixture\nfunc F() uint {return 1}\n")
		if err := VerifyShimBindings(root, r, source); err == nil {
			t.Fatal("selected duplicate accepted")
		}
		if err := os.Remove(filepath.Join(dir, "duplicate.go")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExplicitSourcePredicates(t *testing.T) {
	original := build.Default.ReleaseTags
	build.Default.ReleaseTags = []string{"go1.999"}
	defer func() { build.Default.ReleaseTags = original }()
	dir := t.TempDir()
	for name, text := range map[string]string{
		"future.go": "//go:build go1.28\n\npackage p\n",
		"cgo.go":    "//go:build cgo\n\npackage p\n",
		"race.go":   "//go:build race\n\npackage p\n",
		"sim.go":    "//go:build sim && noasm && math_big_pure_go\n\npackage p\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, native := range []bool{false, true} {
		ctx, err := explicitSourceContext("1.27", SourceContext{OS: "linux", Arch: "arm64", Race: true, Native: native})
		if err != nil {
			t.Fatal(err)
		}
		for name, want := range map[string]bool{"future.go": false, "cgo.go": native, "race.go": true, "sim.go": !native} {
			got, err := ctx.MatchFile(dir, name)
			if err != nil || got != want {
				t.Fatal(name, native, got, err)
			}
		}
	}
}

func TestRuntimeStoreSourceInheritance(t *testing.T) {
	dir := "../hooks/rt"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{"1.25", "1.26", "1.27"} {
		ctx, err := explicitSourceContext(family, SourceContext{OS: "linux", Arch: "arm64"})
		if err != nil {
			t.Fatal(err)
		}
		declarations := map[string]string{}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			active, err := selectedSourceFile(ctx, dir, entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			if active {
				if err := readHookDeclarations(filepath.Join(dir, entry.Name()), "rt", declarations); err != nil {
					t.Fatal(err)
				}
			}
		}
		want := "func(*uintptr, uintptr)"
		if family == "1.25" {
			want += " uintptr"
		}
		if got := declarations["rt.Sync_runtime_StoreReluintptr"]; got != want {
			t.Fatal(family, got, want)
		}
	}
}

// This inspects hypothetical future source selection without admitting it for use.
func TestKernelClearenvSourceInheritance(t *testing.T) {
	for _, family := range []string{"1.25", "1.26", "1.27"} {
		ctx, err := explicitSourceContext(family, SourceContext{OS: "linux", Arch: "arm64"})
		if err != nil {
			t.Fatal(err)
		}
		active, err := selectedSourceFile(ctx, "../hooks/kernel", "clearenv_go126.go")
		if err != nil || active != (family != "1.25") {
			t.Fatalf("%s clearenv selected=%v: %v", family, active, err)
		}
	}
}
