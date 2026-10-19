package translate

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

func selectedTestConfig(t *testing.T) gosimtool.BuildConfig {
	t.Helper()
	selection := profileSelection(t, "rt")
	tc := buildspec.Toolchain{GOROOT: "/selected", Executable: "/selected/bin/go", Version: selection.Identity.Toolchain, HostOS: selection.Identity.HostOS, HostArch: selection.Identity.HostArch}
	cfg, err := gosimtool.NewBuildConfig(selection, tc)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestSelectedBuildConfigOwnershipAndValidation(t *testing.T) {
	cfg := selectedTestConfig(t)
	selection := cfg.Selection
	owned, err := gosimtool.NewBuildConfig(selection, cfg.Toolchain)
	if err != nil {
		t.Fatal(err)
	}
	selection.Identity.Features[0] = "changed"
	selection.RuntimePackages[0] = "changed"
	if err := owned.Validate(); err != nil {
		t.Fatal("input mutation changed owned config:", err)
	}
	if owned.AsDirname() != owned.Digest {
		t.Fatal("directory not bound to identity")
	}
	for name, mutate := range map[string]func(*gosimtool.BuildConfig){
		"digest":        func(c *gosimtool.BuildConfig) { c.Digest = strings.Repeat("f", 64) },
		"identity":      func(c *gosimtool.BuildConfig) { c.Identity.Race = !c.Identity.Race },
		"patch":         func(c *gosimtool.BuildConfig) { c.Toolchain.Version = "go1.25.1" },
		"host":          func(c *gosimtool.BuildConfig) { c.Toolchain.HostOS = "wrong" },
		"executable":    func(c *gosimtool.BuildConfig) { c.Toolchain.Executable = "/different/bin/go" },
		"relative-root": func(c *gosimtool.BuildConfig) { c.Toolchain.GOROOT = "relative" },
	} {
		t.Run(name, func(t *testing.T) {
			c := owned
			mutate(&c)
			if c.Validate() == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	env, err := sourceEnvironment(owned)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"GOOS=linux", "GOARCH=" + owned.Identity.SimulationArch, "CGO_ENABLED=0", "GOTOOLCHAIN=local", "GOPACKAGESDRIVER=off"} {
		count := 0
		key, _, _ := strings.Cut(required, "=")
		for _, value := range env {
			if strings.HasPrefix(value, key+"=") {
				count++
				if value != required {
					t.Fatalf("unexpected %s", value)
				}
			}
		}
		if count != 1 {
			t.Fatalf("%s occurs %d times", key, count)
		}
	}
}

func TestSelectedInputPublication(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	data := bytes.Repeat([]byte("module bytes\n"), 4096)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := publishInputFile(path, data, false); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	original, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishInputFile(path, data, false); err != nil {
		t.Fatal(err)
	}
	reused, _ := os.Stat(path)
	if !os.SameFile(original, reused) || !original.ModTime().Equal(reused.ModTime()) {
		t.Fatal("identical publication replaced file")
	}
	if err := publishInputFile(path, []byte("different"), false); err == nil {
		t.Fatal("changed module accepted")
	}
	actual, _ := os.ReadFile(path)
	if !bytes.Equal(data, actual) {
		t.Fatal("failed publication changed content")
	}
	if err := publishInputFile(path, []byte("sum additions"), true); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary files leaked", entries, err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := publishInputFile(link, data, true); err == nil {
		t.Fatal("mutable symlink accepted")
	}
	if err := publishInputFile(dir, data, false); err == nil {
		t.Fatal("directory accepted")
	}
}

func TestSelectedModulePaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/neria-cloud/gosim\ngo 1.25.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"example.com/app", "example.com/app/v2", "gopkg.in/check.v1"} {
		source, err := modfile.Parse("go.mod", []byte("module "+path+"\ngo 1.25.0\nreplace example.com/dependency => ../dependency\n"), nil)
		if err != nil {
			t.Fatal(err)
		}
		before, _ := source.Format()
		files, err := selectedModuleFiles(root, source, root, "translated", "go1.25.14")
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := modfile.Parse("go.mod", files["go.mod"], nil)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Module.Mod.Path != "translated" || parsed.Go.Version != "1.25.14" {
			t.Fatal("wrong generated module")
		}
		after, _ := source.Format()
		if !bytes.Equal(before, after) {
			t.Fatal("input module mutated")
		}
		for _, replacement := range parsed.Replace {
			if replacement.New.Version == "" && !filepath.IsAbs(replacement.New.Path) {
				t.Fatal("relative replacement leaked")
			}
		}
	}
}

