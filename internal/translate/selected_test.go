package translate

import (
	"bytes"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/selected"
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/hostabi"
	"github.com/neria-cloud/gosim/internal/translate/cache"
)

func TestSelectedTranslation(t *testing.T) {
	if os.Getenv("GOSIM_TEST_LARGE") != "1" {
		t.Skip("run with make test-large")
	}

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, patch := range []string{runtime.Version()} {
		t.Run(patch, func(t *testing.T) {
			tc, err := buildcfg.Resolve(patch)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{
				"test",
				"-ldflags=-checklinkname=0",
				"-tags=linkname",
				"./internal/translate",
				"-run=^TestSelectedTranslationWorker$",
				"-count=1",
				"-v",
			}
			if hostabi.RaceEnabled {
				args = append(args, "-race")
			}
			cmd := buildcfg.Command(tc, root, args...)
			cmd.Env = append(cmd.Env, "GOSIM_TRANSLATION_WORKER=1")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("selected translation worker %s: %v\n%s", patch, err, out)
			}
		})
	}
}

func TestSelectedTranslationWorker(t *testing.T) {
	if os.Getenv("GOSIM_TEST_LARGE") != "1" {
		t.Skip("run with make test-large")
	}

	if os.Getenv("GOSIM_TRANSLATION_WORKER") != "1" {
		return
	}
	framework, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	tc, err := buildcfg.Resolve(runtime.Version())
	if err != nil {
		t.Fatal(err)
	}
	t.Run("reject-unbound-loader", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if err := validateLoaderWorker(tc); err == nil {
			t.Fatal("loader accepted an unbound executable")
		}
	})
	project := t.TempDir()
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		t.Fatal(err)
	}
	if err := gosimtool.MakeGoModForTest(framework, project, nil); err != nil {
		t.Fatal(err)
	}
	source := []byte(
		"package fixture\nvar values=map[int]int{1:2}\nfunc Work()int{ch:=make(chan int,1);go func(){ch<-values[1]}();return <-ch}\n",
	)
	app := filepath.Join(project, "app.go")
	if err := os.WriteFile(app, source, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "app_test.go"), []byte(`package fixture
import "testing"
func TestWork(t *testing.T) { if Work() != 2 { t.Fatal("work") } }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	if err := os.WriteFile(filepath.Join(project, "tools.go"), []byte("//go:build never\npackage fixture\nimport _ \"github.com/neria-cloud/gosim\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tidy := buildcfg.Command(tc, project, "mod", "tidy")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("fixture setup: %v\n%s", err, out)
	}
	cfg := liveTranslationConfig(t)
	if err := os.MkdirAll(filepath.Join(project, ".gosim"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := cache.NewDB(filepath.Join(project, ".gosim", "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cached := cache.NewCache(db)
	first, err := translatePackages(cached, []string{"."}, t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := translatePackages(cached, []string{"."}, t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(first.Packages, []string{"translated/test"}) || !slices.Equal(first.Packages, second.Packages) {
		t.Fatal(first.Packages, second.Packages)
	}
	a, b := readTree(t, first.RootOutputDir), readTree(t, second.RootOutputDir)
	if !bytes.Contains(a["test/gosim_meta_test.go"], []byte("TestMain")) ||
		!bytes.Contains(a["test/gosim_meta_test.go"], []byte(".Driver{}")) {
		t.Fatal("test metadata did not select generated Driver")
	}
	if _, ok := a["math/big/arith_decl_pure.go"]; !ok {
		t.Fatal("math/big did not select its Go arithmetic")
	}
	if len(a) != len(b) {
		t.Fatal("cache hit changed output file set")
	}
	for name, data := range a {
		if !bytes.Equal(data, b[name]) {
			t.Fatalf("cache miss/hit differs at %s", name)
		}
	}
	digest := first.Deps["translated/test"][app]
	if digest == "" || digest != second.Deps["translated/test"][app] {
		t.Fatal("missing content-based dependencies")
	}
	t.Run("current-source-discovery", func(t *testing.T) {
		testCurrentSourceDiscovery(t, cfg, cached, project, first.Deps["translated/test"])
	})
	stat, err := os.Stat(app)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app, bytes.Replace(source, []byte("1:2"), []byte("1:3"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(app, stat.ModTime(), stat.ModTime()); err != nil {
		t.Fatal(err)
	}
	cfg = liveTranslationConfig(t)
	third, err := translatePackages(cached, []string{"."}, t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if third.Deps["translated/test"][app] == digest {
		t.Fatal("unchanged mtime hid source change")
	}
	current, err := InspectDependencies(&TranslateInput{Packages: []string{"."}, Cfg: cfg})
	if err != nil || !maps.Equal(current["translated/test"], third.Deps["translated/test"]) {
		t.Fatal("inspection did not reflect source edit with unchanged mtime", err)
	}
	if bytes.Equal(readTree(t, third.RootOutputDir)["test/gosim_globals.go"], a["test/gosim_globals.go"]) {
		t.Fatal("source change reused translated app")
	}
	for name, data := range a {
		if strings.HasSuffix(name, ".go") &&
			(bytes.Contains(data, []byte(gosimtool.Module+"/internal/simulation")) || bytes.Contains(data, []byte(gosimtool.Module+"/internal/hooks/"))) {
			t.Fatalf("legacy path in selected output %s", name)
		}
	}
}

func testCurrentSourceDiscovery(
	t *testing.T,
	cfg gosimtool.BuildConfig,
	cached *cache.Cache,
	project string,
	original map[string]string,
) {
	t.Helper()
	inspect := func(pattern string) map[string]string {
		t.Helper()
		cfg = liveTranslationConfig(t)
		deps, err := InspectDependencies(&TranslateInput{Packages: []string{pattern}, Cfg: cfg})
		if err != nil {
			t.Fatal(err)
		}
		pkg := "translated/test"
		if pattern == "./other" {
			pkg += "/other"
		}
		if len(deps[pkg]) == 0 {
			t.Fatal("missing inspected dependencies", pkg)
		}
		return deps[pkg]
	}
	if !maps.Equal(appSourceInputs(project, inspect(".")), appSourceInputs(project, original)) {
		t.Fatal("inspection disagrees with translated source inputs")
	}
	other := filepath.Join(project, "other")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	embed := filepath.Join(other, "payload.txt")
	write(embed, "before")
	write(
		filepath.Join(other, "other.go"),
		"package other\nimport _ \"embed\"\n//go:embed payload.txt\nvar Payload string\n",
	)
	write(
		filepath.Join(other, "other_test.go"),
		"package other_test\nimport (\"testing\"; \"test/other\")\nfunc TestOther(t *testing.T) { if other.Payload != \"before\" { t.Fatal(other.Payload) } }\n",
	)
	cfg = liveTranslationConfig(t)
	batch, err := translatePackages(cached, []string{".", "./other"}, t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(appSourceInputs(project, batch.Deps["translated/test"]), appSourceInputs(project, original)) {
		t.Fatal("batch membership changed root package inputs")
	}
	otherInputs := inspect("./other")
	if !maps.Equal(otherInputs, batch.Deps["translated/test/other"]) {
		t.Fatal("batch build differs from individual package inspection")
	}
	if otherInputs[embed] == "" || original[embed] != "" || otherInputs[filepath.Join(project, "app.go")] != "" {
		t.Fatal("package closures include unrelated sources or omit embedded content")
	}
	before, err := os.Stat(embed)
	if err != nil {
		t.Fatal(err)
	}
	write(embed, "after!")
	if err := os.Chtimes(embed, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if maps.Equal(inspect("./other"), otherInputs) {
		t.Fatal("embedded content change with unchanged mtime was missed")
	}
	if !maps.Equal(appSourceInputs(project, inspect(".")), appSourceInputs(project, original)) {
		t.Fatal("unrelated package mutation invalidated root")
	}
	extra := filepath.Join(project, "extra.go")
	write(extra, "package fixture\nvar Extra = 1\n")
	added := inspect(".")
	if added[extra] == "" || maps.Equal(appSourceInputs(project, added), appSourceInputs(project, original)) {
		t.Fatal("new source was missed")
	}
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(appSourceInputs(project, inspect(".")), appSourceInputs(project, original)) {
		t.Fatal("removing added source did not restore original inputs")
	}
}

func liveTranslationConfig(t *testing.T) gosimtool.BuildConfig {
	t.Helper()
	r, err := buildcfg.ResolveRunningProcess(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	p := selected.Profile()
	inputs, err := buildcfg.Fingerprint(r.Toolchain)
	if err != nil {
		t.Fatal(err)
	}
	opts := buildspec.BuildOptions{Race: hostabi.RaceEnabled}
	contract, err := spec.CheckContracts(t.Context(), r, p, opts, inputs, "")
	if err != nil {
		t.Fatal(err)
	}
	id, err := buildcfg.Identity(
		t.Context(),
		buildspec.Config{Schema: 2, Features: []string{"rt"}, SimulationOS: "linux"},
		r,
		p,
		opts,
		inputs,
		contract,
	)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := p.Select(id)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := gosimtool.NewBuildConfig(t.Context(), selection, r)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func appSourceInputs(project string, inputs map[string]string) map[string]string {
	result := make(map[string]string)
	for name, digest := range inputs {
		rel, err := filepath.Rel(project, name)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) &&
			!strings.HasPrefix(rel, ".gosim"+string(filepath.Separator)) {
			result[name] = digest
		}
	}
	return result
}

func readTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		result[filepath.ToSlash(relative)] = data
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return result
}
