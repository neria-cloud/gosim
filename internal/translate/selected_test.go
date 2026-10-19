package translate

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/race"
	"github.com/neria-cloud/gosim/internal/translate/cache"
)

func TestSelectedTranslation(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, patch := range []string{"go1.25.14", "go1.26.9"} {
		t.Run(patch, func(t *testing.T) {
			tc, err := buildcfg.Resolve(patch)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"test", "-ldflags=-checklinkname=0", "-tags=linkname", "./internal/translate", "-run=^TestSelectedTranslationWorker$", "-count=1", "-v"}
			if race.Enabled {
				args = append(args, "-race")
			}
			cmd := buildcfg.Command(tc, root, args...)
			cmd.Env = append(cmd.Env, "GOSIM_TRANSLATION_WORKER=1")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("pinned translation worker %s: %v\n%s", patch, err, out)
			}
		})
	}
}

func TestSelectedTranslationWorker(t *testing.T) {
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
	selection := profileSelection(t, "rt")
	selection.Identity.Race = race.Enabled
	cfg, err := gosimtool.NewBuildConfig(selection, tc)
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
	if err := gosimtool.MakeGoModForTest(framework, project, nil); err != nil {
		t.Fatal(err)
	}
	source := []byte("package fixture\nvar values=map[int]int{1:2}\nfunc Work()int{ch:=make(chan int,1);go func(){ch<-values[1]}();return <-ch}\n")
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
	db, err := cache.NewDB(filepath.Join(project, "cache.db"))
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
	if !bytes.Contains(a["test/gosim_meta_test.go"], []byte("TestMainDriver")) || !bytes.Contains(a["test/gosim_meta_test.go"], []byte(".Driver{}")) {
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
	third, err := translatePackages(cached, []string{"."}, t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if third.Deps["translated/test"][app] == digest {
		t.Fatal("unchanged mtime hid source change")
	}
	if bytes.Equal(readTree(t, third.RootOutputDir)["test/gosim_globals.go"], a["test/gosim_globals.go"]) {
		t.Fatal("source change reused translated app")
	}
	for name, data := range a {
		if strings.HasSuffix(name, ".go") && (bytes.Contains(data, []byte(gosimtool.Module+"/internal/simulation")) || bytes.Contains(data, []byte(gosimtool.Module+"/internal/hooks/"))) {
			t.Fatalf("legacy path in selected output %s", name)
		}
	}
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