func TestSelectedGraphDropsOnlyHookBodies(t *testing.T) {
	p, err := Policies(profileSelection(t, "rt"))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "app.go")
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(source, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pkg := &packages.Package{ID: "example.com/app", PkgPath: "example.com/app", Name: "app", Module: &packages.Module{Path: "example.com/app"}, CompiledGoFiles: []string{source}, Imports: map[string]*packages.Package{
		"net/http": {ID: "net/http", PkgPath: "net/http", Name: "http"},
	}}
	p.hooks[PackageSelector{Pkg: pkg.PkgPath, Selector: "replaced"}] = PackageSelector{Pkg: "owned", Selector: "hook"}
	write("package app; import h \"net/http\";func replaced(){h.Get(\"url\")}")
	graph, err := selectGraph([]*packages.Package{pkg}, p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(graph.order, []string{pkg.ID}) {
		t.Fatal(graph.order)
	}
	for _, text := range []string{
		"package app; import h \"net/http\";func retained(){h.Get(\"url\")}",
		"package app; import h \"net/http\";func replaced() *h.Client{return nil}",
		"package app; import _ \"net/http\";func replaced(){}",
		"package app; import . \"net/http\";func replaced(){Get(\"url\")}",
	} {
		write(text)
		if _, err := selectGraph([]*packages.Package{pkg}, p); err == nil {
			t.Fatalf("surviving rejected dependency accepted: %s", text)
		}
	}
}

func TestSelectedTranslationReturnsInputErrors(t *testing.T) {
	for _, input := range []*TranslateInput{nil, {}, {Packages: []string{"."}}} {
		if _, err := Translate(input); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	cfg := selectedTestConfig(t)
	if _, err := translatePackage(&translatePackageArgs{Selection: cfg.Selection, pkg: &packages.Package{ID: "example.com/missing", PkgPath: "example.com/missing"}}); err == nil || !strings.Contains(err.Error(), "could not load package") {
		t.Fatalf("missing source: %v", err)
	}
}

func TestSelectedPushedDefinitions(t *testing.T) {
	p, err := Policies(profileSelection(t, "rt"))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "push.go")
	text := `package impl
//go:linkname valid net.newUnixFile
func valid(){}
//go:linkname removed net.removed
func removed(){}
//go:linkname native runtime.native
func native(){}
//go:linkname absent net.absent
func absent()
`
	if err := os.WriteFile(source, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := &packages.Package{ID: "example.com/impl", PkgPath: "example.com/impl", CompiledGoFiles: []string{source}}
	graph := &selectedGraph{order: []string{pkg.ID}, nodes: map[string]*packages.Package{pkg.ID: pkg}, native: map[string]bool{}}
	p.hooks[PackageSelector{Pkg: pkg.PkgPath, Selector: "removed"}] = PackageSelector{Pkg: "owned", Selector: "hook"}
	got, err := pushedDefinitions(graph, p)
	if err != nil {
		t.Fatal(err)
	}
	want := map[PackageSelector]bool{{Pkg: "net", Selector: "newUnixFile"}: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("definitions=%v, want %v", got, want)
	}
	graph.native[pkg.ID] = true
	got, err = pushedDefinitions(graph, p)
	if err != nil || len(got) != 0 {
		t.Fatal("native definitions admitted", got, err)
	}
}
